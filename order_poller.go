package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"go.uber.org/zap"
)

type orderPoller struct {
	client        cosmosclient.Client
	indexerURL    string
	interval      time.Duration
	indexerClient *http.Client
	logger        *zap.Logger

	batchSize       int
	newOrders       chan []*demandOrder
	canFulfillOrder func(*demandOrder) bool
	sync.Mutex
	pathMap map[string]string
}

func newOrderPoller(
	client cosmosclient.Client,
	canFulfillOrder func(*demandOrder) bool,
	pollingCfg OrderPollingConfig,
	batchSize int,
	newOrders chan []*demandOrder,
	logger *zap.Logger,
) *orderPoller {
	return &orderPoller{
		client:          client,
		indexerURL:      pollingCfg.IndexerURL,
		interval:        pollingCfg.Interval,
		batchSize:       batchSize,
		logger:          logger.With(zap.String("module", "order-poller")),
		newOrders:       newOrders,
		canFulfillOrder: canFulfillOrder,
		pathMap:         make(map[string]string),
		indexerClient:   &http.Client{Timeout: 25 * time.Second},
	}
}

const (
	ordersQuery = `{"query": "{ibcTransferDetails(filter: {network: {equalTo: \"%s\"} status: {equalTo: EibcPending}}) {nodes { eibcOrderId amount destinationChannel blockHeight rollappId eibcFee }}}"}`
)

type Order struct {
	EibcOrderId string `json:"eibcOrderId"`
	Amount      string `json:"amount"`
	Fee         string `json:"eibcFee"`
	RollappId   string `json:"rollappId"`
	BlockHeight string `json:"blockHeight"`
}

type ordersResponse struct {
	Data struct {
		IbcTransferDetails struct {
			Nodes []Order `json:"nodes"`
		} `json:"ibcTransferDetails"`
	} `json:"data"`
}

func (p *orderPoller) start(ctx context.Context) {
	go func() {
		for c := time.Tick(p.interval); ; <-c {
			select {
			case <-ctx.Done():
				return
			default:
				if err := p.pollPendingDemandOrders(); err != nil {
					p.logger.Error("failed to refresh demand orders", zap.Error(err))
				}
			}
		}
	}()
}

func (p *orderPoller) pollPendingDemandOrders() error {
	demandOrders, err := p.getDemandOrdersFromIndexer()
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	orders := p.convertOrders(demandOrders)
	batch := make([]*demandOrder, 0, p.batchSize)
	ids := make([]string, 0, len(orders))

	for _, order := range orders {
		batch = append(batch, order)
		ids = append(ids, order.id)

		if len(batch) >= p.batchSize || len(batch) == len(orders) {
			p.newOrders <- batch
			batch = make([]*demandOrder, 0, p.batchSize)
			ids = make([]string, 0, len(orders))

			if p.logger.Level() <= zap.DebugLevel {
				p.logger.Debug("new orders batch", zap.Strings("count", ids))
			} else {
				p.logger.Info("new orders batch", zap.Int("count", len(ids)))
			}
		}
	}

	return nil
}

func (p *orderPoller) convertOrders(demandOrders []Order) (orders []*demandOrder) {
	for _, order := range demandOrders {
		if order.Fee == "" {
			p.logger.Warn("order fee is empty", zap.String("order", order.EibcOrderId))
			continue
		}

		fee, err := sdk.ParseCoinsNormalized(order.Fee)
		if err != nil {
			p.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}

		denom := fee.GetDenomByIndex(0)

		amountStr := fmt.Sprintf("%s%s", order.Amount, denom)
		amount, err := sdk.ParseCoinsNormalized(amountStr)
		if err != nil {
			p.logger.Error("failed to parse amount", zap.Error(err))
			continue
		}

		var blockHeight uint64
		if order.BlockHeight != "" {
			blockHeight, err = strconv.ParseUint(order.BlockHeight, 10, 64)
			if err != nil {
				p.logger.Error("failed to parse block height", zap.Error(err))
				continue
			}
		}

		newOrder := &demandOrder{
			id:          order.EibcOrderId,
			amount:      amount,
			fee:         fee,
			denom:       denom,
			rollappId:   order.RollappId,
			blockHeight: blockHeight,
		}

		if !p.canFulfillOrder(newOrder) {
			continue
		}

		orders = append(orders, newOrder)
	}

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].blockHeight < orders[j].blockHeight
	})
	return orders
}

func (p *orderPoller) getDemandOrdersFromIndexer() ([]Order, error) {
	p.logger.Debug("getting demand orders from indexer")

	queryStr := fmt.Sprintf(ordersQuery, p.client.Context().ChainID)
	body := strings.NewReader(queryStr)

	resp, err := p.indexerClient.Post(p.indexerURL, "application/json", body)
	if err != nil {
		return nil, fmt.Errorf("failed to get demand orders: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get demand orders: %s", resp.Status)
	}

	var res ordersResponse
	if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return res.Data.IbcTransferDetails.Nodes, nil
}

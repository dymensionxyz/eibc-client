package eibc

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
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
)

type orderPoller struct {
	chainID       string
	indexerURL    string
	interval      time.Duration
	indexerClient *http.Client
	logger        *zap.Logger

	getOrders    func() ([]Order, error)
	orderTracker *orderTracker
	sync.Mutex
}

func newOrderPoller(
	chainID string,
	orderTracker *orderTracker,
	pollingCfg config.OrderPollingConfig,
	logger *zap.Logger,
) *orderPoller {
	o := &orderPoller{
		chainID:       chainID,
		indexerURL:    pollingCfg.IndexerURL,
		interval:      pollingCfg.Interval,
		logger:        logger.With(zap.String("module", "order-poller")),
		orderTracker:  orderTracker,
		indexerClient: &http.Client{Timeout: 25 * time.Second},
	}
	o.getOrders = o.getDemandOrdersFromIndexer
	return o
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

func (p *orderPoller) start(ctx context.Context) error {
	if err := p.pollPendingDemandOrders(); err != nil {
		return fmt.Errorf("failed to refresh demand orders: %w", err)
	}

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
	return nil
}

func (p *orderPoller) pollPendingDemandOrders() error {
	demandOrders, err := p.getOrders()
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	newOrders := p.convertOrders(demandOrders)

	if len(newOrders) == 0 {
		p.logger.Debug("no new orders")
		return nil
	}

	if p.logger.Level() <= zap.DebugLevel {
		ids := make([]string, 0, len(newOrders))
		for _, order := range newOrders {
			ids = append(ids, order.id)
		}
		p.logger.Debug("new demand orders", zap.Strings("ids", ids))
	} else {
		p.logger.Info("new demand orders", zap.Int("count", len(newOrders)))
	}

	p.orderTracker.addOrder(newOrders...)

	return nil
}

func (p *orderPoller) convertOrders(demandOrders []Order) (orders []*demandOrder) {
	for _, order := range demandOrders {
		if order.Fee == "" {
			p.logger.Warn("order fee is empty", zap.String("order", order.EibcOrderId))
			continue
		}

		fee, err := sdk.ParseCoinNormalized(order.Fee)
		if err != nil {
			p.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}

		amountStr := fmt.Sprintf("%s%s", order.Amount, fee.Denom)
		amount, err := sdk.ParseCoinsNormalized(amountStr)
		if err != nil {
			p.logger.Error("failed to parse amount", zap.Error(err))
			continue
		}

		var blockHeight int64
		if order.BlockHeight != "" {
			blockHeight, err = strconv.ParseInt(order.BlockHeight, 10, 64)
			if err != nil {
				p.logger.Error("failed to parse block height", zap.Error(err))
				continue
			}
		}

		validationWaitTime := p.orderTracker.validation.ValidationWaitTime
		validDeadline := time.Now().Add(validationWaitTime)

		newOrder := &demandOrder{
			id:            order.EibcOrderId,
			amount:        amount,
			fee:           fee,
			denom:         fee.Denom,
			rollappId:     order.RollappId,
			proofHeight:   blockHeight,
			validDeadline: validDeadline,
		}

		if !p.orderTracker.canFulfillOrder(newOrder) {
			continue
		}

		if err := p.orderTracker.findLPForOrder(newOrder); err != nil {
			p.logger.Warn("failed to find LP for order", zap.Error(err), zap.String("order_id", newOrder.id))
			continue
		}

		orders = append(orders, newOrder)
	}

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].proofHeight < orders[j].proofHeight
	})
	return orders
}

func (p *orderPoller) getDemandOrdersFromIndexer() ([]Order, error) {
	p.logger.Debug("getting demand orders from indexer")

	queryStr := fmt.Sprintf(ordersQuery, p.chainID)
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

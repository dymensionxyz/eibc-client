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
	"github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"go.uber.org/zap"
)

type orderPoller struct {
	client        cosmosclient.Client
	indexerURL    string
	interval      time.Duration
	indexerClient *http.Client
	logger        *zap.Logger

	batchSize int
	newOrders chan []*demandOrder
	tracker   *orderTracker
	sync.Mutex
	pathMap map[string]string
}

func newOrderPoller(
	client cosmosclient.Client,
	tracker *orderTracker,
	pollingCfg OrderPollingConfig,
	batchSize int,
	newOrders chan []*demandOrder,
	logger *zap.Logger,
) *orderPoller {
	return &orderPoller{
		client:        client,
		indexerURL:    pollingCfg.IndexerURL,
		interval:      pollingCfg.Interval,
		batchSize:     batchSize,
		logger:        logger.With(zap.String("module", "order-poller")),
		newOrders:     newOrders,
		tracker:       tracker,
		pathMap:       make(map[string]string),
		indexerClient: &http.Client{Timeout: 25 * time.Second},
	}
}

const (
	ordersQuery = `{"query": "{ibcTransferDetails(filter: {network: {equalTo: \"%s\"} status: {equalTo: EibcPending}}) {nodes { eibcOrderId denom amount destinationChannel time }}}"}`
)

type Order struct {
	EibcOrderId        string `json:"eibcOrderId"`
	Denom              string `json:"denom"`
	Amount             string `json:"amount"`
	Fee                string `json:"fee"`
	DestinationChannel string `json:"destinationChannel"`
	Time               string `json:"time"`
	time               time.Time
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
				if err := p.pollPendingDemandOrders(ctx); err != nil {
					p.logger.Error("failed to refresh demand orders", zap.Error(err))
				}
			}
		}
	}()
}

func (p *orderPoller) pollPendingDemandOrders(ctx context.Context) error {
	p.logger.Debug("refreshing demand orders")

	orders, err := p.getDemandOrdersFromIndexer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	unfulfilledOrders := make([]*demandOrder, 0, len(orders))

	for _, d := range orders {
		if !p.tracker.canFulfillOrder(d.EibcOrderId, d.Denom) {
			continue
		}

		amountStr := fmt.Sprintf("%s%s", d.Amount, d.Denom)

		totalAmount, err := sdk.ParseCoinNormalized(amountStr)
		if err != nil {
			p.logger.Error("failed to parse amount", zap.Error(err))
			continue
		}
		order := &demandOrder{
			id:     d.EibcOrderId,
			amount: sdk.NewCoins(totalAmount),
			fee:    d.Fee,
		}
		unfulfilledOrders = append(unfulfilledOrders, order)
	}

	if len(unfulfilledOrders) == 0 {
		return nil
	}

	ids := make([]string, 0, len(unfulfilledOrders))
	for _, order := range unfulfilledOrders {
		ids = append(ids, order.id)
	}

	if p.logger.Level() <= zap.DebugLevel {
		p.logger.Debug("new demand orders", zap.Strings("count", ids))
	} else {
		p.logger.Info("new demand orders", zap.Int("count", len(ids)))
	}

	batch := make([]*demandOrder, 0, p.batchSize)

	for _, order := range unfulfilledOrders {
		batch = append(batch, order)

		if len(batch) >= p.batchSize || len(batch) == len(unfulfilledOrders) {
			p.newOrders <- batch
			batch = make([]*demandOrder, 0, p.batchSize)
		}
	}

	return nil
}

func (p *orderPoller) getDemandOrdersFromIndexer(ctx context.Context) ([]Order, error) {
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

	var orders []Order

	// parse the time format of "1714742916108" into time.Time and sort by time
	for _, order := range res.Data.IbcTransferDetails.Nodes {
		if order.Time == "" {
			continue
		}

		timeUnix, err := strconv.ParseInt(order.Time, 10, 64)
		if err != nil {
			p.logger.Error("failed to parse time", zap.Error(err))
			continue
		}

		denom, err := p.getDenomFromPath(ctx, order.Denom, order.DestinationChannel)
		if err != nil {
			p.logger.Error("failed to get denom hash", zap.String("d.Denom", order.Denom), zap.Error(err))
			continue
		}

		if order.Fee == "" {
			p.logger.Warn("order fee is empty", zap.String("order", order.EibcOrderId))
			continue
		}

		newOrder := Order{
			EibcOrderId:        order.EibcOrderId,
			Denom:              denom,
			Amount:             order.Amount,
			Fee:                order.Fee,
			DestinationChannel: order.DestinationChannel,
			Time:               order.Time,
			time:               time.Unix(timeUnix/1000, (timeUnix%1000)*int64(time.Millisecond)),
		}

		orders = append(orders, newOrder)
	}

	// sort by time
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].time.Before(orders[j].time)
	})

	return orders, nil
}

func (p *orderPoller) getDenomFromPath(ctx context.Context, path, destChannel string) (string, error) {
	zeroChannelPrefix := "transfer/channel-0/"

	// Remove "transfer/channel-0/" prefix if it exists
	path = strings.TrimPrefix(path, zeroChannelPrefix)

	if path == defaultHubDenom {
		return defaultHubDenom, nil
	}

	// for denoms other than adym we should have a full path
	// in order to be albe to derive the ibc denom hash
	if !strings.Contains(path, "channel-") {
		path = fmt.Sprintf("transfer/%s/%s", destChannel, path)
	}

	p.Lock()
	defer p.Unlock()

	denom, ok := p.pathMap[path]
	if ok {
		return denom, nil
	}

	queryClient := types.NewQueryClient(p.client.Context())

	req := &types.QueryDenomHashRequest{
		Trace: path,
	}

	res, err := queryClient.DenomHash(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to query denom hash: %w", err)
	}

	ibcDenom := fmt.Sprintf("ibc/%s", res.Hash)

	p.pathMap[path] = ibcDenom

	return ibcDenom, nil
}

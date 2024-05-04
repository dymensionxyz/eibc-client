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
	"github.com/dymensionxyz/dymension/v3/x/common/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderFetcher struct {
	client        cosmosclient.Client
	denomFetch    *denomFetcher
	indexerURL    string
	indexerClient *http.Client
	logger        *zap.Logger

	batchSize     int
	domu          sync.Mutex
	newOrders     chan []*demandOrder
	unfulfilledCh chan []string
	demandOrders  map[string]struct{}
}

func newOrderFetcher(
	client cosmosclient.Client,
	denomFetch *denomFetcher,
	indexerURL string,
	batchSize int,
	newOrders chan []*demandOrder,
	unfulfilledCh chan []string,
	logger *zap.Logger,
) *orderFetcher {
	return &orderFetcher{
		client:        client,
		denomFetch:    denomFetch,
		indexerURL:    indexerURL,
		indexerClient: &http.Client{Timeout: 30 * time.Second},
		batchSize:     batchSize,
		logger:        logger.With(zap.String("module", "order-fetcher")),
		newOrders:     newOrders,
		unfulfilledCh: unfulfilledCh,
		demandOrders:  make(map[string]struct{}),
	}
}

func (of *orderFetcher) start(ctx context.Context, refreshInterval, cleanupInterval time.Duration) error {
	if err := of.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	of.orderRefresher(ctx, refreshInterval)
	of.orderCleaner(ctx, cleanupInterval) // TODO: check how many blocks the order is old
	of.doneOrders(ctx)

	return nil
}

func (of *orderFetcher) doneOrders(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ids := <-of.unfulfilledCh:
				of.deleteOrders(ids)
			}
		}
	}()
}

func (of *orderFetcher) deleteOrders(ids []string) {
	of.domu.Lock()
	defer of.domu.Unlock()

	for _, id := range ids {
		delete(of.demandOrders, id)
	}
}

func (of *orderFetcher) refreshPendingDemandOrders(ctx context.Context) error {
	of.logger.Debug("refreshing demand orders")

	orders, err := of.getDemandOrdersFromIndexer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	unfulfilledOrders := make([]*demandOrder, 0, len(orders))

	// TODO: maybe check here which denoms the whale can provide funds for

	of.domu.Lock()
	for _, d := range orders {
		// if already in the map, means fulfilled or fulfilling
		if _, found := of.demandOrders[d.EibcOrderId]; found {
			continue
		}

		amountStr := fmt.Sprintf("%s%s", d.Amount, d.Denom)

		amount, err := sdk.ParseCoinNormalized(amountStr)
		if err != nil {
			of.logger.Error("failed to parse amount", zap.Error(err))
			continue
		}
		// otherwise, save to prevent duplicates
		of.demandOrders[d.EibcOrderId] = struct{}{}
		order := &demandOrder{
			id:    d.EibcOrderId,
			price: sdk.NewCoins(amount),
			// fee:   d.Fee,
		}
		unfulfilledOrders = append(unfulfilledOrders, order)
	}
	of.domu.Unlock()

	if len(unfulfilledOrders) == 0 {
		return nil
	}

	of.logger.Info("new demand orders", zap.Int("count", len(unfulfilledOrders)))

	batch := make([]*demandOrder, 0, of.batchSize)

	for _, order := range unfulfilledOrders {
		batch = append(batch, order)

		if len(batch) >= of.batchSize || len(batch) == len(unfulfilledOrders) {
			of.newOrders <- batch
			batch = make([]*demandOrder, 0, of.batchSize)
		}
	}

	return nil
}

func (of *orderFetcher) enqueueEventOrders(res tmtypes.ResultEvent) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return
	}

	// packetKeys := res.Events["eibc.packet_key"]
	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	// recipients := res.Events["transfer.recipient"]
	unfulfilledOrders := make([]*demandOrder, 0, len(ids))

	of.domu.Lock()
	for i, id := range ids {
		if statuses[i] != types.Status_PENDING.String() {
			continue
		}

		// exclude ones that are already fulfilled or fulfilling
		if _, found := of.demandOrders[id]; found {
			continue
		}
		// otherwise, save to prevent duplicates
		of.demandOrders[id] = struct{}{}

		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			of.logger.Error("failed to parse price", zap.Error(err))
			continue
		}
		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			of.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}
		order := &demandOrder{
			id:    id,
			price: sdk.NewCoins(price),
			fee:   sdk.NewCoins(fee),
		}
		unfulfilledOrders = append(unfulfilledOrders, order)
	}
	of.domu.Unlock()

	if len(unfulfilledOrders) == 0 {
		return
	}

	of.logger.Info("new demand orders from event", zap.Int("count", len(unfulfilledOrders)))

	batch := make([]*demandOrder, 0, of.batchSize)

	for _, order := range unfulfilledOrders {
		batch = append(batch, order)

		if len(batch) >= of.batchSize || len(batch) == len(unfulfilledOrders) {
			of.newOrders <- batch
			batch = make([]*demandOrder, 0, of.batchSize)
		}
	}
}

const (
	ordersQuery = `{"query": "{ibcTransferDetails(filter: {network: {equalTo: \"%s\"} status: {equalTo: EibcPending}}) {nodes { eibcOrderId denom amount destinationChannel time }}}"}`
)

type Order struct {
	EibcOrderId        string `json:"eibcOrderId"`
	Denom              string `json:"denom"`
	Amount             string `json:"amount"`
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

func (of *orderFetcher) getDemandOrdersFromIndexer(ctx context.Context) ([]Order, error) {
	queryStr := fmt.Sprintf(ordersQuery, of.client.Context().ChainID)
	body := strings.NewReader(queryStr)

	resp, err := of.indexerClient.Post(of.indexerURL, "application/json", body)
	if err != nil {
		return nil, fmt.Errorf("failed to get demand orders: %w", err)
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
			of.logger.Error("failed to parse time", zap.Error(err))
			continue
		}

		denom, err := of.denomFetch.getDenomFromPath(ctx, order.Denom, order.DestinationChannel)
		if err != nil {
			of.logger.Error("failed to get denom hash", zap.String("d.Denom", order.Denom), zap.Error(err))
			continue
		}

		newOrder := Order{
			EibcOrderId:        order.EibcOrderId,
			Denom:              denom,
			Amount:             order.Amount,
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

func (of *orderFetcher) subscribeToPendingDemandOrders(ctx context.Context) error {
	if err := of.client.RPC.Start(); err != nil {
		return fmt.Errorf("failed to start rpc: %w", err)
	}

	const query = "eibc.is_fulfilled='false'"

	resCh, err := of.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				of.enqueueEventOrders(res)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (of *orderFetcher) orderRefresher(ctx context.Context, refreshInterval time.Duration) {
	go func() {
		for c := time.Tick(refreshInterval); ; <-c {
			select {
			case <-ctx.Done():
				return
			default:
				if err := of.refreshPendingDemandOrders(ctx); err != nil {
					of.logger.Error("failed to refresh demand orders", zap.Error(err))
				}
			}
		}
	}()
}

func (of *orderFetcher) orderCleaner(ctx context.Context, orderCleanupInterval time.Duration) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(orderCleanupInterval):
				if err := of.cleanup(); err != nil {
					of.logger.Error("failed to cleanup", zap.Error(err))
				}
			}
		}
	}()
}

func (of *orderFetcher) cleanup() error {
	of.domu.Lock()
	defer of.domu.Unlock()

	cleanupCount := 0

	for id, _ := range of.demandOrders {
		// TODO: check if credited
		// cleanup fulfilled and credited demand orders
		delete(of.demandOrders, id)
		cleanupCount++
	}

	if cleanupCount > 0 {
		of.logger.Info("cleaned up fulfilled orders", zap.Int("count", cleanupCount))
	}
	return nil
}
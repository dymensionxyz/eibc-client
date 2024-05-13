package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	newOrders     chan []*demandOrder
	unfulfilledCh chan []string
	tracker       *orderTracker
}

func newOrderFetcher(
	client cosmosclient.Client,
	denomFetch *denomFetcher,
	tracker *orderTracker,
	indexerURL string,
	batchSize int,
	newOrders chan []*demandOrder,
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
		tracker:       tracker,
	}
}

func (of *orderFetcher) start(ctx context.Context, refreshInterval, cleanupInterval time.Duration) error {
	if err := of.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	of.orderRefresher(ctx, refreshInterval)

	return nil
}

func (of *orderFetcher) refreshPendingDemandOrders(ctx context.Context) error {
	of.logger.Debug("refreshing demand orders")

	orders, err := of.getDemandOrdersFromIndexer(ctx)
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	unfulfilledOrders := make([]*demandOrder, 0, len(orders))

	// TODO: maybe check here which denoms the whale can provide funds for

	for _, d := range orders {
		// if already in the map, means fulfilled or fulfilling
		if of.tracker.isOrderFulfilled(d.EibcOrderId) {
			continue
		}

		if of.tracker.isOrderCurrent(d.EibcOrderId) {
			continue
		}

		amountStr := fmt.Sprintf("%s%s", d.Amount, d.Denom)

		totalAmount, err := sdk.ParseCoinNormalized(amountStr)
		if err != nil {
			of.logger.Error("failed to parse amount", zap.Error(err))
			continue
		}
		order := &demandOrder{
			id:     d.EibcOrderId,
			amount: sdk.NewCoins(totalAmount),
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

	if of.logger.Level() <= zap.DebugLevel {
		of.logger.Debug("new demand orders", zap.Strings("count", ids))
	} else {
		of.logger.Info("new demand orders", zap.Int("count", len(ids)))
	}

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

func (of *orderFetcher) enqueueEventOrders(res tmtypes.ResultEvent) error {
	newOrders, err := parseOrdersFromEvents(res)
	if err != nil {
		return fmt.Errorf("failed to parse orders from events: %w", err)
	}

	batch := make([]*demandOrder, 0, of.batchSize)

	for _, order := range newOrders {
		// exclude ones that are already fulfilled or fulfilling
		if of.tracker.isOrderFulfilled(order.id) {
			continue
		}

		batch = append(batch, order)

		if len(batch) >= of.batchSize || len(batch) == len(newOrders) {
			of.newOrders <- batch
			batch = make([]*demandOrder, 0, of.batchSize)
		}
	}

	if len(batch) == 0 {
		return nil
	}

	of.logger.Info("new demand orders from event", zap.Int("count", len(batch)))

	return nil
}

func parseOrdersFromEvents(res tmtypes.ResultEvent) ([]*demandOrder, error) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return nil, nil
	}

	// packetKeys := res.Events["eibc.packet_key"]
	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	// recipients := res.Events["transfer.recipient"]
	newOrders := make([]*demandOrder, 0, len(ids))

	for i, id := range ids {
		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse price: %w", err)
		}

		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse fee: %w", err)
		}

		order := &demandOrder{
			id:     id,
			amount: sdk.NewCoins(price.Add(fee)),
			status: statuses[i],
		}
		newOrders = append(newOrders, order)
	}
	return newOrders, nil
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
	const query = "eibc.is_fulfilled='false'"

	resCh, err := of.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := of.enqueueEventOrders(res); err != nil {
					of.logger.Error("failed to enqueue event orders", zap.Error(err))
				}
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

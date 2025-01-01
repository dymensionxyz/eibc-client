package eibc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderPoller struct {
	chainID       string
	indexerURL    string
	rollapps      []string
	interval      time.Duration
	indexerClient *http.Client
	rollappClient types.QueryClient
	logger        *zap.Logger

	getOrders       func(ctx context.Context) ([]Order, error)
	orderTracker    *orderTracker
	lastBlockHeight atomic.Uint64
}

func newOrderPoller(
	clientCtx client.Context,
	orderTracker *orderTracker,
	pollingCfg config.OrderPollingConfig,
	rollapps []string,
	logger *zap.Logger,
) *orderPoller {
	o := &orderPoller{
		chainID:       clientCtx.ChainID,
		rollapps:      rollapps,
		indexerURL:    pollingCfg.IndexerURL,
		interval:      pollingCfg.Interval,
		logger:        logger.With(zap.String("module", "order-poller")),
		orderTracker:  orderTracker,
		rollappClient: types.NewQueryClient(clientCtx),
		indexerClient: &http.Client{Timeout: 25 * time.Second},
	}
	o.getOrders = o.getDemandOrdersFromIndexer
	return o
}

const (
	rollappOrdersQuery = `{"query": "{ibcTransferDetails(filter: {network: {equalTo: \"%s\"} status: { in: [EibcPending, Refunding] }, blockHeight: { greaterThan: \"%s\" }, rollappId: { equalTo: \"%s\"}, proofHeight: {greaterThan: \"%s\"}}) {nodes { eibcOrderId amount proofHeight blockHeight price rollappId eibcFee }}}"}`
)

type Order struct {
	EibcOrderId string `json:"eibcOrderId"`
	Amount      string `json:"amount"`
	Price       string `json:"price"`
	Fee         string `json:"eibcFee"`
	RollappId   string `json:"rollappId"`
	ProofHeight string `json:"proofHeight"`
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
	if err := p.pollPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to refresh demand orders: %w", err)
	}

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
	return nil
}

func (p *orderPoller) pollPendingDemandOrders(ctx context.Context) error {
	newDemandOrders, err := p.getOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	demandOrders := make([]Order, 0, len(newDemandOrders))
	for _, order := range newDemandOrders {
		blockHeight, err := strconv.ParseUint(order.BlockHeight, 10, 64)
		if err != nil {
			p.logger.Error("failed to parse block height", zap.Error(err))
			continue
		}
		if blockHeight > p.lastBlockHeight.Load() {
			p.lastBlockHeight.Store(blockHeight)
		}
		demandOrders = append(demandOrders, order)
	}

	newOrders := p.convertOrders(demandOrders)

	if len(newOrders) == 0 {
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

	p.orderTracker.trackOrders(newOrders...)

	return nil
}

func (p *orderPoller) convertOrders(demandOrders []Order) (orders []*demandOrder) {
	for _, order := range demandOrders {
		if order.Fee == "" {
			continue
		}

		if order.Price == "" {
			continue
		}

		if order.ProofHeight == "" {
			continue
		}

		fee, err := sdk.ParseCoinNormalized(order.Fee)
		if err != nil {
			p.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}

		amount, ok := sdk.NewIntFromString(order.Amount)
		if !ok {
			p.logger.Error("failed to parse amount", zap.String("amount", order.Amount))
			continue
		}

		priceInt, ok := sdk.NewIntFromString(order.Price)
		if !ok {
			p.logger.Error("failed to parse price", zap.String("price", order.Price))
			continue
		}

		price := sdk.NewCoins(sdk.NewCoin(fee.Denom, priceInt))

		proofHeight, err := strconv.ParseInt(order.ProofHeight, 10, 64)
		if err != nil {
			p.logger.Error("failed to parse proof height", zap.Error(err))
			continue
		}

		// in case tracked order got updated
		existOrder, ok := p.orderTracker.pool.getOrder(order.EibcOrderId)
		if ok {
			// update the fee and price of the order
			existOrder.fee = fee
			existOrder.price = price
			p.orderTracker.pool.upsertOrder(existOrder)
			continue
		}

		newOrder := &demandOrder{
			id:            order.EibcOrderId,
			price:         price,
			amount:        amount,
			fee:           fee,
			denom:         fee.Denom,
			rollappId:     order.RollappId,
			proofHeight:   proofHeight,
			validDeadline: time.Now().Add(p.orderTracker.validation.WaitTime),
			from:          "indexer",
		}

		if !p.orderTracker.canFulfillOrder(newOrder) {
			continue
		}

		if err := p.orderTracker.findLPForOrder(newOrder); err != nil {
			p.logger.Debug("failed to find LP for order", zap.Error(err), zap.String("order_id", newOrder.id))
			continue
		}

		orders = append(orders, newOrder)
	}

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].proofHeight < orders[j].proofHeight
	})
	return orders
}

func (p *orderPoller) getDemandOrdersFromIndexer(ctx context.Context) ([]Order, error) {
	var demandOrders []Order
	for _, rollapp := range p.rollapps {
		orders, err := p.getRollappDemandOrdersFromIndexer(ctx, rollapp)
		if err != nil {
			return nil, fmt.Errorf("failed to get demand orders: %w", err)
		}
		demandOrders = append(demandOrders, orders...)
	}
	return demandOrders, nil
}

func (p *orderPoller) getRollappDemandOrdersFromIndexer(ctx context.Context, rollappId string) ([]Order, error) {
	var lastFinalizedHeight string
	lastHeightResp, err := p.rollappClient.LatestHeight(ctx, &types.QueryGetLatestHeightRequest{
		RollappId: rollappId,
		Finalized: true,
	})
	if err != nil {
		p.logger.Warn("failed to get latest height, using 0", zap.Error(err))
	} else {
		lastFinalizedHeight = fmt.Sprint(lastHeightResp.Height)
	}

	queryStr := fmt.Sprintf(rollappOrdersQuery, p.chainID, fmt.Sprint(p.lastBlockHeight.Load()), rollappId, lastFinalizedHeight)
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

func (p *orderPoller) resetOrderPolling() {
	p.lastBlockHeight.Store(0)
}

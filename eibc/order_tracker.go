package eibc

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/cosmos/gogoproto/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderTracker struct {
	broadcastTx    broadcastTxFn
	getLPGrants    getLPGrantsFn
	fullNodeClient *nodeClient
	logger         *zap.Logger
	bots           map[string]*orderFulfiller
	policyAddress  string

	fomu                sync.Mutex
	lpmu                sync.Mutex
	fulfilledOrders     map[string]*demandOrder
	validOrdersCh       chan []*demandOrder
	outputOrdersCh      chan<- []*demandOrder
	lps                 map[string]lp
	minOperatorFeeShare sdk.Dec

	pool orderPool

	batchSize                int
	finalizedCheckerInterval time.Duration
	validation               *config.ValidationConfig
	fulfilledOrdersCh        chan *orderBatch
	subscriberID             string
}

type lp struct {
	address             string
	rollapps            map[string]bool
	denoms              map[string]bool
	maxPrice            sdk.Coins
	minFeePercentage    sdk.Dec
	operatorFeeShare    sdk.Dec
	settlementValidated bool
}

const (
	defaultFinalizedCheckerInterval = 10 * time.Minute
)

type (
	broadcastTxFn func(accName string, msgs ...sdk.Msg) (cosmosclient.Response, error)
	getLPGrantsFn func(ctx context.Context, in *authz.QueryGranteeGrantsRequest, opts ...grpc.CallOption) (*authz.QueryGranteeGrantsResponse, error)
)

func newOrderTracker(
	hubClient cosmosclient.Client,
	policyAddress string,
	minOperatorFeeShare sdk.Dec,
	fullNodeClient *nodeClient,
	fulfilledOrdersCh chan *orderBatch,
	bots map[string]*orderFulfiller,
	subscriberID string,
	batchSize int,
	validation *config.ValidationConfig,
	ordersCh chan<- []*demandOrder,
	logger *zap.Logger,
) *orderTracker {
	azc := authz.NewQueryClient(hubClient.Context())
	return &orderTracker{
		broadcastTx:              hubClient.BroadcastTx,
		getLPGrants:              azc.GranteeGrants,
		policyAddress:            policyAddress,
		minOperatorFeeShare:      minOperatorFeeShare,
		fullNodeClient:           fullNodeClient,
		pool:                     orderPool{orders: make(map[string]*demandOrder)},
		fulfilledOrdersCh:        fulfilledOrdersCh,
		bots:                     bots,
		lps:                      make(map[string]lp),
		batchSize:                batchSize,
		finalizedCheckerInterval: defaultFinalizedCheckerInterval,
		validation:               validation,
		validOrdersCh:            make(chan []*demandOrder),
		outputOrdersCh:           ordersCh,
		logger:                   logger.With(zap.String("module", "order-resolver")),
		subscriberID:             subscriberID,
		fulfilledOrders:          make(map[string]*demandOrder),
	}
}

func (or *orderTracker) start(ctx context.Context) error {
	if err := or.refreshLPs(ctx); err != nil {
		return fmt.Errorf("failed to load LPs: %w", err)
	}

	or.selectOrdersWorker(ctx)

	go or.fulfilledOrdersWorker(ctx)

	return nil
}

func (or *orderTracker) refreshLPs(ctx context.Context) error {
	if err := or.loadLPs(ctx); err != nil {
		return fmt.Errorf("failed to load LPs: %w", err)
	}

	go func() {
		t := time.NewTicker(5 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := or.loadLPs(ctx); err != nil {
					or.logger.Error("failed to load LPs", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

func (or *orderTracker) loadLPs(ctx context.Context) error {
	grants, err := or.getLPGrants(ctx, &authz.QueryGranteeGrantsRequest{
		Grantee: or.policyAddress,
	})
	if err != nil {
		return fmt.Errorf("failed to get LP grants: %w", err)
	}

	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	for _, grant := range grants.Grants {
		if grant.Authorization == nil {
			continue
		}

		g := new(types.FulfillOrderAuthorization)
		if err = proto.Unmarshal(grant.Authorization.Value, g); err != nil {
			return fmt.Errorf("failed to unmarshal grant: %w", err)
		}

		lp := lp{
			address:             grant.Granter,
			rollapps:            make(map[string]bool),
			denoms:              make(map[string]bool),
			maxPrice:            g.MaxPrice,
			minFeePercentage:    g.MinLpFeePercentage.Dec,
			operatorFeeShare:    g.OperatorFeeShare.Dec,
			settlementValidated: g.SettlementValidated,
		}

		// check the operator fee is the minimum for what the operator wants
		if lp.operatorFeeShare.LT(or.minOperatorFeeShare) {
			continue
		}

		for _, rollappID := range g.Rollapps {
			lp.rollapps[rollappID] = true
		}
		for _, denom := range g.Denoms {
			lp.denoms[denom] = true
		}
		or.lps[grant.Granter] = lp
	}

	return nil
}

// Demand orders are first added:
// - in sequencer mode, the first batch is sent to the output channel, and the rest of the orders are added to the pool
// - in p2p and settlement mode, all orders are added to the pool
// Then, the orders are periodically popped (fetched and deleted) from the pool and checked for validity.
// If the order is valid, it is sent to the output channel.
// If the order is not valid, it is added back to the pool.
// After the order validity deadline is expired, the order is removed permanently.
// Once an order is fulfilled, it is removed from the store
func (or *orderTracker) selectOrdersWorker(ctx context.Context) {
	toCheckOrdersCh := make(chan []*demandOrder, or.batchSize)
	go or.pullOrders(ctx, toCheckOrdersCh)
	go or.enqueueValidOrders(ctx, toCheckOrdersCh)
}

func (or *orderTracker) pullOrders(ctx context.Context, toCheckOrdersCh chan []*demandOrder) {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orders := or.pool.popOrders(or.batchSize)
			if len(orders) == 0 {
				continue
			}
			// in "sequencer" mode send the orders directly to be fulfilled,
			// in other modes, send the orders to be checked for validity
			if or.validation.FallbackLevel == config.ValidationModeSequencer {
				or.outputOrdersCh <- orders
			} else {
				toCheckOrdersCh <- orders
			}
		}
	}
}

func (or *orderTracker) enqueueValidOrders(ctx context.Context, toCheckOrdersCh <-chan []*demandOrder) {
	for {
		select {
		case <-ctx.Done():
			return
		case orders := <-toCheckOrdersCh:
			validOrders, retryOrders := or.getValidAndRetryOrders(ctx, orders)
			if len(validOrders) > 0 {
				or.outputOrdersCh <- validOrders
			}
			if len(retryOrders) > 0 {
				or.pool.addOrder(retryOrders...)
			}
		}
	}
}

func (or *orderTracker) getValidAndRetryOrders(ctx context.Context, orders []*demandOrder) (validOrders, invalidOrders []*demandOrder) {
	for _, order := range orders {
		expectedValidationLevel := validationLevelP2P
		if order.settlementValidated {
			expectedValidationLevel = validationLevelSettlement
		}
		valid, err := or.fullNodeClient.BlockValidated(ctx, order.blockHeight, expectedValidationLevel)
		if err != nil {
			or.logger.Error("failed to check validation of block", zap.Error(err))
			continue
		}
		if valid {
			validOrders = append(validOrders, order)
			continue
		}
		if or.isOrderExpired(order) {
			or.logger.Debug("order has expired", zap.String("id", order.id))
			continue
		}
		or.logger.Debug("order is not valid yet", zap.String("id", order.id))
		// order is not valid yet, so add it back to the pool
		invalidOrders = append(invalidOrders, order)
	}
	return
}

func (or *orderTracker) isOrderExpired(order *demandOrder) bool {
	return time.Now().After(order.validDeadline)
}

func (or *orderTracker) addOrder(orders ...*demandOrder) {
	// - in mode "sequencer" we send a batch directly to be fulfilled,
	// and any orders that overflow the batch are added to the pool
	// - in mode "p2p" and "settlement" all orders are added to the pool
	if or.validation.FallbackLevel == config.ValidationModeSequencer {
		var (
			batchToSend []*demandOrder
			batchToPool []*demandOrder
		)
		// send one batch to fulfilling, add the rest to the pool
		for _, o := range orders {
			if len(batchToSend) < or.batchSize {
				batchToSend = append(batchToSend, o)
				continue
			}
			batchToPool = append(batchToPool, o)
		}
		or.outputOrdersCh <- batchToSend
		orders = batchToPool
	}
	or.pool.addOrder(orders...)
}

func (or *orderTracker) fulfilledOrdersWorker(ctx context.Context) {
	for {
		select {
		case batch := <-or.fulfilledOrdersCh:
			if err := or.addFulfilledOrders(batch); err != nil {
				or.logger.Error("failed to add fulfilled orders", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

// addFulfilledOrders adds the fulfilled orders to the fulfilledOrders cache, and removes them from the orderPool.
// It also persists the state to the database.
func (or *orderTracker) addFulfilledOrders(batch *orderBatch) error {
	or.fomu.Lock()
	for _, order := range batch.orders {
		if len(order.amount) == 0 {
			continue
		}
		// add to cache
		or.fulfilledOrders[order.id] = order
		or.pool.removeOrder(order.id) // just in case it's still in the pool

	}
	or.fomu.Unlock()
	return nil
}

func (or *orderTracker) canFulfillOrder(order *demandOrder) bool {
	if or.isOrderFulfilled(order.id) {
		return false
	}
	// we are already processing this order
	if or.isOrderInPool(order.id) {
		return false
	}

	return true
}

func (or *orderTracker) findLPForOrder(order *demandOrder) (*lp, error) {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	lps := or.filterLPsForOrder(order)
	if len(lps) == 0 {
		return nil, fmt.Errorf("no LPs found for order")
	}

	bestLP := selectBestLP(lps)
	if bestLP == nil {
		return nil, fmt.Errorf("LP not found")
	}

	return bestLP, nil
}

func (or *orderTracker) filterLPsForOrder(order *demandOrder) []lp {
	lps := make([]lp, 0, len(or.lps))
	for _, lp := range or.lps {
		// check the fee is at least the minimum for what the lp wants
		operatorFee := sdk.NewDecFromInt(order.fee.Amount).Mul(or.minOperatorFeeShare)
		amountDec := sdk.NewDecFromInt(order.amount[0].Amount.Add(order.fee.Amount))
		minLPFee := amountDec.Mul(lp.minFeePercentage).RoundInt()
		lpFee := order.fee.Amount.Sub(operatorFee.RoundInt())

		if lpFee.LT(minLPFee) {
			continue
		}

		// check the rollapp is allowed
		if len(lp.rollapps) > 0 && !lp.rollapps[order.rollappId] {
			continue
		}

		// check the denom is allowed
		if len(lp.denoms) > 0 && !lp.denoms[order.fee.Denom] {
			continue
		}

		// check the order price does not exceed the max price
		if lp.maxPrice.IsAllPositive() {
			if order.amount.IsAllGT(lp.maxPrice) {
				continue
			}
		}

		lps = append(lps, lp)
	}
	return lps
}

func selectBestLP(lps []lp) *lp {
	if len(lps) == 0 {
		return nil
	}

	sort.Slice(lps, func(i, j int) bool {
		// first criterion: settlementValidated (false comes before true)
		if lps[i].settlementValidated != lps[j].settlementValidated {
			return !lps[i].settlementValidated && lps[j].settlementValidated
		}
		// second criterion: higher operatorFeeShare
		return lps[i].operatorFeeShare.GT(lps[j].operatorFeeShare)
	})

	return &lps[0]
}

func (or *orderTracker) isOrderFulfilled(id string) bool {
	or.fomu.Lock()
	defer or.fomu.Unlock()

	_, ok := or.fulfilledOrders[id]
	return ok
}

func (or *orderTracker) isOrderInPool(id string) bool {
	return or.pool.hasOrder(id)
}

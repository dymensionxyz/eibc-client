package eibc

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/gogoproto/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderTracker struct {
	getBalances    getSpendableBalancesFn
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
	lps                 map[string]*lp
	minOperatorFeeShare sdk.Dec

	pool orderPool

	batchSize              int
	validation             *config.ValidationConfig
	fulfilledOrdersCh      chan *orderBatch
	subscriberID           string
	balanceRefreshInterval time.Duration
}

type (
	getSpendableBalancesFn func(ctx context.Context, in *banktypes.QuerySpendableBalancesRequest, opts ...grpc.CallOption) (*banktypes.QuerySpendableBalancesResponse, error)
	getLPGrantsFn          func(ctx context.Context, in *authz.QueryGranteeGrantsRequest, opts ...grpc.CallOption) (*authz.QueryGranteeGrantsResponse, error)
)

func newOrderTracker(
	hubClient cosmosClient,
	policyAddress string,
	minOperatorFeeShare sdk.Dec,
	fullNodeClient *nodeClient,
	fulfilledOrdersCh chan *orderBatch,
	bots map[string]*orderFulfiller,
	subscriberID string,
	batchSize int,
	validation *config.ValidationConfig,
	ordersCh chan<- []*demandOrder,
	balanceRefreshInterval time.Duration,
	logger *zap.Logger,
) *orderTracker {
	azc := authz.NewQueryClient(hubClient.Context())
	bc := banktypes.NewQueryClient(hubClient.Context())
	return &orderTracker{
		getBalances:            bc.SpendableBalances,
		getLPGrants:            azc.GranteeGrants,
		policyAddress:          policyAddress,
		minOperatorFeeShare:    minOperatorFeeShare,
		fullNodeClient:         fullNodeClient,
		pool:                   orderPool{orders: make(map[string]*demandOrder)},
		fulfilledOrdersCh:      fulfilledOrdersCh,
		bots:                   bots,
		lps:                    make(map[string]*lp),
		batchSize:              batchSize,
		validation:             validation,
		validOrdersCh:          make(chan []*demandOrder),
		outputOrdersCh:         ordersCh,
		logger:                 logger.With(zap.String("module", "order-resolver")),
		subscriberID:           subscriberID,
		balanceRefreshInterval: balanceRefreshInterval,
		fulfilledOrders:        make(map[string]*demandOrder),
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

		resp, err := or.getBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
			Address: grant.Granter,
		})
		if err != nil {
			return fmt.Errorf("failed to get LP balances: %w", err)
		}

		lp := &lp{
			address:  grant.Granter,
			rollapps: make(map[string]rollappCriteria),
			balance:  resp.Balances,
		}

		for _, rollapp := range g.Rollapps {
			// check the operator fee is the minimum for what the operator wants
			if rollapp.OperatorFeeShare.Dec.LT(or.minOperatorFeeShare) {
				continue
			}

			denoms := make(map[string]bool)
			for _, denom := range rollapp.Denoms {
				denoms[denom] = true
			}
			lp.rollapps[rollapp.RollappId] = rollappCriteria{
				rollappID:           rollapp.RollappId,
				denoms:              denoms,
				maxPrice:            rollapp.MaxPrice,
				minFeePercentage:    rollapp.MinLpFeePercentage.Dec,
				operatorFeeShare:    rollapp.OperatorFeeShare.Dec,
				settlementValidated: rollapp.SettlementValidated,
			}
		}

		if len(lp.rollapps) == 0 {
			continue
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
		valid, err := or.fullNodeClient.BlockValidated(ctx, order.rollappId, order.proofHeight, expectedValidationLevel)
		if err != nil {
			or.logger.Error("failed to check validation of block", zap.Error(err))
			continue
		}
		if valid {
			validOrders = append(validOrders, order)
			continue
		}
		if or.isOrderExpired(order) {
			or.releaseAllReservedOrdersFunds(order)
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
	if !or.isRollappSupported(order.rollappId) {
		return false
	}

	if or.isOrderFulfilled(order.id) {
		return false
	}
	// we are already processing this order
	if or.isOrderInPool(order.id) {
		return false
	}

	return true
}

func (or *orderTracker) findLPForOrder(order *demandOrder) error {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	lps := or.filterLPsForOrder(order)
	if len(lps) == 0 {
		return fmt.Errorf("no LPs found for order")
	}

	// randomize the list of LPs to avoid always selecting the same one
	// this is important for the case where multiple LPs have the same operatorFeeShare
	// and the same settlementValidated status
	shuffleLPs(lps)

	bestLP := selectBestLP(lps, order.rollappId)
	if bestLP == nil {
		return fmt.Errorf("LP not found")
	}

	order.lpAddress = bestLP.address
	order.settlementValidated = bestLP.rollapps[order.rollappId].settlementValidated
	order.operatorFeePart = bestLP.rollapps[order.rollappId].operatorFeeShare

	// optimistically deduct from the LP's balance
	bestLP.reserveFunds(order.amount)

	return nil
}

func shuffleLPs(lps []*lp) {
	rand.Shuffle(len(lps), func(i, j int) {
		lps[i], lps[j] = lps[j], lps[i]
	})
}

func (or *orderTracker) filterLPsForOrder(order *demandOrder) []*lp {
	lps := make([]*lp, 0, len(or.lps))
	for _, lp := range or.lps {
		amount := order.amount
		if !lp.hasBalance(amount) {
			continue
		}

		// check the rollapp is allowed
		rollapp, ok := lp.rollapps[order.rollappId]
		if !ok {
			continue
		}

		// check the denom is allowed
		if len(rollapp.denoms) > 0 && !rollapp.denoms[order.fee.Denom] {
			continue
		}

		// check the order price does not exceed the max price
		if rollapp.maxPrice.IsAllPositive() && order.amount.IsAnyGT(rollapp.maxPrice) {
			continue
		}

		// check the fee is at least the minimum for what the lp wants
		operatorFee := sdk.NewDecFromInt(order.fee.Amount).Mul(rollapp.operatorFeeShare).RoundInt()
		amountDec := sdk.NewDecFromInt(order.amount[0].Amount.Add(order.fee.Amount))
		minLPFee := amountDec.Mul(rollapp.minFeePercentage).RoundInt()
		lpFee := order.fee.Amount.Sub(operatorFee)

		if lpFee.LT(minLPFee) {
			continue
		}

		lps = append(lps, lp)
	}
	return lps
}

func selectBestLP(lps []*lp, rollappID string) *lp {
	if len(lps) == 0 {
		return nil
	}

	sort.Slice(lps, func(i, j int) bool {
		// first criterion: settlementValidated (false comes before true)
		if lps[i].rollapps[rollappID].settlementValidated != lps[j].rollapps[rollappID].settlementValidated {
			return !lps[i].rollapps[rollappID].settlementValidated && lps[j].rollapps[rollappID].settlementValidated
		}
		// second criterion: higher operatorFeeShare
		return lps[i].rollapps[rollappID].operatorFeeShare.GT(lps[j].rollapps[rollappID].operatorFeeShare)
	})

	return lps[0]
}

func (or *orderTracker) isRollappSupported(rollappID string) bool {
	_, ok := or.fullNodeClient.rollapps[rollappID]
	return ok
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

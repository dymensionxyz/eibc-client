package eibc

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/eibc-client/config"
)

type orderTracker struct {
	getBalances    getSpendableBalancesFn
	getLPGrants    getLPGrantsFn
	fullNodeClient *nodeClient
	logger         *zap.Logger
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
	toCheckOrdersCh        chan []*demandOrder
	subscriberID           string
	balanceRefreshInterval time.Duration
	validateOrdersInterval time.Duration
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
	subscriberID string,
	batchSize int,
	validation *config.ValidationConfig,
	ordersCh chan<- []*demandOrder,
	balanceRefreshInterval,
	validateOrdersInterval time.Duration,
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
		lps:                    make(map[string]*lp),
		batchSize:              batchSize,
		validation:             validation,
		validOrdersCh:          make(chan []*demandOrder),
		outputOrdersCh:         ordersCh,
		logger:                 logger.With(zap.String("module", "order-resolver")),
		subscriberID:           subscriberID,
		balanceRefreshInterval: balanceRefreshInterval,
		validateOrdersInterval: validateOrdersInterval,
		toCheckOrdersCh:        make(chan []*demandOrder, batchSize),
	}
}

func (or *orderTracker) start(ctx context.Context) error {
	if err := or.loadLPs(ctx); err != nil {
		return fmt.Errorf("failed to load LPs: %w", err)
	}

	go or.lpLoader(ctx)
	go or.balanceRefresher(ctx)
	go or.orderValidator(ctx)
	go or.enqueueValidOrders(ctx)

	return nil
}

func (or *orderTracker) lpLoader(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
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
}

func (or *orderTracker) orderValidator(ctx context.Context) {
	ticker := time.NewTicker(or.validateOrdersInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			or.checkOrders()
		}
	}
}

func (or *orderTracker) checkOrders() {
	orders := or.pool.popOrders(or.batchSize)
	if len(orders) == 0 {
		return
	}
	// in "sequencer" mode send the orders directly to be fulfilled,
	// in other modes, send the orders to be checked for validity
	if or.validation.FallbackLevel == config.ValidationModeSequencer {
		or.outputOrdersCh <- orders
	} else {
		or.toCheckOrdersCh <- orders
	}
}

func (or *orderTracker) enqueueValidOrders(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case orders := <-or.toCheckOrdersCh:
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
			or.pool.removeOrder(order.id)
			validOrders = append(validOrders, order)
			continue
		}
		if or.isOrderExpired(order) {
			or.evictOrder(order)
			or.logger.Debug("order has expired", zap.String("id", order.id))
			continue
		}
		or.logger.Debug("order is not valid yet", zap.String("id", order.id), zap.String("from", order.from))

		if !or.orderFulfillable(order) {
			or.evictOrder(order)
			or.logger.Debug("order is not fulfillable anymore", zap.String("id", order.id))
			continue
		}

		// order is not valid yet, so add it back to the pool
		order.checking = false
		invalidOrders = append(invalidOrders, order)
	}
	return
}

func (or *orderTracker) isOrderExpired(order *demandOrder) bool {
	fmt.Printf("order.id: %s; order.validDeadline: %v\n", order.id, order.validDeadline)
	return time.Now().After(order.validDeadline)
}

func (or *orderTracker) orderFulfillable(order *demandOrder) bool {
	if err := or.findLPForOrder(order); err != nil {
		or.logger.Debug("failed to find LP for order", zap.Error(err), zap.String("order_id", order.id))
		return false
	}

	return true
}

func (or *orderTracker) evictOrder(order *demandOrder) {
	or.pool.removeOrder(order.id)
	or.releaseAllReservedOrdersFunds(order)
}

func (or *orderTracker) trackOrders(orders ...*demandOrder) {
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
	or.pool.upsertOrder(orders...)
	go or.checkOrders()
}

func (or *orderTracker) canFulfillOrder(order *demandOrder) bool {
	if !or.isRollappSupported(order.rollappId) {
		return false
	}

	return true
}

func (or *orderTracker) findLPForOrder(order *demandOrder) error {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	lps, lpMiss := or.filterLPsForOrder(order)
	if len(lps) == 0 {
		return fmt.Errorf("no LPs found for order: %s", strings.Join(lpMiss, "; "))
	}

	if order.lpAddress != "" {
		// check if the LP is still valid
		for _, l := range lps {
			if l.address == order.lpAddress {
				// in case it changed
				order.settlementValidated = l.rollapps[order.rollappId].settlementValidated
				return nil
			}
		}
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
	bestLP.reserveFunds(order.price)

	return nil
}

func shuffleLPs(lps []*lp) {
	rand.Shuffle(len(lps), func(i, j int) {
		lps[i], lps[j] = lps[j], lps[i]
	})
}

func (or *orderTracker) filterLPsForOrder(order *demandOrder) ([]*lp, []string) {
	lps := make([]*lp, 0, len(or.lps))
	lpSkip := make([]string, 0, len(or.lps))

	for _, lp := range or.lps {
		amount := order.price
		if !lp.hasBalance(amount) {
			lpSkip = append(lpSkip, fmt.Sprintf("%s: balance", lp.address))
			continue
		}

		// check the rollapp is allowed
		rollapp, ok := lp.rollapps[order.rollappId]
		if !ok {
			lpSkip = append(lpSkip, fmt.Sprintf("%s: rollapp", lp.address))
			continue
		}

		// check the denom is allowed
		if len(rollapp.denoms) > 0 && !rollapp.denoms[order.fee.Denom] {
			lpSkip = append(lpSkip, fmt.Sprintf("%s: denom", lp.address))
			continue
		}

		// check the order price does not exceed the max price
		if rollapp.maxPrice.IsAllPositive() && order.price.IsAnyGT(rollapp.maxPrice) {
			lpSkip = append(lpSkip, fmt.Sprintf("%s: max_price", lp.address))
			continue
		}

		// check the fee is at least the minimum for what the lp wants
		amountDec := sdk.NewDecFromInt(order.price[0].Amount.Add(order.fee.Amount))
		minFee := amountDec.Mul(rollapp.minFeePercentage).RoundInt()

		if order.fee.Amount.LT(minFee) {
			lpSkip = append(lpSkip, fmt.Sprintf("%s: min_fee", lp.address))
			continue
		}

		lps = append(lps, lp)
	}
	return lps, lpSkip
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

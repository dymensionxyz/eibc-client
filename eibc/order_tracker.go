package eibc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"sort"
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
	resetPoller    func()
	fullNodeClient *nodeClient
	logger         *zap.Logger
	policyAddress  string

	fomu                sync.Mutex
	lpmu                sync.Mutex
	fulfilledOrders     map[string]*demandOrder
	validOrdersCh       chan []*demandOrder
	outputOrdersCh      chan<- []*demandOrder
	processedOrdersCh   chan []orderFulfillResult
	lps                 map[string]*lp
	minOperatorFeeShare sdk.Dec

	pool              orderPool
	failedOrderHashes map[string]string
	fohmu             sync.Mutex

	numFulfillers, maxOrdersPerTx int
	validation                    *config.ValidationConfig
	toCheckOrdersCh               chan []*demandOrder
	subscriberID                  string
	balanceRefreshInterval        time.Duration
	validateOrdersInterval        time.Duration
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
	numFulfillers,
	maxOrdersPerTx int,
	validation *config.ValidationConfig,
	ordersCh chan<- []*demandOrder,
	processedOrdersCh chan []orderFulfillResult,
	balanceRefreshInterval,
	validateOrdersInterval time.Duration,
	resetPoller func(),
	logger *zap.Logger,
) *orderTracker {
	azc := authz.NewQueryClient(hubClient.Context())
	bc := banktypes.NewQueryClient(hubClient.Context())
	return &orderTracker{
		getBalances:            bc.SpendableBalances,
		getLPGrants:            azc.GranteeGrants,
		resetPoller:            resetPoller,
		policyAddress:          policyAddress,
		minOperatorFeeShare:    minOperatorFeeShare,
		fullNodeClient:         fullNodeClient,
		pool:                   orderPool{orders: make(map[string]*demandOrder)},
		failedOrderHashes:      make(map[string]string),
		lps:                    make(map[string]*lp),
		numFulfillers:          numFulfillers,
		maxOrdersPerTx:         maxOrdersPerTx,
		validation:             validation,
		validOrdersCh:          make(chan []*demandOrder),
		outputOrdersCh:         ordersCh,
		processedOrdersCh:      processedOrdersCh,
		logger:                 logger.With(zap.String("module", "order-tracker")),
		subscriberID:           subscriberID,
		balanceRefreshInterval: balanceRefreshInterval,
		validateOrdersInterval: validateOrdersInterval,
		toCheckOrdersCh:        make(chan []*demandOrder, numFulfillers),
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
	go or.manageProcessed()

	return nil
}

func (or *orderTracker) lpLoader(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
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
	orders := or.pool.popOrders(config.NewOrderBufferSize)
	if len(orders) == 0 {
		return
	}

	// sort orders by proof height descending
	slices.SortFunc(orders, func(o1, o2 *demandOrder) int {
		if o1.proofHeight < o2.proofHeight {
			return 1
		} else if o1.proofHeight > o2.proofHeight {
			return -1
		} else {
			return 0
		}
	})

	// in "sequencer" mode send the orders directly to be fulfilled,
	// in other modes, send the orders to be checked for validity
	// if or.validation.FallbackLevel == config.ValidationModeSequencer {
	//	or.outputOrdersCh <- orders
	// } else {
	or.toCheckOrdersCh <- orders
	// }
}

func (or *orderTracker) manageProcessed() {
	for {
		select {
		case orders := <-or.processedOrdersCh:
			for _, orderRes := range orders {
				if orderRes.failedOrderHash != "" {
					or.fohmu.Lock()
					or.failedOrderHashes[orderRes.orderID] = orderRes.failedOrderHash
					or.fohmu.Unlock()
				}
				or.pool.removeOrder(orderRes.orderID)
			}
		}
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
				chunkSize := max(or.maxOrdersPerTx, len(validOrders)/or.numFulfillers)
				slices.Chunk(validOrders, chunkSize)(func(batch []*demandOrder) bool {
					or.outputOrdersCh <- batch
					return true
				})
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
			or.logger.Error("failed to check validation of block", zap.String("order_id", order.id), zap.Error(err))
			continue
		}
		if valid {
			or.pool.setValid(order.id)
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
	/*if or.validation.FallbackLevel == config.ValidationModeSequencer {
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
	}*/
	or.pool.upsertOrder(orders...)
	go or.checkOrders()
}

func (or *orderTracker) canFulfillOrder(order *demandOrder) bool {
	if !or.isRollappSupported(order.rollappId) {
		return false
	}

	or.fohmu.Lock()
	defer or.fohmu.Unlock()

	if failedHash, ok := or.failedOrderHashes[order.id]; ok {
		jsn, _ := json.Marshal(&hashableOrder{ID: order.id, Fee: order.fee})
		hash := fmt.Sprintf("%x", sha256.Sum256(jsn))
		if hash == failedHash {
			return false
		}

		delete(or.failedOrderHashes, order.id)
	}

	return true
}

func (or *orderTracker) findLPForOrder(order *demandOrder) error {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	lps, lpMiss := or.filterLPsForOrder(order)
	if len(lps) == 0 {
		for _, m := range lpMiss {
			or.logger.Debug("LP skipped", zap.String("reason", m))
		}
		return fmt.Errorf("no LPs found for order")
	}

	if order.lpAddress != "" {
		// check if the LP is still valid
		for _, l := range lps {
			if l.address == order.lpAddress {
				// in case it changed
				order.settlementValidated = l.Rollapps[order.rollappId].SettlementValidated
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
	order.settlementValidated = bestLP.Rollapps[order.rollappId].SettlementValidated
	order.operatorFeePart = bestLP.Rollapps[order.rollappId].OperatorFeeShare

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
		// check the rollapp is allowed
		rollapp, ok := lp.Rollapps[order.rollappId]
		if !ok {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s", lp.address, order.rollappId))
			continue
		}

		// check the denom is allowed
		if len(rollapp.Denoms) > 0 && !rollapp.Denoms[order.fee.Denom] {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s; denom: %s", lp.address, order.rollappId, order.fee.Denom))
			continue
		}

		// check the order price does not exceed the max price
		if rollapp.MaxPrice.IsAllPositive() && order.price.IsAnyGT(rollapp.MaxPrice) {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s: max_price: %s", lp.address, order.rollappId, rollapp.MaxPrice.String()))
			continue
		}

		if !rollapp.spendLimit.IsAllGTE(order.price) {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s: spend_limit: %s", lp.address, order.rollappId, rollapp.spendLimit.String()))
			continue
		}

		// check the fee is at least the minimum of what the lp wants
		minFee := sdk.NewDecFromInt(order.amount).Mul(rollapp.MinFeePercentage).TruncateInt()

		if order.fee.Amount.LT(minFee) {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s: min_fee: %s", lp.address, order.rollappId, minFee.String()))
			continue
		}

		if !lp.hasBalance(order.price) {
			lpSkip = append(lpSkip, fmt.Sprintf("lp: %s; rollapp: %s: balance", lp.address, order.rollappId))
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
		if lps[i].Rollapps[rollappID].SettlementValidated != lps[j].Rollapps[rollappID].SettlementValidated {
			return !lps[i].Rollapps[rollappID].SettlementValidated && lps[j].Rollapps[rollappID].SettlementValidated
		}
		// second criterion: higher operatorFeeShare
		return lps[i].Rollapps[rollappID].OperatorFeeShare.GT(lps[j].Rollapps[rollappID].OperatorFeeShare)
	})

	return lps[0]
}

func (or *orderTracker) isRollappSupported(rollappID string) bool {
	_, ok := or.fullNodeClient.rollapps[rollappID]
	return ok
}

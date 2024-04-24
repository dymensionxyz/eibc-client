package main

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/dymension/v3/x/common/types"
	eibctypes "github.com/dymensionxyz/dymension/v3/x/eibc/types"
	rollapptypes "github.com/dymensionxyz/dymension/v3/x/rollapp/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

type demandOrder struct {
	id                string
	price             sdk.Coins
	fee               sdk.Coins
	fulfilledAtHeight uint64
	alertedLowFunds   bool
}

func (oc *orderClient) GetDemandOrders() map[string]*demandOrder {
	oc.domu.Lock()
	defer oc.domu.Unlock()
	return oc.demandOrders
}

func (oc *orderClient) refreshPendingDemandOrders(ctx context.Context) error {
	oc.logger.Debug("refreshing demand orders")

	res, err := oc.getDemandOrdersByStatus(ctx, types.Status_PENDING.String())
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	oc.domu.Lock()
	defer oc.domu.Unlock()

	newCount := 0

	for _, d := range res {
		if _, found := oc.demandOrders[d.Id]; found || d.IsFullfilled {
			continue
		}
		oc.demandOrders[d.Id] = &demandOrder{
			id:    d.Id,
			price: d.Price,
			fee:   d.Fee,
		}
		newCount++
	}

	if newCount > 0 {
		oc.logger.Info("new demand orders", zap.Int("count", newCount))
	}
	return nil
}

func (oc *orderClient) resToDemandOrder(res tmtypes.ResultEvent) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return
	}

	oc.logger.Info("received demand orders", zap.Int("count", len(ids)))

	// packetKeys := res.Events["eibc.packet_key"]
	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	// recipients := res.Events["transfer.recipient"]

	oc.domu.Lock()
	defer oc.domu.Unlock()

	for i, id := range ids {
		if statuses[i] != types.Status_PENDING.String() {
			continue
		}
		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			oc.logger.Error("failed to parse price", zap.Error(err))
			continue
		}
		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			oc.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}
		order := &demandOrder{
			id:    id,
			price: sdk.NewCoins(price),
			fee:   sdk.NewCoins(fee),
		}
		oc.demandOrders[id] = order
	}
}

func (oc *orderClient) fulfillOrders(ctx context.Context) error {
	toFulfillIDBatches, err := oc.prepareDemandOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare demand orders: %w", err)
	}

	for _, toFulfillIDs := range toFulfillIDBatches {
		if len(toFulfillIDs) == 0 {
			return nil
		}

		oc.logger.Info("fulfilling orders", zap.Int("count", len(toFulfillIDs)), zap.Strings("ids", toFulfillIDs))

		if err := oc.fulfillDemandOrders(toFulfillIDs...); err != nil {
			return fmt.Errorf("failed to fulfill order: %w", err)
		}

		// mark the orders as fulfilled
		for _, id := range toFulfillIDs {
			latestHeight, err := oc.getLatestHeight(ctx)
			if err != nil {
				return fmt.Errorf("failed to get latest height: %w", err)
			}
			oc.demandOrders[id].fulfilledAtHeight = uint64(latestHeight)
		}
	}

	return nil
}

func (oc *orderClient) prepareDemandOrders(ctx context.Context) ([][]string, error) {
	oc.domu.Lock()
	defer oc.domu.Unlock()

	// hack to avoid checking balances every time, TODO: implement balance caching
	demandOrders := make([]*demandOrder, 0, len(oc.demandOrders))
	for _, order := range oc.demandOrders {
		if order.fulfilledAtHeight == 0 {
			demandOrders = append(demandOrders, order)
		}
	}
	if len(demandOrders) == 0 {
		return nil, nil
	}

	// TODO: cache balances locally and update from events
	accountBalances, err := oc.getAccountBalances(ctx, oc.account.GetAddress().String())
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	gasBalance := accountBalances.AmountOf(oc.minimumGasBalance.Denom)
	gasDiff := oc.minimumGasBalance.Amount.Sub(gasBalance)

	if gasDiff.IsPositive() {
		oc.alertLowGasBalance(ctx, sdk.NewCoin(oc.minimumGasBalance.Denom, gasDiff))
		return nil, nil // alert once and skip the orders
	}

	toFulfillIDs := make([][]string, len(demandOrders)/oc.maxOrdersPerTx+1)
	batchIdx := 0

outer:
	for i, order := range demandOrders {
		// check if the account has enough balance for all price denoms to fulfill the order
		for _, coin := range order.price {
			balanceForDenom := accountBalances.AmountOf(coin.Denom)
			diff := coin.Amount.Sub(balanceForDenom)

			if diff.IsPositive() { // TODO: configure minimum balance per denom
				oc.alertLowOrderBalance(ctx, order, sdk.NewCoin(coin.Denom, diff))
				// if at least one denom is insufficient, skip the order - TODO: can we do partial fulfillment?
				continue outer
			}
			// deduct the price from the account balance
			accountBalances.Sub(coin)
		}
		toFulfillIDs[batchIdx] = append(toFulfillIDs[batchIdx], order.id)
		// if the batch is full, move to the next batch
		batchIdx = (i + 1) / oc.maxOrdersPerTx
	}

	return toFulfillIDs, nil
}

func (oc *orderClient) getDemandOrdersByStatus(ctx context.Context, status string) ([]*eibctypes.DemandOrder, error) {
	queryClient := eibctypes.NewQueryClient(oc.client.Context())
	resp, err := queryClient.DemandOrdersByStatus(ctx, &eibctypes.QueryDemandOrdersByStatusRequest{
		Status: status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get demand orders: %w", err)
	}
	return resp.DemandOrders, nil
}

func (oc *orderClient) subscribeToPendingDemandOrders(ctx context.Context) error {
	if err := oc.client.RPC.Start(); err != nil {
		return fmt.Errorf("failed to start rpc: %w", err)
	}

	const query = "eibc.is_fulfilled='false'"

	resCh, err := oc.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				oc.resToDemandOrder(res)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (oc *orderClient) fulfillDemandOrders(demandOrderID ...string) error {
	msgs := make([]sdk.Msg, len(demandOrderID))

	for i, id := range demandOrderID {
		msgs[i] = &eibctypes.MsgFulfillOrder{
			OrderId:          id,
			FulfillerAddress: oc.account.GetAddress().String(),
		}
	}

	_, err := oc.client.BroadcastTx(oc.accountName, msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return nil
}

func (oc *orderClient) getDisputePeriodInBlocks(ctx context.Context) (uint64, error) {
	queryClient := rollapptypes.NewQueryClient(oc.client.Context())
	resp, err := queryClient.Params(ctx, &rollapptypes.QueryParamsRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to get dispute period: %w", err)
	}
	return resp.Params.DisputePeriodInBlocks, nil
}

func (oc *orderClient) getLatestHeight(ctx context.Context) (int64, error) {
	status, err := oc.client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

func (oc *orderClient) cleanup() error {
	oc.domu.Lock()
	defer oc.domu.Unlock()

	cleanupCount := 0

	for id, order := range oc.demandOrders {
		// cleanup fulfilled and credited demand orders
		// if order.credited {
		if order.fulfilledAtHeight > 0 {
			delete(oc.demandOrders, id)
			cleanupCount++
		}
	}

	if cleanupCount > 0 {
		oc.logger.Info("cleaned up fulfilled orders", zap.Int("count", cleanupCount))
	}
	return nil
}

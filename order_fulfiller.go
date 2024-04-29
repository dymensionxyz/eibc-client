package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	eibctypes "github.com/dymensionxyz/dymension/v3/x/eibc/types"
	rollapptypes "github.com/dymensionxyz/dymension/v3/x/rollapp/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderFulfiller struct {
	accountSvc *accountService
	client     cosmosclient.Client
	logger     *zap.Logger

	orderDisputePeriodInBlocks uint64
}

func newOrderFulfiller(accountSvc *accountService, client cosmosclient.Client, logger *zap.Logger) *orderFulfiller {
	return &orderFulfiller{
		accountSvc: accountSvc,
		client:     client,
		logger:     logger.With(zap.String("module", "order-fulfiller"), zap.String("name", accountSvc.accountName)),
	}
}

func (ol *orderFulfiller) fulfillOrders(
	ctx context.Context,
	toFulfillOrders chan []*demandOrder,
	failedOrderIDs chan<- []string,
) {

	retryQueueCh := make(chan []*demandOrder, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case batch := <-retryQueueCh:
				time.Sleep(1 * time.Second) // TODO: make it nicer
				ol.logger.Debug(
					"sending orders for retry",
					zap.String("bot-name", ol.accountSvc.accountName),
					zap.Int("count", len(batch)),
				)
				select {
				case toFulfillOrders <- batch:
				default:
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-toFulfillOrders:
			coins := sdk.NewCoins()

			for _, order := range batch {
				coins = coins.Add(order.price...)
			}

			ol.logger.Debug("ensuring balances for orders")

			ensuredDenoms, err := ol.accountSvc.ensureBalances(ctx, coins)
			if err != nil {
				ol.logger.Error("failed to ensure balances", zap.Error(err))
				return
			}

			leftoverBatch := make([]*demandOrder, 0, len(batch))
			ids := make([]string, 0, len(batch))

		outer:
			for _, order := range batch {
				for _, price := range order.price {
					if !slices.Contains(ensuredDenoms, price.Denom) {
						leftoverBatch = append(leftoverBatch, order)
						continue outer
					}

					ids = append(ids, order.id)
				}
			}

			if len(ids) == 0 {
				ol.logger.Debug(
					"no orders to fulfill",
					zap.String("bot-name", ol.accountSvc.accountName),
					zap.Int("count", len(leftoverBatch)),
				)
				retryQueueCh <- leftoverBatch
				continue
			}

			ol.logger.Info("fulfilling orders", zap.Int("count", len(ids)))

			if err := ol.fulfillDemandOrders(ids...); err != nil {
				ol.logger.Error("failed to fulfill orders", zap.Error(err), zap.Strings("ids", ids))
				failedOrderIDs <- ids
				continue
			}

			// mark the orders as fulfilled
			/*for _, id := range ids {
				latestHeight, err := ol.getLatestHeight(ctx)
				if err != nil {
					return fmt.Errorf("failed to get latest height: %w", err)
				}
				ol.demandOrders[id].fulfilledAtHeight = uint64(latestHeight)
			}*/
		}
	}
}

func (ol *orderFulfiller) fulfillDemandOrders(demandOrderID ...string) error {
	msgs := make([]sdk.Msg, len(demandOrderID))

	for i, id := range demandOrderID {
		msgs[i] = &eibctypes.MsgFulfillOrder{
			OrderId:          id,
			FulfillerAddress: ol.accountSvc.account.GetAddress().String(),
		}
	}

	_, err := ol.client.BroadcastTx(ol.accountSvc.accountName, msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return nil
}

func (ol *orderFulfiller) getDisputePeriodInBlocks(ctx context.Context) (uint64, error) {
	queryClient := rollapptypes.NewQueryClient(ol.client.Context())
	resp, err := queryClient.Params(ctx, &rollapptypes.QueryParamsRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to get dispute period: %w", err)
	}
	return resp.Params.DisputePeriodInBlocks, nil
}

// not in use currently
func (ol *orderFulfiller) disputePeriodUpdater(ctx context.Context, disputePeriodRefreshInterval time.Duration) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(disputePeriodRefreshInterval):
				var err error
				// TODO: mutex?
				ol.orderDisputePeriodInBlocks, err = ol.getDisputePeriodInBlocks(ctx)
				if err != nil {
					ol.logger.Error("failed to refresh dispute period", zap.Error(err))
				}
			}
		}
	}()
}

func (ol *orderFulfiller) getLatestHeight(ctx context.Context) (int64, error) {
	status, err := ol.client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

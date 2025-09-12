package eibc

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	types "github.com/dymensionxyz/eibc-client/types/eibc"
)

type orderFinalizer struct {
	accountName  string
	accountAddr  string
	queryService *queryService
	client       cosmosClient
	rollapps     []string
	interval     time.Duration
	logger       *zap.Logger
}

func newFinalizer(
	accountName, accountAddr string,
	queryService *queryService,
	cClient cosmosClient,
	rollapps []string,
	interval time.Duration,
	logger *zap.Logger,
) *orderFinalizer {
	return &orderFinalizer{
		accountName:  accountName,
		accountAddr:  accountAddr,
		queryService: queryService,
		client:       cClient,
		rollapps:     rollapps,
		interval:     interval,
		logger:       logger.With(zap.String("module", "order-finalizer")),
	}
}

func (f *orderFinalizer) start(ctx context.Context) error {
	if err := f.finalizeClaimableOrders(ctx); err != nil {
		return fmt.Errorf("could not finalize demand orders: %w", err)
	}

	go func() {
		for c := time.Tick(f.interval); ; <-c {
			select {
			case <-ctx.Done():
				return
			default:
				if err := f.finalizeClaimableOrders(ctx); err != nil {
					f.logger.Error("failed to finalize demand orders", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (f *orderFinalizer) finalizeClaimableOrders(ctx context.Context) error {
	claimableOrders, err := f.queryService.getClaimableDemandOrders(ctx, f.rollapps, f.accountAddr)
	if err != nil {
		return fmt.Errorf("could not get claimable demand orders: %w", err)
	}

	if len(claimableOrders) == 0 {
		return nil
	}

	if err := f.finalizeOrders(claimableOrders); err != nil {
		return fmt.Errorf("could not finalize demand orders: %w", err)
	}

	return nil
}

func (f *orderFinalizer) finalizeOrders(orders []Order) error {
	msgs := make([]sdk.Msg, 0, len(orders))
	for _, order := range orders {
		msg := types.MsgFinalizePacketByPacketKey{
			Sender:    f.accountAddr,
			PacketKey: base64.StdEncoding.EncodeToString([]byte(order.TrackingPacketKey)),
		}
		if err := msg.ValidateBasic(); err != nil {
			f.logger.Error("failed to validate finalize packet", zap.Error(err))
			continue
		}
		msgs = append(msgs, &msg)
	}

	f.logger.Debug("finalizing demand orders", zap.Int("count", len(msgs)))

	rsp, err := f.client.BroadcastTx(f.accountName, msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	_, err = waitForTx(f.client, rsp.TxHash)
	if err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	f.logger.Debug("finalized demand orders", zap.Int("count", len(msgs)))

	return nil
}

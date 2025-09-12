package eibc

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/types"
	eibc "github.com/dymensionxyz/eibc-client/types/eibc"
)

const (
	queryServiceBatchSizeLimit = 10 // if this is higher, could run into insufficient fee error
)

// queryService provides a centralized service for querying blockchain data
type queryService struct {
	rollappClient   types.QueryClient
	eibcOrderClient eibc.QueryClient
	logger          *zap.Logger
}

func newQueryService(clientCtx client.Context, logger *zap.Logger) *queryService {
	return &queryService{
		rollappClient:   types.NewQueryClient(clientCtx),
		eibcOrderClient: eibc.NewQueryClient(clientCtx),
		logger:          logger.With(zap.String("module", "query-service")),
	}
}

// getClaimableDemandOrders retrieves all claimable demand orders for the given operator across all rollapps
func (qs *queryService) getClaimableDemandOrders(
	ctx context.Context,
	rollapps []string,
	operatorAddr string,
) ([]Order, error) {
	var demandOrders []Order
	for _, rollapp := range rollapps {
		orders, err := qs.getClaimableRollappDemandOrders(
			ctx,
			rollapp,
			eibc.RollappPacket_ON_RECV,
			operatorAddr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get claimable demand orders: %w", err)
		}
		demandOrders = append(demandOrders, orders...)

		orders, err = qs.getClaimableRollappDemandOrders(
			ctx,
			rollapp,
			eibc.RollappPacket_ON_TIMEOUT,
			operatorAddr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get claimable demand orders: %w", err)
		}
		demandOrders = append(demandOrders, orders...)

		orders, err = qs.getClaimableRollappDemandOrders(
			ctx,
			rollapp,
			eibc.RollappPacket_ON_ACK,
			operatorAddr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get claimable demand orders: %w", err)
		}
		demandOrders = append(demandOrders, orders...)
	}

	return demandOrders, nil
}

// getClaimableRollappDemandOrders retrieves claimable demand orders for a specific rollapp and packet type
func (qs *queryService) getClaimableRollappDemandOrders(
	ctx context.Context,
	rollappId string,
	typ eibc.RollappPacket_Type,
	operatorAddr string,
) ([]Order, error) {
	var lastFinalizedHeight uint64 = 0
	lastHeightResp, err := qs.rollappClient.LatestHeight(ctx, &types.QueryGetLatestHeightRequest{
		RollappId: rollappId,
		Finalized: true,
	})
	if err == nil {
		lastFinalizedHeight = lastHeightResp.Height
	}

	resp, err := qs.eibcOrderClient.DemandOrdersByStatus(
		ctx,
		&eibc.QueryDemandOrdersByStatusRequest{
			Status:           eibc.Status_PENDING,
			Type:             typ,
			RollappId:        rollappId,
			FulfillmentState: eibc.FulfillmentState_FULFILLED,
			Pagination: &query.PageRequest{
				Limit: queryServiceBatchSizeLimit,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get fulfilled demand orders: %w", err)
	}

	var orders []Order
	for _, order := range resp.DemandOrders {
		if operatorAddr != order.FulfillerAddress {
			continue
		}

		if order.Fee == nil || order.Fee.IsAnyNil() || !order.Fee.IsAllPositive() {
			continue
		}

		proofHeight := parseProofHeight(order.TrackingPacketKey, order.Id, qs.logger)

		if proofHeight > 0 && lastFinalizedHeight < proofHeight {
			continue
		}

		orders = append(orders, Order{
			EibcOrderId:       order.Id,
			Amount:            order.Price[0].Amount.Add(order.Fee[0].Amount).String(),
			Price:             order.Price[0].Amount.String(),
			Fee:               order.Fee[0].String(),
			RollappId:         order.RollappId,
			ProofHeight:       fmt.Sprint(proofHeight),
			BlockHeight:       "0",
			TrackingPacketKey: order.TrackingPacketKey,
		})
	}

	return orders, nil
}

// getRollappPendingDemandOrders retrieves pending unfulfilled demand orders for a specific rollapp
func (qs *queryService) getRollappPendingDemandOrders(
	ctx context.Context,
	rollappId string,
	typ eibc.RollappPacket_Type,
) ([]Order, error) {
	var lastFinalizedHeight uint64 = 0
	lastHeightResp, err := qs.rollappClient.LatestHeight(ctx, &types.QueryGetLatestHeightRequest{
		RollappId: rollappId,
		Finalized: true,
	})
	if err == nil {
		lastFinalizedHeight = lastHeightResp.Height
	}

	resp, err := qs.eibcOrderClient.DemandOrdersByStatus(
		ctx,
		&eibc.QueryDemandOrdersByStatusRequest{
			Status:           eibc.Status_PENDING,
			Type:             typ,
			RollappId:        rollappId,
			FulfillmentState: eibc.FulfillmentState_UNFULFILLED,
			Pagination: &query.PageRequest{
				Limit: 100000,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get demand orders: %w", err)
	}

	var orders []Order
	for _, order := range resp.DemandOrders {
		if order.FulfillerAddress != "" {
			continue
		}
		if order.Fee == nil || order.Fee.IsAnyNil() || !order.Fee.IsAllPositive() {
			continue
		}

		proofHeight := parseProofHeight(order.TrackingPacketKey, order.Id, qs.logger)

		if proofHeight > 0 && lastFinalizedHeight < proofHeight {
			continue
		}

		orders = append(orders, Order{
			EibcOrderId:       order.Id,
			Amount:            order.Price[0].Amount.Add(order.Fee[0].Amount).String(),
			Price:             order.Price[0].Amount.String(),
			Fee:               order.Fee[0].String(),
			RollappId:         order.RollappId,
			ProofHeight:       fmt.Sprint(proofHeight),
			BlockHeight:       "0",
			TrackingPacketKey: order.TrackingPacketKey,
		})
	}

	return orders, nil
}

// parseProofHeight extracts the proof height from tracking key
func parseProofHeight(trackingKey, orderId string, logger *zap.Logger) uint64 {
	proofHeightEndian := strings.Split(trackingKey, "/")[2]

	defer func() {
		if r := recover(); r != nil {
			logger.Debug(
				"failed to parse proof height",
				zap.String("proof_height", proofHeightEndian),
				zap.String("order_id", orderId),
			)
		}
	}()

	return sdk.BigEndianToUint64([]byte(proofHeightEndian))
}

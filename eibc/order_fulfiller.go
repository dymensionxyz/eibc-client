package eibc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderFulfiller struct {
	account             account
	policyAddress       string
	operatorAddress     string
	client              cosmosClient
	logger              *zap.Logger
	FulfillDemandOrders func(demandOrder ...*demandOrder) error

	releaseAllReservedOrdersFunds func(demandOrder ...*demandOrder)
	debitAllReservedOrdersFunds   func(demandOrder ...*demandOrder)
	newOrdersCh                   chan []*demandOrder
	fulfilledOrdersCh             chan<- *orderBatch
}

type cosmosClient interface {
	BroadcastTx(accountName string, msgs ...sdk.Msg) (cosmosclient.Response, error)
	Context() client.Context
}

func newOrderFulfiller(
	newOrdersCh chan []*demandOrder,
	fulfilledOrdersCh chan<- *orderBatch,
	client cosmosClient,
	acc account,
	policyAddress string,
	operatorAddress string,
	releaseAllReservedOrdersFunds func(demandOrder ...*demandOrder),
	debitAllReservedOrdersFunds func(demandOrder ...*demandOrder),
	logger *zap.Logger,
) *orderFulfiller {
	o := &orderFulfiller{
		account:                       acc,
		policyAddress:                 policyAddress,
		operatorAddress:               operatorAddress,
		client:                        client,
		fulfilledOrdersCh:             fulfilledOrdersCh,
		newOrdersCh:                   newOrdersCh,
		releaseAllReservedOrdersFunds: releaseAllReservedOrdersFunds,
		debitAllReservedOrdersFunds:   debitAllReservedOrdersFunds,
		logger: logger.With(zap.String("module", "order-fulfiller"),
			zap.String("bot-name", acc.Name), zap.String("address", acc.Address)),
	}
	o.FulfillDemandOrders = o.fulfillAuthorizedDemandOrders
	return o
}

// add command that creates all the bots to be used?

func buildBot(
	acc account,
	operatorAddress string,
	logger *zap.Logger,
	cfg config.BotConfig,
	clientCfg config.ClientConfig,
	newOrderCh chan []*demandOrder,
	fulfilledCh chan *orderBatch,
	releaseAllReservedOrdersFunds func(demandOrder ...*demandOrder),
	debitAllReservedOrdersFunds func(demandOrder ...*demandOrder),
) (*orderFulfiller, error) {
	cosmosClient, err := cosmosclient.New(config.GetCosmosClientOptions(clientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %s;err: %w", acc.Name, err)
	}

	return newOrderFulfiller(
		newOrderCh,
		fulfilledCh,
		cosmosClient,
		acc,
		cfg.PolicyAddress,
		operatorAddress,
		releaseAllReservedOrdersFunds,
		debitAllReservedOrdersFunds,
		logger,
	), nil
}

func (ol *orderFulfiller) start(ctx context.Context) error {
	ol.logger.Info("starting fulfiller...")

	ol.fulfillOrders(ctx)
	return nil
}

func (ol *orderFulfiller) fulfillOrders(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case orders := <-ol.newOrdersCh:
			if err := ol.processBatch(orders); err != nil {
				ol.logger.Error("failed to process batch", zap.Error(err))
			}
		}
	}
}

func (ol *orderFulfiller) processBatch(batch []*demandOrder) error {
	if len(batch) == 0 {
		ol.logger.Debug("no orders to fulfill")
		return nil
	}

	var ids []string
	for _, order := range batch {
		ids = append(ids, order.id)
	}

	ol.logger.Info("fulfilling orders", zap.Strings("ids", ids))

	if err := ol.FulfillDemandOrders(batch...); err != nil {
		ol.releaseAllReservedOrdersFunds(batch...)
		return fmt.Errorf("failed to fulfill orders: ids: %v; %w", ids, err)
	} else {
		ol.debitAllReservedOrdersFunds(batch...)
	}

	ol.logger.Info("orders fulfilled", zap.Strings("ids", ids))

	go func() {
		if len(ids) == 0 {
			return
		}

		ol.fulfilledOrdersCh <- &orderBatch{
			orders:    batch,
			fulfiller: ol.account.Address, // TODO
		}
	}()

	return nil
}

/*
1. dymd tx eibc fulfill-order-authorized 388cedaafbe9ea05c5b6422970005d4a9cb13b2b679afedb99aa82ccff8784aa 10 --rollapp-id rollappwasme_1235-1 --fulfiller-address dym1s5y26zt0msaypsafujrltq7f0h04zzu0e8q5kr --operator-address dym1qhxedstgx9fv3zmjuj687y6lh5cwm9czhqajhw --price 1000adym --fulfiller-fee-part 0.4 --settlement-validated --from alex --generate-only > tx.json
2. dymd tx authz exec tx.json --from dym1c799jddmlz7segvg6jrw6w2k6svwafganjdznard3tc74n7td7rqrx4c5e --fees 1dym -y --generate-only > tx_exec.json
3. dymd tx group submit-proposal proposal.json --from xela --fees 1dym --exec try --gas auto --fee-granter dym1qhxedstgx9fv3zmjuj687y6lh5cwm9czhqajhw -y
*/
func (ol *orderFulfiller) fulfillAuthorizedDemandOrders(demandOrder ...*demandOrder) error {
	fulfillMsgs := make([]sdk.Msg, len(demandOrder))

	for i, order := range demandOrder {
		fulfillMsgs[i] = types.NewMsgFulfillOrderAuthorized(
			order.id,
			order.rollappId,
			order.lpAddress,
			ol.operatorAddress,
			order.fee.Amount.String(),
			order.amount,
			order.operatorFeePart,
			order.settlementValidated,
		)
	}

	// bech32 decode the policy address
	_, policyAddress, err := bech32.DecodeAndConvert(ol.policyAddress)
	if err != nil {
		return fmt.Errorf("failed to decode policy address: %w", err)
	}

	authzMsg := authz.NewMsgExec(policyAddress, fulfillMsgs)

	proposalMsg, err := types.NewMsgSubmitProposal(
		ol.policyAddress,
		[]string{ol.account.Address},
		[]sdk.Msg{&authzMsg},
		"== Fulfill Order ==",
		types.Exec_EXEC_TRY,
		"fulfill-order-authorized",
		"fulfill-order-authorized",
	)
	if err != nil {
		return fmt.Errorf("failed to create proposal message: %w", err)
	}

	rsp, err := ol.client.BroadcastTx(ol.account.Name, proposalMsg)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	ol.logger.Info("broadcast tx", zap.String("tx-hash", rsp.TxHash))

	resp, err := waitForTx(ol.client, rsp.TxHash)
	if err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	var presp []proposalResp
	if err = json.Unmarshal([]byte(resp.TxResponse.RawLog), &presp); err != nil {
		return fmt.Errorf("failed to unmarshal tx response: %w", err)
	}

	// hack to extract error from logs
	for _, p := range presp {
		for _, ev := range p.Events {
			if ev.Type == "cosmos.group.v1.EventExec" {
				for _, attr := range ev.Attributes {
					if attr.Key == "logs" && strings.Contains(attr.Value, "proposal execution failed") {
						theErr := ""
						parts := strings.Split(attr.Value, " : ")
						if len(parts) > 1 {
							theErr = parts[1]
						} else {
							theErr = attr.Value
						}
						return fmt.Errorf("proposal execution failed: %s", theErr)
					}
				}
			}
		}
	}

	ol.logger.Info("tx executed", zap.String("tx-hash", rsp.TxHash))

	return nil
}

type proposalResp struct {
	MsgIndex int `json:"msg_index"`
	Events   []struct {
		Type       string `json:"type"`
		Attributes []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"attributes"`
	} `json:"events"`
}

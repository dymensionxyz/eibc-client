package main

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type slacker struct {
	*slack.Client // TODO: abstract this out
	channelID     string
	enabled       bool
}

func (oc *orderClient) begOnSlack(ctx context.Context, orderID string, coin sdk.Coin, chainID, node string) (string, error) {
	if !oc.slack.enabled {
		oc.logger.Debug("Slack is disabled")
		return "", nil
	}

	address := oc.account.GetAddress().String()
	oc.logger.With(
		zap.String("orderID", orderID),
		zap.String("denom", coin.Denom),
		zap.String("amount", coin.Amount.String()),
		zap.String("address", address),
	).Debug("Slack post @poor-bots")

	message := fmt.Sprintf("Please sir, send %s to my account %s, so I can fulfill order '%s'. I'm on chain %s, on node %s",
		coin.String(), address, orderID, chainID, node)

	if orderID == "gas" {
		message = fmt.Sprintf("Please sir, send %s to my account %s, so I have enough gas to continue fulfilling orders. I'm on chain %s, on node %s",
			coin.String(), address, chainID, node)
	}

	respChannel, respTimestamp, err := oc.slack.PostMessageContext(
		ctx,
		oc.slack.channelID,
		slack.MsgOptionText(message, false),
	)
	if err != nil {
		return "", err
	}

	oc.logger.With(
		zap.String("channel", respChannel),
		zap.String("timestamp", respTimestamp),
	).Debug("Slack message successfully sent")

	time.Sleep(time.Second * 5)

	_, _, _ = oc.slack.PostMessageContext(
		ctx,
		oc.slack.channelID,
		slack.MsgOptionText("I can wait...", false),
	)
	return respTimestamp, nil
}

func (oc *orderClient) alertLowOrderBalance(ctx context.Context, order *demandOrder, coin sdk.Coin) {
	// this is lost after a restart
	if order.alertedLowFunds {
		return
	}

	oc.logger.Info("Low balance to fulfill order", zap.String("orderID", order.id), zap.String("balance", coin.String()))

	if _, err := oc.begOnSlack(ctx, order.id, coin, oc.chainID, oc.node); err != nil {
		oc.logger.Error("failed to bed on slack", zap.Error(err))
	}
	order.alertedLowFunds = true
}

func (oc *orderClient) alertLowGasBalance(ctx context.Context, coin sdk.Coin) {
	if oc.alertedLowGas {
		return
	}

	oc.logger.Warn("Low gas balance", zap.String("balance", coin.String()))

	if _, err := oc.begOnSlack(ctx, "gas", coin, oc.chainID, oc.node); err != nil {
		oc.logger.Error("failed to bed on slack", zap.Error(err))
	}
	oc.alertedLowGas = true
}

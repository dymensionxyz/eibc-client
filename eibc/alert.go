package eibc

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/slack-go/slack"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
)

type slacker struct {
	*slack.Client // TODO: abstract this out
	channelID     string
	enabled       bool
	logger        *zap.Logger
}

func newSlacker(config config.SlackConfig, logger *zap.Logger) *slacker {
	return &slacker{
		Client:    slack.New(config.AppToken),
		channelID: config.ChannelID,
		enabled:   config.Enabled,
		logger:    logger.With(zap.String("module", "slack")),
	}
}

func (oc *slacker) begOnSlack(
	ctx context.Context,
	address string,
	coin, balance sdk.Coin,
	chainID, node string,
) (string, error) {
	if !oc.enabled {
		oc.logger.Debug("Slack is disabled")
		return "", nil
	}

	oc.logger.With(
		zap.String("amount", coin.String()),
		zap.String("balance", balance.String()),
		zap.String("Address", address),
	).Debug("Slack post @poor-bots")

	message := fmt.Sprintf("Please sir, send %s to my account %s. I'm on chain '%s', on node %s and I only have %s.",
		coin.String(), address, chainID, node, balance.String())

	respChannel, respTimestamp, err := oc.PostMessageContext(
		ctx,
		oc.channelID,
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

	_, _, _ = oc.PostMessageContext(
		ctx,
		oc.channelID,
		slack.MsgOptionText("I can wait...", false),
	)
	return respTimestamp, nil
}

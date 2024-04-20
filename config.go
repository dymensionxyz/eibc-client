package main

import (
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type Config struct {
	KeyringBackend string `mapstructure:"keyring_backend" default:"test"`
	AccountName    string `mapstructure:"account_name"`
	Mnemonic       string `mapstructure:"mnemonic"`
	NodeAddress    string `mapstructure:"node_address"`
	ChainID        string `mapstructure:"chain_id"`
	GasPrices      string `mapstructure:"gas_prices"`
	GasFees        string `mapstructure:"gas_fees"`

	SlackConfig slackConfig `mapstructure:"slack"`
}

type slackConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	AppToken  string `mapstructure:"app_token"`
	ChannelID string `mapstructure:"channel_id"`
}

const (
	// nodeAddress = "https://rpc.hwpd.noisnemyd.xyz:443"
	nodeAddress           = "http://localhost:36657"
	chainID               = "dymension_100-1"
	hubAddressPrefix      = "dym"
	pubKeyPrefix          = "pub"
	defaultGasLimit       = 300000
	defaultGasPrices      = "2000000000adym"
	minimumDymBalance     = "997863676000000000adym"
	testKeyringBackend    = "test"
	defaultMaxOrdersPerTx = 10

	defaultRefreshInterval       = 30 * time.Second
	defaultFulfillInterval       = 2 * time.Second
	defaultCleanupInterval       = 3600 * time.Second
	defaultDisputePeriodInterval = 10 * time.Hour
	defaultBalanceCheckInterval  = 30 * time.Second
)

func getCosmosClientOptions(config Config) []cosmosclient.Option {
	options := []cosmosclient.Option{
		cosmosclient.WithAddressPrefix(hubAddressPrefix),
		cosmosclient.WithBroadcastMode(flags.BroadcastBlock),
		cosmosclient.WithNodeAddress(config.NodeAddress),
		cosmosclient.WithFees(config.GasFees),
		cosmosclient.WithGasLimit(defaultGasLimit),
		cosmosclient.WithGasPrices(config.GasPrices),
		cosmosclient.WithKeyringBackend(cosmosaccount.KeyringBackend(config.KeyringBackend)),
	}
	return options
}

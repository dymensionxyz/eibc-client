package main

import (
	"log"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/google/uuid"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type Config struct {
	KeyringBackend    string `mapstructure:"keyring_backend"`
	HomeDir           string `mapstructure:"home_dir"`
	AccountName       string `mapstructure:"account_name"`
	Mnemonic          string `mapstructure:"mnemonic"`
	NodeAddress       string `mapstructure:"node_address"`
	ChainID           string `mapstructure:"chain_id"`
	GasPrices         string `mapstructure:"gas_prices"`
	GasFees           string `mapstructure:"gas_fees"`
	MinimumGasBalance string `mapstructure:"minimum_gas_balance"`

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
	nodeAddress              = "http://localhost:36657"
	chainID                  = "dymension_100-1"
	hubAddressPrefix         = "dym"
	pubKeyPrefix             = "pub"
	defaultGasLimit          = 300000
	defaultGasDenom          = "adym"
	defaultGasPrices         = "2000000000" + defaultGasDenom
	defaultMinimumGasBalance = "40000000000" + defaultGasDenom
	testKeyringBackend       = "test"
	defaultMaxOrdersPerTx    = 3

	mnemonicEntropySize = 256

	defaulOrdertRefreshInterval         = 30 * time.Second
	defaultOrderFulfillInterval         = 5 * time.Second
	defaultOrderCleanupInterval         = 3600 * time.Second
	defaultDisputePeriodRefreshInterval = 10 * time.Hour
)

func initConfig() {
	// Set default values
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	defaultHomeDir := home + "/.dymension"

	viper.SetDefault("keyring_backend", testKeyringBackend)
	viper.SetDefault("keyring_dir", defaultHomeDir)
	viper.SetDefault("home_dir", defaultHomeDir)
	viper.SetDefault("node_address", nodeAddress)
	viper.SetDefault("chain_id", chainID)
	viper.SetDefault("gas_prices", defaultGasPrices)
	viper.SetDefault("account_name", "hub_"+uuid.Must(uuid.NewRandom()).String()[:4])
	viper.SetDefault("slack.enabled", true)
	viper.SetDefault("slack.channel_id", "poor-bots")
	viper.SetDefault("minimum_gas_balance", defaultMinimumGasBalance)

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".order-client")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func getCosmosClientOptions(config Config) []cosmosclient.Option {
	options := []cosmosclient.Option{
		cosmosclient.WithAddressPrefix(hubAddressPrefix),
		cosmosclient.WithHome(config.HomeDir),
		cosmosclient.WithBroadcastMode(flags.BroadcastBlock),
		cosmosclient.WithNodeAddress(config.NodeAddress),
		cosmosclient.WithFees(config.GasFees),
		cosmosclient.WithGasLimit(defaultGasLimit),
		cosmosclient.WithGasPrices(config.GasPrices),
		cosmosclient.WithKeyringBackend(cosmosaccount.KeyringBackend(config.KeyringBackend)),
	}
	return options
}

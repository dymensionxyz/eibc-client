package main

import (
	"log"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type Config struct {
	HomeDir                      string        `mapstructure:"home_dir"`
	NodeAddress                  string        `mapstructure:"node_address"`
	GasPrices                    string        `mapstructure:"gas_prices"`
	GasFees                      string        `mapstructure:"gas_fees"`
	MinimumGasBalance            string        `mapstructure:"minimum_gas_balance"`
	OrderRefreshInterval         time.Duration `mapstructure:"order_refresh_interval"`
	OrderCleanupInterval         time.Duration `mapstructure:"order_cleanup_interval"`
	DisputePeriodRefreshInterval time.Duration `mapstructure:"dispute_period_refresh_interval"`

	Whale whaleConfig `mapstructure:"whale"`
	Bots  botConfig   `mapstructure:"bots"`

	LogLevel    string      `mapstructure:"log_level"`
	SlackConfig slackConfig `mapstructure:"slack"`
}

type botConfig struct {
	NumberOfBots   int                          `mapstructure:"number_of_bots"`
	KeyringBackend cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir     string                       `mapstructure:"keyring_dir"`
	TopUpFactor    uint64                       `mapstructure:"top_up_factor"`
	MaxOrdersPerTx int                          `mapstructure:"max_orders_per_tx"`
}

type whaleConfig struct {
	AccountName    string                       `mapstructure:"account_name"`
	KeyringBackend cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir     string                       `mapstructure:"keyring_dir"`
	AllowedDenoms  []string                     `mapstructure:"allowed_denoms"`
}

type slackConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	AppToken  string `mapstructure:"app_token"`
	ChannelID string `mapstructure:"channel_id"`
}

const (
	defaultNodeAddress       = "http://localhost:36657"
	hubAddressPrefix         = "dym"
	pubKeyPrefix             = "pub"
	defaultLogLevel          = "info"
	defaultGasLimit          = 300000
	defaultGasDenom          = "adym"
	defaultGasPrices         = "2000000000" + defaultGasDenom
	defaultMinimumGasBalance = "40000000000" + defaultGasDenom
	testKeyringBackend       = "test"

	botNamePrefix                       = "bot-"
	whaleAccountName                    = "client"
	defaultBotTopUpFactor               = 2
	defaultNumberOfBots                 = 1
	newOrderBufferSize                  = 100
	defaultMaxOrdersPerTx               = 1
	defaultOrderRefreshInterval         = 30 * time.Second
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
	defaultHomeDir := home + "/.order-client"

	viper.SetDefault("home_dir", defaultHomeDir)
	viper.SetDefault("log_level", defaultLogLevel)
	viper.SetDefault("whale.account_name", whaleAccountName)
	viper.SetDefault("whale.keyring_backend", testKeyringBackend)
	viper.SetDefault("whale.allowed_denoms", []string{defaultGasDenom})
	viper.SetDefault("whale.keyring_dir", defaultHomeDir)
	viper.SetDefault("bots.keyring_backend", testKeyringBackend)
	viper.SetDefault("bots.keyring_dir", defaultHomeDir)
	viper.SetDefault("bots.number_of_bots", defaultNumberOfBots)
	viper.SetDefault("bots.top_up_factor", defaultBotTopUpFactor)
	viper.SetDefault("node_address", defaultNodeAddress)
	viper.SetDefault("gas_prices", defaultGasPrices)
	viper.SetDefault("minimum_gas_balance", defaultMinimumGasBalance)
	viper.SetDefault("max_orders_per_tx", defaultMaxOrdersPerTx)
	viper.SetDefault("order_refresh_interval", defaultOrderRefreshInterval)
	viper.SetDefault("order_cleanup_interval", defaultOrderCleanupInterval)
	viper.SetDefault("dispute_period_refresh_interval", defaultDisputePeriodRefreshInterval)
	viper.SetDefault("slack.enabled", false)
	viper.SetDefault("slack.app_token", "<your-slack-app-token>")
	viper.SetDefault("slack.channel_id", "<your-slack-channel-id>")

	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		cfgFile = defaultHomeDir + "/config.yaml"
		viper.AddConfigPath(defaultHomeDir)
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}
}

type clientConfig struct {
	homeDir        string
	nodeAddress    string
	gasFees        string
	gasPrices      string
	keyringBackend cosmosaccount.KeyringBackend
}

func getCosmosClientOptions(config clientConfig) []cosmosclient.Option {
	options := []cosmosclient.Option{
		cosmosclient.WithAddressPrefix(hubAddressPrefix),
		cosmosclient.WithHome(config.homeDir),
		cosmosclient.WithBroadcastMode(flags.BroadcastBlock),
		cosmosclient.WithNodeAddress(config.nodeAddress),
		cosmosclient.WithFees(config.gasFees),
		cosmosclient.WithGasLimit(defaultGasLimit),
		cosmosclient.WithGasPrices(config.gasPrices),
		cosmosclient.WithKeyringBackend(config.keyringBackend),
		cosmosclient.WithKeyringDir(config.homeDir),
	}
	return options
}

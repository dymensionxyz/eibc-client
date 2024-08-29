package main

import (
	"log"
	"time"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type Config struct {
	HomeDir      string             `mapstructure:"home_dir"`
	NodeAddress  string             `mapstructure:"node_address"`
	DBPath       string             `mapstructure:"db_path"`
	Gas          GasConfig          `mapstructure:"gas"`
	OrderPolling OrderPollingConfig `mapstructure:"order_polling"`

	Whale whaleConfig `mapstructure:"whale"`
	Bots  botConfig   `mapstructure:"bots"`

	LogLevel    string      `mapstructure:"log_level"`
	SlackConfig slackConfig `mapstructure:"slack"`
	SkipRefund  bool        `mapstructure:"skip_refund"`
}

type OrderPollingConfig struct {
	IndexerURL string        `mapstructure:"indexer_url"`
	Interval   time.Duration `mapstructure:"interval"`
	Enabled    bool          `mapstructure:"enabled"`
}

type GasConfig struct {
	Prices            string `mapstructure:"prices"`
	Fees              string `mapstructure:"fees"`
	MinimumGasBalance string `mapstructure:"minimum_gas_balance"`
}

type botConfig struct {
	NumberOfBots   int                          `mapstructure:"number_of_bots"`
	KeyringBackend cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir     string                       `mapstructure:"keyring_dir"`
	TopUpFactor    int                          `mapstructure:"top_up_factor"`
	MaxOrdersPerTx int                          `mapstructure:"max_orders_per_tx"`
}

type whaleConfig struct {
	AccountName              string                       `mapstructure:"account_name"`
	KeyringBackend           cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir               string                       `mapstructure:"keyring_dir"`
	AllowedBalanceThresholds map[string]string            `mapstructure:"allowed_balance_thresholds"`
}

type slackConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	AppToken  string `mapstructure:"app_token"`
	ChannelID string `mapstructure:"channel_id"`
}

const (
	defaultNodeAddress       = "http://localhost:36657"
	defaultDBPath            = "mongodb://localhost:27017"
	hubAddressPrefix         = "dym"
	pubKeyPrefix             = "pub"
	defaultLogLevel          = "info"
	defaultHubDenom          = "adym"
	defaultGasFees           = "3000000000000000" + defaultHubDenom
	defaultMinimumGasBalance = "1000000000000000000" + defaultHubDenom
	testKeyringBackend       = "test"

	botNamePrefix               = "bot-"
	defaultWhaleAccountName     = "client"
	defaultBotTopUpFactor       = 5
	defaultNumberOfBots         = 30
	newOrderBufferSize          = 100
	defaultMaxOrdersPerTx       = 10
	defaultOrderRefreshInterval = 30 * time.Second
)

var defaultBalanceThresholds = map[string]string{defaultHubDenom: "1000000000000"}

func initConfig() {
	// Set default values
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	defaultHomeDir := home + "/.eibc-client"

	viper.SetDefault("log_level", defaultLogLevel)
	viper.SetDefault("home_dir", defaultHomeDir)
	viper.SetDefault("node_address", defaultNodeAddress)
	viper.SetDefault("db_path", defaultDBPath)

	viper.SetDefault("order_polling.interval", defaultOrderRefreshInterval)
	viper.SetDefault("order_polling.enabled", false)

	viper.SetDefault("gas.fees", defaultGasFees)
	viper.SetDefault("gas.minimum_gas_balance", defaultMinimumGasBalance)

	viper.SetDefault("whale.account_name", defaultWhaleAccountName)
	viper.SetDefault("whale.keyring_backend", testKeyringBackend)
	viper.SetDefault("whale.allowed_balance_thresholds", defaultBalanceThresholds)
	viper.SetDefault("whale.keyring_dir", defaultHomeDir)

	viper.SetDefault("bots.keyring_backend", testKeyringBackend)
	viper.SetDefault("bots.keyring_dir", defaultHomeDir)
	viper.SetDefault("bots.number_of_bots", defaultNumberOfBots)
	viper.SetDefault("bots.top_up_factor", defaultBotTopUpFactor)
	viper.SetDefault("bots.max_orders_per_tx", defaultMaxOrdersPerTx)

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
		cosmosclient.WithNodeAddress(config.nodeAddress),
		cosmosclient.WithFees(config.gasFees),
		cosmosclient.WithGasPrices(config.gasPrices),
		cosmosclient.WithKeyringBackend(config.keyringBackend),
		cosmosclient.WithKeyringDir(config.homeDir),
	}
	return options
}

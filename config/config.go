package config

import (
	"log"
	"time"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type Config struct {
	ServerAddress string             `mapstructure:"server_address"`
	HomeDir       string             `mapstructure:"home_dir"`
	NodeAddress   string             `mapstructure:"node_address"`
	DBPath        string             `mapstructure:"db_path"`
	Gas           GasConfig          `mapstructure:"gas"`
	OrderPolling  OrderPollingConfig `mapstructure:"order_polling"`

	Whale           WhaleConfig     `mapstructure:"whale"`
	Bots            BotConfig       `mapstructure:"bots"`
	FulfillCriteria FulfillCriteria `mapstructure:"fulfill_criteria"`

	LogLevel    string      `mapstructure:"log_level"`
	SlackConfig SlackConfig `mapstructure:"slack"`
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

type BotConfig struct {
	NumberOfBots   int                          `mapstructure:"number_of_bots"`
	KeyringBackend cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir     string                       `mapstructure:"keyring_dir"`
	TopUpFactor    int                          `mapstructure:"top_up_factor"`
	MaxOrdersPerTx int                          `mapstructure:"max_orders_per_tx"`
}

type WhaleConfig struct {
	AccountName              string                       `mapstructure:"account_name"`
	KeyringBackend           cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir               string                       `mapstructure:"keyring_dir"`
	AllowedBalanceThresholds map[string]string            `mapstructure:"allowed_balance_thresholds"`
}

type FulfillCriteria struct {
	MinFeePercentage MinFeePercentage `mapstructure:"min_fee_percentage"`
	FulfillmentMode  FulfillmentMode  `mapstructure:"fulfillment_mode"`
}

type FulfillmentMode struct {
	Level              FulfillmentLevel `mapstructure:"level"`
	FullNodes          []string         `mapstructure:"full_nodes"`
	MinConfirmations   int              `mapstructure:"min_confirmations"`
	ValidationWaitTime time.Duration    `mapstructure:"validation_wait_time"`
}

type MinFeePercentage struct {
	Chain map[string]float32 `mapstructure:"chain"`
	Asset map[string]float32 `mapstructure:"asset"`
}

type SlackConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	BotToken  string `mapstructure:"bot_token"`
	AppToken  string `mapstructure:"app_token"`
	ChannelID string `mapstructure:"channel_id"`
}

const (
	defaultNodeAddress       = "http://localhost:36657"
	defaultDBPath            = "mongodb://localhost:27017"
	HubAddressPrefix         = "dym"
	PubKeyPrefix             = "pub"
	defaultLogLevel          = "info"
	defaultHubDenom          = "adym"
	defaultGasFees           = "3000000000000000" + defaultHubDenom
	defaultMinimumGasBalance = "1000000000000000000" + defaultHubDenom
	testKeyringBackend       = "test"

	BotNamePrefix               = "bot-"
	defaultWhaleAccountName     = "client"
	defaultBotTopUpFactor       = 5
	defaultNumberOfBots         = 30
	NewOrderBufferSize          = 100
	defaultMaxOrdersPerTx       = 10
	defaultOrderRefreshInterval = 30 * time.Second
)

type FulfillmentLevel string

const (
	FulfillmentModeSequencer  FulfillmentLevel = "sequencer"
	FulfillmentModeP2P                         = "p2p"
	FulfillmentModeSettlement                  = "settlement"
)

func (f FulfillmentLevel) Validate() bool {
	return f == FulfillmentModeSequencer || f == FulfillmentModeP2P || f == FulfillmentModeSettlement
}

var defaultBalanceThresholds = map[string]string{defaultHubDenom: "1000000000000"}

func InitConfig() {
	// Set default values
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	defaultHomeDir := home + "/.eibc-client"

	viper.SetDefault("server_address", ":8000")
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
	if CfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(CfgFile)
	} else {
		CfgFile = defaultHomeDir + "/config.yaml"
		viper.AddConfigPath(defaultHomeDir)
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}
}

var CfgFile string

type ClientConfig struct {
	HomeDir        string
	NodeAddress    string
	GasFees        string
	GasPrices      string
	KeyringBackend cosmosaccount.KeyringBackend
}

func GetCosmosClientOptions(config ClientConfig) []cosmosclient.Option {
	options := []cosmosclient.Option{
		cosmosclient.WithAddressPrefix(HubAddressPrefix),
		cosmosclient.WithHome(config.HomeDir),
		cosmosclient.WithNodeAddress(config.NodeAddress),
		cosmosclient.WithFees(config.GasFees),
		cosmosclient.WithGas(cosmosclient.GasAuto),
		cosmosclient.WithGasPrices(config.GasPrices),
		cosmosclient.WithKeyringBackend(config.KeyringBackend),
		cosmosclient.WithKeyringDir(config.HomeDir),
	}
	return options
}

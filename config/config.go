package config

import (
	"log"
	"time"

	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type Config struct {
	NodeAddress  string             `mapstructure:"node_address"`
	Gas          GasConfig          `mapstructure:"gas"`
	OrderPolling OrderPollingConfig `mapstructure:"order_polling"`

	Operator   OperatorConfig   `mapstructure:"operator"`
	Bots       BotConfig        `mapstructure:"bots"`
	Validation ValidationConfig `mapstructure:"validation"`

	LogLevel string `mapstructure:"log_level"`
}

type OrderPollingConfig struct {
	IndexerURL string        `mapstructure:"indexer_url"`
	Interval   time.Duration `mapstructure:"interval"`
	Enabled    bool          `mapstructure:"enabled"`
}

type GasConfig struct {
	Prices string `mapstructure:"prices"`
	Fees   string `mapstructure:"fees"`
}

type BotConfig struct {
	NumberOfBots    int                          `mapstructure:"number_of_bots"`
	OperatorAddress string                       `mapstructure:"operator_address"`
	PolicyAddress   string                       `mapstructure:"policy_address"`
	KeyringBackend  cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir      string                       `mapstructure:"keyring_dir"`
	MaxOrdersPerTx  int                          `mapstructure:"max_orders_per_tx"`
}

type OperatorConfig struct {
	AccountName    string                       `mapstructure:"account_name"`
	KeyringBackend cosmosaccount.KeyringBackend `mapstructure:"keyring_backend"`
	KeyringDir     string                       `mapstructure:"keyring_dir"`
	GroupID        int                          `mapstructure:"group_id"`
	MinFeeShare    string                       `mapstructure:"min_fee_share"`
}

type ValidationConfig struct {
	FallbackLevel      ValidationLevel `mapstructure:"fallback_level"`
	FullNodes          []string        `mapstructure:"full_nodes"`
	MinConfirmations   int             `mapstructure:"min_confirmations"`
	ValidationWaitTime time.Duration   `mapstructure:"validation_wait_time"`
}

const (
	defaultNodeAddress = "http://localhost:36657"
	defaultDBPath      = "mongodb://localhost:27017"
	HubAddressPrefix   = "dym"
	PubKeyPrefix       = "pub"
	defaultLogLevel    = "info"
	defaultHubDenom    = "adym"
	defaultGasFees     = "3000000000000000" + defaultHubDenom
	testKeyringBackend = "test"

	BotNamePrefix               = "bot-"
	defaultOperatorAccountName  = "client"
	defaultNumberOfBots         = 30
	NewOrderBufferSize          = 100
	defaultMaxOrdersPerTx       = 10
	defaultOrderRefreshInterval = 30 * time.Second
)

type ValidationLevel string

const (
	ValidationModeSequencer  ValidationLevel = "sequencer"
	ValidationModeP2P                        = "p2p"
	ValidationModeSettlement                 = "settlement"
)

func (f ValidationLevel) Validate() bool {
	return f == ValidationModeSequencer || f == ValidationModeP2P || f == ValidationModeSettlement
}

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
	viper.SetDefault("node_address", defaultNodeAddress)
	viper.SetDefault("db_path", defaultDBPath)

	viper.SetDefault("order_polling.interval", defaultOrderRefreshInterval)
	viper.SetDefault("order_polling.enabled", false)

	viper.SetDefault("gas.fees", defaultGasFees)

	viper.SetDefault("operator.account_name", defaultOperatorAccountName)
	viper.SetDefault("operator.keyring_backend", testKeyringBackend)
	viper.SetDefault("operator.keyring_dir", defaultHomeDir)

	viper.SetDefault("bots.keyring_backend", testKeyringBackend)
	viper.SetDefault("bots.keyring_dir", defaultHomeDir)
	viper.SetDefault("bots.number_of_bots", defaultNumberOfBots)
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
	FeeGranter     string
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
		cosmosclient.WithFeeGranter(config.FeeGranter),
	}
	return options
}

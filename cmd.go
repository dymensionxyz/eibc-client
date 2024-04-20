package main

import (
	"log"

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "order-client",
	Short: "Order client for Dymension Hub",
	Long:  `A client or bot for scanning and fulfilling demand orders found on the Dymension Hub chain.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := Config{}
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		oc, err := newOrderClient(config)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		if err := oc.start(); err != nil {
			log.Fatalf("failed to start order client: %v", err)
		}
	},
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	// Set default values
	viper.SetDefault("keyring_backend", testKeyringBackend)
	viper.SetDefault("node_address", nodeAddress)
	viper.SetDefault("chain_id", chainID)
	viper.SetDefault("gas_prices", defaultGasPrices)
	viper.SetDefault("account_name", "hub_"+uuid.Must(uuid.NewRandom()).String()[:4])
	viper.SetDefault("slack.enabled", true)
	viper.SetDefault("slack.channel_id", "poor-bots")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.Fatalf("failed to get home directory: %v", err)
		}

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

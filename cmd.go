package main

import (
	"log"

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

		oc, err := newOrderClient(cmd.Context(), config)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		if err := oc.start(cmd.Context()); err != nil {
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

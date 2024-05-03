package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "order-client",
	Short: "eIBC Order client for Dymension Hub",
	Long:  `Order client for Dymension Hub that scans for demand orders and fulfills them.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no arguments are provided, print usage information
		if len(args) == 0 {
			if err := cmd.Usage(); err != nil {
				log.Fatalf("Error printing usage: %v", err)
			}
		}
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the order client",
	Long:  `Initialize the order client by generating a config file with default values.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := Config{}
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		// if home dir doesn't exist, create it
		if _, err := os.Stat(config.HomeDir); os.IsNotExist(err) {
			if err := os.MkdirAll(config.HomeDir, 0755); err != nil {
				log.Fatalf("failed to create home directory: %v", err)
			}
		}

		if err := viper.WriteConfigAs(cfgFile); err != nil {
			log.Fatalf("failed to write config file: %v", err)
		}

		fmt.Printf("Config file created: %s\n", cfgFile)
		fmt.Println()
		fmt.Println("Edit the config file to set the correct values for your environment.")
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the order client",
	Long:  `Start the order client that scans for demand orders and fulfills them.`,
	Run: func(cmd *cobra.Command, args []string) {
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		config := Config{}
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		log.Printf("using config file: %+v", viper.ConfigFileUsed())

		oc, err := newOrderClient(cmd.Context(), config)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		if config.Bots.NumberOfBots == 0 {
			log.Println("no bots to start")
			return
		}

		if err := oc.start(cmd.Context()); err != nil {
			log.Fatalf("failed to start order client: %v", err)
		}
	},
}

var cfgFile string

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

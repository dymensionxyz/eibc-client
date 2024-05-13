package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

var balancesCmd = &cobra.Command{
	Use:   "balances",
	Short: "Get account balances",
	Long:  `Get account balances for the configured whale account and the bot accounts.`,
	Run: func(cmd *cobra.Command, args []string) {
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		config := Config{}
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		config.skipRefund = true

		if all, _ := cmd.Flags().GetBool("all"); all {
			accs, err := getBotAccounts("dymd", config.Bots.KeyringDir)
			if err != nil {
				log.Fatalf("failed to get bot accounts: %v", err)
			}
			config.Bots.NumberOfBots = len(accs)
		}

		oc, err := newOrderClient(cmd.Context(), config)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		if err := oc.whale.accountSvc.refreshBalances(cmd.Context()); err != nil {
			log.Fatalf("failed to refresh whale account balances: %v", err)
		}

		totalBalances := sdk.NewCoins(oc.whale.accountSvc.balances...)

		longestAmountStr := 0

		for _, bal := range oc.whale.accountSvc.balances {
			amtStr := formatAmount(bal.Amount.String())
			if len(amtStr) > longestAmountStr {
				longestAmountStr = len(amtStr)
			}
		}

		fmt.Println()
		fmt.Println("Bots Balances:")

		for _, b := range oc.bots {
			if err := b.accountSvc.refreshBalances(cmd.Context()); err != nil {
				log.Fatalf("failed to refresh bot account balances: %v", err)
			}

			totalBalances = totalBalances.Add(b.accountSvc.balances...)

			for _, bal := range b.accountSvc.balances {
				amtStr := formatAmount(bal.Amount.String())
				if len(amtStr) > longestAmountStr {
					longestAmountStr = len(amtStr)
				}
			}
		}

		maxDen := 68

		dividerItem := ""
		for i := 0; i < longestAmountStr+maxDen+3; i++ {
			dividerItem += "="
		}

		i := 0
		for name, b := range oc.bots {
			i++
			accPref := fmt.Sprintf("%d. | '%s': ", i, name)
			printAccountBalances(b.accountSvc.balances, b.accountSvc.account.GetAddress().String(), longestAmountStr, maxDen, accPref, dividerItem)
		}

		fmt.Println()
		fmt.Println("Whale Balances:")

		accPref := fmt.Sprintf("Whale | '%s': ", oc.whale.accountSvc.accountName)
		printAccountBalances(oc.whale.accountSvc.balances, oc.whale.accountSvc.account.GetAddress().String(), longestAmountStr, maxDen, accPref, dividerItem)

		fmt.Println()
		fmt.Println("Total Balances:")
		fmt.Println(dividerItem)

		printBalances(totalBalances, longestAmountStr, maxDen)
	},
}

func printAccountBalances(balances sdk.Coins, address string, maxBal, maxDen int, accPref, dividerItem string) {
	if balances.IsZero() {
		return
	}

	dividerAcc := ""

	fmt.Printf("%s", dividerItem)

	accLine := fmt.Sprintf("\n %s%s |", accPref, address)
	for i := 0; i < len(accLine)-1; i++ {
		dividerAcc += "-"
	}
	fmt.Printf("%s\n", accLine)
	fmt.Printf("%s\n", dividerAcc)

	printBalances(balances, maxBal, maxDen)

	fmt.Println()
}

func printBalances(balances sdk.Coins, maxBal, maxDen int) {
	dividerBal, dividerDen := "", ""

	for i := 0; i < maxBal; i++ {
		dividerBal += "-"
	}

	for i := 0; i < maxDen; i++ {
		dividerDen += "-"
	}

	fmt.Printf("%*s | Denom\n", maxBal, "Amount")
	fmt.Printf("%*s | %s\n", maxBal, dividerBal, dividerDen)

	for _, bl := range balances {
		amtStr := formatAmount(bl.Amount.String())
		fmt.Printf("%*s | %-s\n", maxBal, amtStr, bl.Denom)
	}
}

func formatAmount(numStr string) string {
	if len(numStr) <= 18 {
		return "0," + strings.Repeat("0", 18-len(numStr)) + numStr
	}
	return numStr[:len(numStr)-18] + "," + numStr[len(numStr)-18:]
}

var cfgFile string

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)

	balancesCmd.Flags().BoolP("all", "a", false, "Filter by fulfillment status")
	rootCmd.AddCommand(balancesCmd)

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

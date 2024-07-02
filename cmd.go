package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dymensionxyz/eibc-client/cmd/version"
	"github.com/dymensionxyz/eibc-client/store"
)

var rootCmd = &cobra.Command{
	Use:   "eibc-client",
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
	Use:   "funds",
	Short: "Get account funds",
	Long:  `Get account balances and pending rewards for the configured whale account and the bot accounts.`,
	Run: func(cmd *cobra.Command, args []string) {
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		config := Config{}
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		config.SkipRefund = true

		oc, err := newOrderClient(cmd.Context(), config)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		defer oc.orderTracker.store.Close()

		if err := oc.whale.accountSvc.refreshBalances(cmd.Context()); err != nil {
			log.Fatalf("failed to refresh whale account balances: %v", err)
		}

		longestAmountStr := 0

		for _, bal := range oc.whale.accountSvc.balances {
			amtStr := formatAmount(bal.Amount.String())
			if len(amtStr) > longestAmountStr {
				longestAmountStr = len(amtStr)
			}
		}

		fmt.Println()
		fmt.Println("Bots Funds:")

		bots, err := oc.orderTracker.store.GetBots(cmd.Context(), store.OnlyWithFunds())
		if err != nil {
			log.Fatalf("failed to get bots from db: %v", err)
		}

		for _, b := range bots {
			balances, err := sdk.ParseCoinsNormalized(strings.Join(b.Balances, ","))
			if err != nil {
				log.Fatalf("failed to parse balance: %v", err)
			}

			pendingRewards, err := sdk.ParseCoinsNormalized(strings.Join(b.PendingRewards, ","))
			if err != nil {
				log.Fatalf("failed to parse pending rewards: %v", err)
			}

			for _, bal := range balances {
				if len(bal.Amount.String()) > longestAmountStr {
					longestAmountStr = len(bal.Amount.String())
				}
			}

			for _, pr := range pendingRewards {
				if len(pr.Amount.String()) > longestAmountStr {
					longestAmountStr = len(pr.Amount.String())
				}
			}
		}

		maxDen := 68

		dividerItem := ""
		dividerFunds := ""

		for i := 0; i < longestAmountStr+maxDen+3; i++ {
			dividerItem += "="
			dividerFunds += "-"
		}

		totalBalances := sdk.NewCoins(oc.whale.accountSvc.balances...)
		totalPendingRewards := sdk.NewCoins()

		i := 0
		for _, b := range bots {
			i++

			balances, err := sdk.ParseCoinsNormalized(strings.Join(b.Balances, ","))
			if err != nil {
				log.Fatalf("failed to parse balance: %v", err)
			}

			pendingRewards, err := sdk.ParseCoinsNormalized(strings.Join(b.PendingRewards, ","))
			if err != nil {
				log.Fatalf("failed to parse pending rewards: %v", err)
			}

			totalBalances = totalBalances.Add(balances...)
			totalPendingRewards = totalPendingRewards.Add(pendingRewards...)

			if !balances.IsZero() {
				accPref := fmt.Sprintf("%d. | '%s': ", i, b.Name)
				printAccountSlot(b.Address, accPref, dividerItem)
				fmt.Println("Balances:")
				printBalances(balances, longestAmountStr, maxDen)
				fmt.Println()
			}

			if !pendingRewards.IsZero() {
				fmt.Println("Pending Rewards:")
				fmt.Println(dividerFunds)
				printBalances(pendingRewards, longestAmountStr, maxDen)
			}
		}

		fmt.Println()
		fmt.Println("Whale Balances:")

		if !oc.whale.accountSvc.balances.IsZero() {
			accPref := fmt.Sprintf("Whale | '%s': ", oc.whale.accountSvc.accountName)
			printAccountSlot(
				oc.whale.accountSvc.account.GetAddress().String(),
				accPref,
				dividerItem,
			)
			printBalances(oc.whale.accountSvc.balances, longestAmountStr, maxDen)
			fmt.Println()
		}

		fmt.Println()
		fmt.Println("Total:")
		fmt.Println(dividerItem)
		fmt.Println("Balances:")
		printBalances(totalBalances, longestAmountStr, maxDen)

		fmt.Println(dividerFunds)
		fmt.Println("Pending Rewards:")
		printBalances(totalPendingRewards, longestAmountStr, maxDen)
	},
}

func printAccountSlot(address string, accPref, dividerItem string) {
	dividerAcc := ""

	fmt.Printf("%s", dividerItem)

	accLine := fmt.Sprintf("\n %s%s |", accPref, address)
	for i := 0; i < len(accLine)-1; i++ {
		dividerAcc += "-"
	}
	fmt.Printf("%s\n", accLine)
	fmt.Printf("%s\n", dividerAcc)
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
		amtStr := bl.Amount.String()

		if bl.Denom == "adym" {
			amtStr = formatAmount(bl.Amount.String())
			bl.Denom = "dym"
		}
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

	rootCmd.AddCommand(version.Cmd())

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/eibc"
	utils "github.com/dymensionxyz/eibc-client/utils/viper"
	"github.com/dymensionxyz/eibc-client/version"
)

var RootCmd = &cobra.Command{
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
		cfg := config.Config{}
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		// if bot key dir doesn't exist, create it
		if _, err := os.Stat(cfg.Bots.KeyringDir); os.IsNotExist(err) {
			if err := os.MkdirAll(cfg.Bots.KeyringDir, 0o755); err != nil {
				log.Fatalf("failed to create bot key directory: %v", err)
			}
		}

		if err := viper.WriteConfigAs(config.CfgFile); err != nil {
			log.Fatalf("failed to write config file: %v", err)
		}

		fmt.Printf("Config file created: %s\n", config.CfgFile)
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

		cfg := config.Config{}
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Fatalf("failed to unmarshal config: %v", err)
		}

		if !cfg.Validation.FallbackLevel.Validate() {
			log.Fatalf("invalid fallback validation level: %s", cfg.Validation.FallbackLevel)
		}

		log.Printf("using config file: %+v", viper.ConfigFileUsed())

		logger, err := buildLogger(cfg.LogLevel)
		if err != nil {
			log.Fatalf("failed to build logger: %v", err)
		}

		// Ensure all logs are written
		defer logger.Sync() // nolint: errcheck

		oc, err := eibc.NewOrderClient(cfg, logger)
		if err != nil {
			log.Fatalf("failed to create order client: %v", err)
		}

		if cfg.Bots.NumberOfBots == 0 {
			log.Println("no bots to start")
			return
		}

		if err := oc.Start(cmd.Context()); err != nil {
			log.Fatalf("failed to start order client: %v", err)
		}
	},
}

func buildLogger(logLevel string) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.Set(logLevel); err != nil {
		return nil, fmt.Errorf("failed to set log level: %w", err)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.Lock(os.Stdout),
		level,
	))

	return logger, nil
}

var scaleCmd = &cobra.Command{
	Use:   "scale [count]",
	Short: "scale bot count",
	Long:  `scale the number of bot accounts that fulfill the eibc orders`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		newBotCount, err := strconv.Atoi(args[0])
		if err != nil {
			return
		}

		home, err := homedir.Dir()
		if err != nil {
			log.Fatalf("failed to get home directory: %v", err)
		}

		defaultHomeDir := home + "/.eibc-client"
		config.CfgFile = defaultHomeDir + "/config.yaml"

		viper.SetConfigFile(config.CfgFile)
		err = viper.ReadInConfig()
		if err != nil {
			return
		}

		err = utils.UpdateViperConfig("bots.number_of_bots", newBotCount, viper.ConfigFileUsed())
		if err != nil {
			return
		}

		fmt.Printf(
			"bot count successfully scaled to %d, please restart the eibc process if it's running\n",
			newBotCount,
		)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of roller",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.BuildVersion)
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

func init() {
	RootCmd.CompletionOptions.DisableDefaultCmd = true
	RootCmd.AddCommand(initCmd)
	RootCmd.AddCommand(startCmd)
	RootCmd.AddCommand(scaleCmd)

	RootCmd.AddCommand(versionCmd)

	cobra.OnInitialize(config.InitConfig)

	RootCmd.PersistentFlags().StringVar(&config.CfgFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

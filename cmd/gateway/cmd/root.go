package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var LogDir = "./cmd/gateway/data/logs"
var DataDir = "./cmd/gateway/data"

// rootCmd is the parent of all subcommands.
// It has no RunE — running ashrix-gateway alone prints help.
var rootCmd = &cobra.Command{
	Use:   "ashrix-gateway",
	Short: "Ashrix self-hosted application access gateway",
	Long:  `Ashrix accepts application requests and routes them through outbound QUIC connectors. Configure gateway.yaml and provision the gateway identity files before starting.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("data-dir", "./data", "Gateway identity and state directory")
	rootCmd.PersistentFlags().String("log-dir", "./logs", "Gateway log directory")
	_ = viper.BindPFlag("data_dir", rootCmd.PersistentFlags().Lookup("data-dir"))
	_ = viper.BindPFlag("log_dir", rootCmd.PersistentFlags().Lookup("log-dir"))

}

func initConfig() {

	viper.AddConfigPath("/etc/ashrix")
	viper.AddConfigPath("$HOME/.ashrix/gateway")
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	_, configNotFound := err.(viper.ConfigFileNotFoundError)
	if configNotFound {
		viper.SetConfigName("gateway")
		err = viper.ReadInConfig()
	} else if err == nil {
		viper.SetConfigName("gateway")
		if mergeErr := viper.MergeInConfig(); mergeErr != nil {
			if _, ok := mergeErr.(viper.ConfigFileNotFoundError); !ok {
				err = mergeErr
			}
		}
	}
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file — not fatal.
		} else {
			// Config file exists but is malformed or unreadable — fatal.
			// Examples: bad YAML syntax, permission denied.
			fmt.Fprintln(os.Stderr, "fatal: config error:", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "config loaded:", viper.ConfigFileUsed())
	}
}

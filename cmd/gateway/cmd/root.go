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
	Short: "Ashrix Gateway — secure application access without VPNs",
	Long: `Ashrix Gateway connects your internal applications to the
Ashrix Control Plane, enabling secure access via your existing
identity provider — no VPN required.

Run 'ashrix-gateway register' first, then 'ashrix-gateway start'.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// cobra.OnInitialize runs initConfig before ANY subcommand's RunE.
	// This guarantees Viper is loaded before start, stop, status, or register fire.
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("redis-url", "redis://localhost:6379/1", "Redis Connection URL")
	rootCmd.PersistentFlags().String("redis-password", "", "Redis Password")

	viper.BindPFlag("data_dir", registerCmd.Flags().Lookup("data-dir"))
	viper.BindPFlag("log_dir", registerCmd.Flags().Lookup("log-dir"))

	viper.BindPFlag("redis_url", rootCmd.PersistentFlags().Lookup("redis-url"))
	viper.BindPFlag("redis_password", rootCmd.PersistentFlags().Lookup("redis-password"))

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

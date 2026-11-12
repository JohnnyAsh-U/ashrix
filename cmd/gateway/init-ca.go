package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway2/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(initCACmd)
}

var initCACmd = &cobra.Command{
	Use:   "config",
	Short: "Show Ashrix Gateway config",
	RunE:  initCA,
}

func initCA(cmd *cobra.Command, args []string) error {
	var appConfig config.Config
	err := viper.Unmarshal(&appConfig)
	if err != nil {
		fmt.Println("couldn't load config")
	}

	
	// Generate

	return nil
}

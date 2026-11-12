package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	gateway "github.com/JohnnyAsh-U/ashrix-api/internal/gateway"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Gateway",
	Long:  `Start the self-hosted Ashrix application gateway using local connector and certificate configuration.`,
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load Config from viper and Validate
	var appConfig config.Config
	err := viper.Unmarshal(&appConfig)
	if err != nil {
		return fmt.Errorf("couldn't load config: %w", err)
	}

	if err := appConfig.Validate(); err != nil {
		return fmt.Errorf("Validation Error: %w", err)
	}

	// Initialize Logger
	if err := logger.Init(logger.Config{
		Env:        "dev",
		LogDir:     appConfig.Logging.Dir,
		Level:      "info",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
		MaxBackups: 10,
		Compress:   true,
	}); err != nil {
		return fmt.Errorf("Cannot Initiate Logger Error: %w", err)
	}

	log := logger.App

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Run Gateway
	if err := gateway.RunGateway(ctx, &appConfig, log); err != nil {
		log.Error("Gateway Error", slog.Any("err", err))
		os.Exit(1)
	}
	cancel()
	return nil
}

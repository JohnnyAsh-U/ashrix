package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Ashrix Connector",
	Long:  `Start the Ashrix Gateway`,
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) (err error) {

	var appConfig config.Config
	err = viper.Unmarshal(&appConfig)
	if err != nil {
		return fmt.Errorf("couldn't load config: %w", err)
	}

	if err := appConfig.Validate(); err != nil {
		return fmt.Errorf("Validation Error: %w", err)
	}

	//------------------------Logger Initializer---------------------------
	fmt.Println("Initializing Logger...")

	if err := logger.LoggerInit(logger.LoggerConfig{
		Env:        "prod",
		LogDir:     appConfig.Logging.Dir,
		Level:      "info",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
		MaxBackups: 10,
		Compress:   true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot init logging %v\n", err)
		os.Exit(1)
	}

	log := logger.LoggerApp

	ctx, cancel := context.WithCancel(context.Background())

	if err := connector.RunConnector(ctx, &appConfig, log); err != nil {
		log.Error("Connector ERror", slog.Any("err", err))
		os.Exit(1)
	}

	cancel()
	return nil

}

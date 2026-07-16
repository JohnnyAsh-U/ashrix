package cmd

import (
	"log"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/service"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
    Use:   "uninstall",
    Short: "Uninstall the system service",
    Long:  `Remove the Ashrix connector from system services.`,
    Example: `  # Uninstall service
  ashrix-connector uninstall`,
    Run: func(cmd *cobra.Command, args []string) {
        log.Println("🗑️ Uninstalling service...")

        if err := service.Uninstall(); err != nil {
            log.Fatalf("❌ Failed to uninstall service: %v", err)
        }

        log.Println("✅ Service uninstalled successfully!")
    },
}

func init() {
    rootCmd.AddCommand(uninstallCmd)
}
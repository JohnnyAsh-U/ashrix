package cmd

import (
	"log"
	"os"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
            log.Printf("⚠️ Failed to uninstall service (it might not be installed): %v", err)
        }

        baseDir := viper.GetString("basedir")
        if baseDir != "" {
            log.Printf("🗑️ Deleting all configuration, logs, and certificates at %s...", baseDir)
            if err := os.RemoveAll(baseDir); err != nil {
                log.Printf("❌ Failed to delete directory %s: %v", baseDir, err)
            }
        }

        log.Println("✅ Ashrix Connector uninstalled successfully!")
    },
}

func init() {
    rootCmd.AddCommand(uninstallCmd)
}
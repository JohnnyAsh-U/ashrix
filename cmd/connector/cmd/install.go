package cmd

import (
	"log"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var installCmd = &cobra.Command{
    Use:   "install",
    Short: "Install as a system service",
    Long: `Install the Ashrix connector as a system service.
On Windows: Installs as a Windows Service
On Linux: Installs as a systemd service`,
    Example: `  # Install service
  ashrix-connector install`,
    Run: func(cmd *cobra.Command, args []string) {
        log.Println("📦 Installing service...")

        if err := service.Install(); err != nil {
            log.Fatalf("❌ Failed to install service: %v", err)
        }

        log.Println("✅ Service installed successfully!")
        log.Println("💡 Register with: ashrix-connector start --token <your-token>")
    },
}

func init() {
    rootCmd.AddCommand(installCmd)

    installCmd.Flags().String("name", "ashrix-connector", "Service name")
    installCmd.Flags().String("user", "", "User to run service as (Linux only)")

    viper.BindPFlag("service.name", installCmd.Flags().Lookup("name"))
    viper.BindPFlag("service.user", installCmd.Flags().Lookup("user"))
}
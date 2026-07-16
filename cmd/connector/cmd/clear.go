package cmd

import (
    "log"

    "github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
    Use:   "clear-credential",
    Short: "Clear stored credentials",
    Long:  `Remove all stored credentials and certificates from disk.`,
    Example: `  # Clear credentials
  ashrix-connector clear-credential`,
    Run: func(cmd *cobra.Command, args []string) {
        log.Println("🗑️ Clearing credentials...")

        if err := Storage.ClearCredential(); err != nil {
            log.Fatalf("❌ Failed to clear credentials: %v", err)
        }

        log.Println("✅ Credentials cleared successfully!")
    },
}

func init() {
    rootCmd.AddCommand(clearCmd)
}
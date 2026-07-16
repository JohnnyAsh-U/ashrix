package cmd

import (
	"log"

	// "ashrix-connector/internal/connector"

	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:    "service",
	Short:  "Run as a service/daemon (internal use)",
	Long:   `This command is used internally by the service manager.`,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Println("📟 Running as service")
		if err := runStart(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}

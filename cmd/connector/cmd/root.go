package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

// rootCmd is the parent of all subcommands.
// It has no RunE — running ashrix-connector alone prints help.
var rootCmd = &cobra.Command{
	Use:   "ashrix-connector",
	Short: "Ashrix Connector — secure application access without VPNs",
	Long: `Ashrix Connector connects your internal applications to the
Ashrix Gateway, enabling secure access via your existing
identity provider — no VPN required.

Run 'ashrix-connector start' to connect your internal application`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display gateway version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(),
			"ashrix-gateway %s\n", version)
	},
}

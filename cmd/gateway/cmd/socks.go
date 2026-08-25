package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "Manage SOCKS5 credentials for M2M",
}

var socksAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a SOCKS5 credential for a connector",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFromViper()
		if err != nil {
			return err
		}
		s, err := store.OpenBoltStore(cfg.DataDir)
		if err != nil {
			return err
		}
		defer s.Close()

		connectorID, _ := cmd.Flags().GetString("connector-id")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		if connectorID == "" || username == "" || password == "" {
			return fmt.Errorf("connector-id, username, and password are required")
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}

		cred := &store.SOCKS5Credential{
			ConnectorID:  connectorID,
			Username:     username,
			PasswordHash: string(hash),
			CreatedAt:    time.Now(),
		}

		if err := s.SetSOCKS5Credential(context.Background(), cred); err != nil {
			return err
		}

		fmt.Printf("SOCKS5 credential added for connector %s\n", connectorID)
		return nil
	},
}

var socksDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete SOCKS5 credential for a connector",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFromViper()
		if err != nil {
			return err
		}
		s, err := store.OpenBoltStore(cfg.DataDir)
		if err != nil {
			return err
		}
		defer s.Close()

		connectorID, _ := cmd.Flags().GetString("connector-id")
		if connectorID == "" {
			return fmt.Errorf("connector-id is required")
		}

		if err := s.DeleteSOCKS5Credential(context.Background(), connectorID); err != nil {
			return err
		}

		fmt.Printf("SOCKS5 credential deleted for connector %s\n", connectorID)
		return nil
	},
}

var socksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SOCKS5 credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFromViper()
		if err != nil {
			return err
		}
		s, err := store.OpenBoltStore(cfg.DataDir)
		if err != nil {
			return err
		}
		defer s.Close()

		creds, err := s.ListSOCKS5Credentials(context.Background())
		if err != nil {
			return err
		}

		fmt.Printf("%-36s %-20s %s\n", "CONNECTOR ID", "USERNAME", "CREATED AT")
		for _, c := range creds {
			fmt.Printf("%-36s %-20s %s\n", c.ConnectorID, c.Username, c.CreatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

func init() {
	socksAddCmd.Flags().String("connector-id", "", "Connector ID")
	socksAddCmd.Flags().String("username", "", "SOCKS5 username")
	socksAddCmd.Flags().String("password", "", "SOCKS5 password")

	socksDeleteCmd.Flags().String("connector-id", "", "Connector ID")

	socksCmd.AddCommand(socksAddCmd, socksDeleteCmd, socksListCmd)
	rootCmd.AddCommand(socksCmd)
}

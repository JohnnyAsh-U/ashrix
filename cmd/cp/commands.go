package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/database/store"
	pkica "github.com/JohnnyAsh-U/ashrix-api/internal/cp/pki_ca"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/bootstrap"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/crypto"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/pki"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "ashrix-cp", Short: "Ashrix Control Plane"}
	root.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start the Control Plane servers",
		RunE:  func(*cobra.Command, []string) error { return runServer() },
	})
	setup := &cobra.Command{Use: "setup", Short: "Provision Control Plane identity and initial tenant"}
	setup.AddCommand(pkiSetupCommand(), cpCertSetupCommand(), bundleKeySetupCommand(), firstAdminSetupCommand())
	root.AddCommand(setup)
	return root
}

func loadSetupConfig() (*config.Config, error) {
	_ = godotenv.Load()
	cfg, errs := config.LoadProvisioning()
	if len(errs) > 0 {
		return nil, fmt.Errorf("load setup config: %v", errs)
	}
	return cfg, nil
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func ashrixBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ashrix"), nil
}

func openSetupDatabase(ctx context.Context, cfg *config.Config) (*databaseHandle, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required for this setup command")
	}
	pool, err := database.Connect(ctx, cfg, setupLogger())
	if err != nil {
		return nil, err
	}
	queries := store.New(pool)
	return &databaseHandle{close: pool.Close, pool: pool, pkiRepo: pkica.NewPKICARepository(queries)}, nil
}

type databaseHandle struct {
	pool    *pgxpool.Pool
	close   func()
	pkiRepo pkica.Repository
}

func pkiSetupCommand() *cobra.Command {
	return &cobra.Command{Use: "pki", Short: "Provision root and intermediate CAs", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadSetupConfig()
		if err != nil {
			return err
		}
		if cfg.PKIConfig.PKIUnlockSecret == "" {
			return fmt.Errorf("PKI_UNLOCK_SECRET is required")
		}
		handle, err := openSetupDatabase(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer handle.close()
		baseDir, err := ashrixBaseDir()
		if err != nil {
			return err
		}
		_, err = pki.ProvisionSigner(baseDir, cfg.PKIConfig, handle.pkiRepo, setupLogger())
		if err != nil {
			return err
		}
		fmt.Println("Root and intermediate CA are provisioned and recorded in the database.")
		return nil
	}}
}

func cpCertSetupCommand() *cobra.Command {
	return &cobra.Command{Use: "cp-cert", Short: "Provision the Control Plane mTLS certificate", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadSetupConfig()
		if err != nil {
			return err
		}
		if cfg.PKIConfig.PKIUnlockSecret == "" {
			return fmt.Errorf("PKI_UNLOCK_SECRET is required")
		}
		handle, err := openSetupDatabase(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer handle.close()
		baseDir, err := ashrixBaseDir()
		if err != nil {
			return err
		}
		logger := setupLogger()
		signer, err := pki.ProvisionSigner(baseDir, cfg.PKIConfig, handle.pkiRepo, logger)
		if err != nil {
			return err
		}
		if _, err := crypto.ProvisionControlPlanePKI(baseDir, cfg.PKIConfig.PKIUnlockSecret, signer, handle.pkiRepo, logger); err != nil {
			return err
		}
		fmt.Println("Control Plane certificate is provisioned and recorded in the database.")
		return nil
	}}
}

func bundleKeySetupCommand() *cobra.Command {
	return &cobra.Command{Use: "bundle-key", Short: "Provision the bundle signing key and print the gateway public key", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadSetupConfig()
		if err != nil {
			return err
		}
		if cfg.PKIConfig.BundleSigningSecret == "" {
			return fmt.Errorf("BUNDLE_SIGNING_SECRET is required")
		}
		baseDir, err := ashrixBaseDir()
		if err != nil {
			return err
		}
		signer, err := crypto.ProvisionBundleSigningKeys(baseDir, cfg.PKIConfig.BundleSigningSecret, setupLogger())
		if err != nil {
			return err
		}
		publicPEM, err := signer.GetPublicKeyString()
		if err != nil {
			return err
		}
		publicKeyEnv, err := signer.GetPublicKeyBase64()
		if err != nil {
			return err
		}
		fmt.Println("Bundle-signing public key (safe to distribute):")
		fmt.Print(publicPEM)
		fmt.Println("Set this on every gateway before starting it:")
		fmt.Printf("ASHRIX_POLICY_PUBLIC_KEY=%s\n", publicKeyEnv)
		return nil
	}}
}

func firstAdminSetupCommand() *cobra.Command {
	var orgName, orgSlug,customDomain, email string
	command := &cobra.Command{Use: "bootstrap", Short: "Create the first organization and admin", RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadSetupConfig()
		if err != nil {
			return err
		}
		password, err := readPassword("Initial admin password: ")
		if err != nil {
			return err
		}
		confirmation, err := readPassword("Confirm password: ")
		if err != nil {
			return err
		}
		if password != confirmation {
			return fmt.Errorf("passwords do not match")
		}
		db, err := openSetupDatabase(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer db.close()
		if err := bootstrap.CreateFirstAdmin(cmd.Context(), db.pool, orgName, orgSlug, email, customDomain, password); err != nil {
			return err
		}
		fmt.Printf("Created organization %q and initial admin %s. TOTP enrollment will be required at first login.\n", orgName, email)
		return nil
	}}
	command.Flags().StringVar(&orgName, "org-name", "", "Initial organization name")
	command.Flags().StringVar(&orgSlug, "org-slug", "", "Initial organization slug")
	command.Flags().StringVar(&email, "admin-email", "", "Initial admin email")
	command.Flags().StringVar(&customDomain, "domain-name", "", "Initial admin email (ex: ashrix.io)")
	_ = command.MarkFlagRequired("org-name")
	_ = command.MarkFlagRequired("org-slug")
	_ = command.MarkFlagRequired("admin-email")
	_ = command.MarkFlagRequired("domain-name")
	return command
}

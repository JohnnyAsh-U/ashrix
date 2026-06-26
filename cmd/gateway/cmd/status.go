package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Ashrix Gateway status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadFromViper()

	fmt.Println("=== Ashrix Gateway Status ===")

	// --- Process status ---
	data, err := os.ReadFile(cfg.PIDFile)
	if err != nil {
		fmt.Println("process: stopped")
	} else {
		pidStr := strings.TrimSpace(string(data))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			fmt.Println("process: unknown (corrupt pid file)")
		} else {
			proc, err := os.FindProcess(pid)
			if err != nil {
				fmt.Printf("process: stopped (stale pid %d)\n", pid)
			} else if proc.Signal(syscall.Signal(0)) != nil {
				fmt.Printf("process: stopped (stale pid %d)\n", pid)
			} else {
				fmt.Printf("process: running (pid %d)\n", pid)
			}
		}
	}

	// --- Key and Cert Path ---

	keyPath := filepath.Join(cfg.DataDir, "gateway.key.enc")
	certPath := filepath.Join(cfg.DataDir, "gateway.crt")
	_, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, "SECRET", "GATEWAY")

	fmt.Println(keyPath)
	if err != nil {
		fmt.Println("certificate or key: not found — run register")
	}

	// --- Certificate status ---
	expiry := cert.NotAfter
	remaining := time.Until(expiry)

	switch {
	case time.Now().After(expiry):
		fmt.Printf("certificate: EXPIRED at %s\n",
			expiry.Format(time.RFC3339))
	case remaining < 7*24*time.Hour:
		fmt.Printf("certificate: expiring soon (%s remaining, expires %s)\n",
			remaining.Round(time.Hour),
			expiry.Format(time.RFC3339))
	default:
		fmt.Printf("certificate: valid (expires %s, %s remaining)\n",
			expiry.Format(time.RFC3339),
			remaining.Round(time.Hour))
	}

	fmt.Printf("  common name: %s\n", cert.Subject.CommonName)
	fmt.Printf("  issuer:      %s\n", cert.Issuer.CommonName)

	// --- Key status ---

	info, _ := os.Stat(keyPath)
	mode := info.Mode().Perm()
	if mode != 0600 {
		// Key exists but wrong permissions — security warning
		fmt.Printf("private key: WARNING — permissions are %o, should be 0600\n", mode)
	} else {
		fmt.Println("private key: present (permissions OK)")
	}

	// --- Config status ---
	gatewayID := viper.GetString("gateway_id")
	cpURL := viper.GetString("cp_url")

	if gatewayID == "" {
		fmt.Println("gateway id: not registered")
	} else {
		fmt.Printf("gateway id: %s\n", gatewayID)
	}

	if cpURL == "" {
		fmt.Println("control plane: not configured")
	} else {
		fmt.Printf("control plane: %s\n", cpURL)
	}

	return nil
}

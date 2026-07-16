package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	pki_utils "github.com/JohnnyAsh-U/ashrix-api/pkg/pki"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Ashrix Connector status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	baseDir := viper.GetString("basedir")
	pidFile := filepath.Join(baseDir, "connector.pid")

	fmt.Println("=== Ashrix Connector Status ===")

	// --- Process status ---
	data, err := os.ReadFile(pidFile)
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
	certDir := filepath.Join(baseDir, "certs")
	keyPath := filepath.Join(certDir, "connector.key.enc")
	certPath := filepath.Join(certDir, "connector.crt")
	_, cert, err := pki_utils.LoadKeyAndCert(keyPath, certPath, "SECRET", "CONNECTOR")

	if err != nil {
		fmt.Println("certificate or key: not found — run start to register")
	} else {
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

		fmt.Printf("  ConnectorID: %s\n", cert.Subject.CommonName)
		fmt.Printf("  Issuer:      %s\n", cert.Issuer.CommonName)
	}

	// --- Key status ---
	info, err := os.Stat(keyPath)
	if err == nil {
		mode := info.Mode().Perm()
		if mode != 0600 {
			// Key exists but wrong permissions — security warning
			fmt.Printf("private key: WARNING — permissions are %o, should be 0600\n", mode)
		} else {
			fmt.Println("private key: present (permissions OK)")
		}
	}

	return nil
}
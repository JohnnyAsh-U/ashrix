package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running Ashrix Gateway",
	RunE:  runStop,
}



func runStop(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadFromViper()

	data, err := os.ReadFile(cfg.PIDFile)
	if err != nil {
		// Not running — not an error, just say so.
		fmt.Println("gateway is not running (no pid file found)")
		return nil
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("corrupt pid file at %s: %w", cfg.PIDFile, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process gone — clean up stale pid file
		os.Remove(cfg.PIDFile)
		fmt.Printf("gateway (pid %d) was not running — cleaned up stale pid file\n", pid)
		return nil
	}

	// Verify process is actually alive before signaling.
	// Signal(0) checks existence without sending a real signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		os.Remove(cfg.PIDFile)
		fmt.Printf("gateway (pid %d) was not running — cleaned up stale pid file\n", pid)
		return nil
	}

	// SIGTERM — gives the gateway a chance to shut down cleanly:
	// finish in-flight requests, flush logs, remove pid file.
	// The gateway's signal handler in start.go handles this.
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal gateway (pid %d): %w", pid, err)
	}

	fmt.Printf("gateway (pid %d) signaled to stop\n", pid)
	return nil
}

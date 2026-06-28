package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/JohnnyAsh-U/ashrix-api/internal/connector/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running Ashrix Connector",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	baseDir := config.BaseDir()
	pidFile := filepath.Join(baseDir, "connector.pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("connector is not running (no pid file found)")
		return nil
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("corrupt pid file at %s: %w", pidFile, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		fmt.Printf("connector (pid %d) was not running — cleaned up stale pid file\n", pid)
		return nil
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		os.Remove(pidFile)
		fmt.Printf("connector (pid %d) was not running — cleaned up stale pid file\n", pid)
		return nil
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal connector (pid %d): %w", pid, err)
	}

	fmt.Printf("connector (pid %d) signaled to stop\n", pid)
	return nil
}

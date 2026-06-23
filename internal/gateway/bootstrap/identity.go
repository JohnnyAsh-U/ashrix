package bootstrap

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// IsPIDAlive checks whether a running process owns the given PID file.
//
// Handles three cases:
//   - No PID file → not running (return false)
//   - PID file exists, process alive → running (return true)
//   - PID file exists, process dead → stale file (return false)
//
// This is called at the top of runStart to prevent two gateway instances
// running simultaneously with the same identity — a security event.
func IsPIDAlive(pidFile string) bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		// No PID file — definitely not running
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Corrupt PID file — treat as not running
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal(0) checks if the process exists without actually sending a signal.
	// Returns nil if the process is alive, error if it's dead.
	return proc.Signal(syscall.Signal(0)) == nil
}


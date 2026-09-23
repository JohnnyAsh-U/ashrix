//go:build !windows

package cmd

func runServiceOrConsole() error {
	return runStart()
}

//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
)

func install() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Ashrix Connector
After=network.target

[Service]
Type=simple
ExecStart=%s start
Restart=always
RestartSec=10
User=%s
Environment="HOME=%s"

[Install]
WantedBy=multi-user.target
`, exePath, user, os.Getenv("HOME"))

	servicePath := "/etc/systemd/system/ashrix-connector.service"

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file (need sudo?): %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "ashrix-connector").Run()
	exec.Command("systemctl", "start", "ashrix-connector").Run()

	return nil
}

func uninstall() error {
	exec.Command("systemctl", "stop", "ashrix-connector").Run()
	exec.Command("systemctl", "disable", "ashrix-connector").Run()
	return os.Remove("/etc/systemd/system/ashrix-connector.service")
}
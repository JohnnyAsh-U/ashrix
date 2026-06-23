package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteConfig(cpURL, gatewayID, dataDir, configDir, logDir string) error {
	
	configPath := filepath.Join(configDir, "gateway.yaml")

	config := fmt.Sprintf(`
# Ashrix Gateway Configuration
# Copy to gateway.yaml and edit.
# Written automatically by: ashrix-gateway register
#
# Control Plane URL — assigned by Ashrix
cp_url: %s

# Gateway ID — assigned by CP on registration, do not edit manually
gateway_id: "%s"

# Where the gateway stores its identity (key, cert, pid file)
# Must be writable by the gateway process user
data_dir: %s
# Where the app stores its logs and audit logs
log_dir: %s
`, cpURL, gatewayID, dataDir, logDir,
)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}


	tmpFile, err := os.CreateTemp(configDir, "gateway-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}

	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(config); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write config: %w", err)
	}

	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod config: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close config: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("move config into place: %w", err)
	}

	return nil
}

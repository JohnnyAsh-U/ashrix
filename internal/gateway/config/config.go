package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the Gateway's runtime configuration.
type Config struct {
	NodeID         string
	CPAddr         string // Control Plane gRPC address
	HTTPAddr       string // Address for user traffic
	ConnectorAddr  string // Address for gRPC connector streams
	BootstrapToken string // Only used for initial enrollment
	UnlockSecret   string // 32-byte secret to encrypt the private key
	BasePath       string // Storage for certs and keys
}

// Load reads gateway configuration from environment variables.
func Load() (*Config, error) {
	require := func(key string) string {
		v := os.Getenv(key)
		return v
	}

	optional := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	nodeID := require("ASHRIX_NODE_ID")
	if nodeID == "" {
		return nil, fmt.Errorf("required env var ASHRIX_NODE_ID is not set")
	}

	unlockSecret := require("ASHRIX_UNLOCK_SECRET")
	if unlockSecret == "" {
		return nil, fmt.Errorf("required env var ASHRIX_UNLOCK_SECRET is not set")
	}

	home, _ := os.UserHomeDir()
	basePath := optional("ASHRIX_BASE_PATH", filepath.Join(home, ".ashrix", "gateway"))

	cfg := &Config{
		NodeID:         nodeID,
		CPAddr:         optional("ASHRIX_CP_ADDR", "cp.ashrix.io:443"),
		HTTPAddr:       optional("ASHRIX_HTTP_ADDR", "0.0.0.0:443"),
		ConnectorAddr:  optional("ASHRIX_CONNECTOR_ADDR", "0.0.0.0:4443"),
		BootstrapToken: os.Getenv("ASHRIX_BOOTSTRAP_TOKEN"),
		UnlockSecret:   unlockSecret,
		BasePath:       basePath,
	}

	return cfg, nil
}

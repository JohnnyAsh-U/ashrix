package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ControlPlaneURL string `yaml:"control_plane_url"`
	GatewayURL      string `yaml:"gateway_url"`
	DefaultGateway  string `yaml:"default_gateway"`
	Debug           bool   `yaml:"debug"`
}

func DefaultConfig() *Config {
	return &Config{
		ControlPlaneURL: getEnvOrDefault("ASHRIX_CP_URL", "http://localhost:8080"),
		GatewayURL:      getEnvOrDefault("ASHRIX_GATEWAY_URL", "localhost:8443"),
		DefaultGateway:  getEnvOrDefault("ASHRIX_DEFAULT_GATEWAY", "localhost:8443"),
		Debug:           os.Getenv("ASHRIX_DEBUG") == "true" || os.Getenv("ASHRIX_DEBUG") == "1",
	}
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home directory unavailable: %w", err)
	}
	dir := filepath.Join(home, ".config", "ashrix")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return dir, nil
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	dir, err := GetConfigDir()
	if err != nil {
		return cfg, nil // Fall back to default/env config
	}

	configPath := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Environment variable overrides
	if envCP := os.Getenv("ASHRIX_CP_URL"); envCP != "" {
		cfg.ControlPlaneURL = envCP
	}
	if envGW := os.Getenv("ASHRIX_GATEWAY_URL"); envGW != "" {
		cfg.GatewayURL = envGW
	}
	if os.Getenv("ASHRIX_DEBUG") == "true" || os.Getenv("ASHRIX_DEBUG") == "1" {
		cfg.Debug = true
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

package config

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
)

type Config struct {
	CPURL     string
	Token     string
	GatewayID string
	DataDir   string
	LogDir string
	PIDFile   string
}

// Load reads gateway configuration from environment variables.
func LoadFromViper() (*Config, error) {

	cfg := &Config{
		CPURL:     viper.GetString("cp_url"),
		Token:     viper.GetString("token"),
		GatewayID: viper.GetString("gateway_id"),
		LogDir: viper.GetString("log_dir"),
		DataDir:   viper.GetString("data_dir"),
		PIDFile:   filepath.Join(viper.GetString("data_dir"), "gateway.pid"),
	}

	return cfg, nil
}

func Validate(cfg *Config) error {
	if cfg.CPURL == "" {
		return fmt.Errorf(
			"cp_url is required\n\n" +
				"Set via flag:    --cp-url=https://cp.ashrix.io\n",
		)
	}

	if cfg.GatewayID == "" {
		return fmt.Errorf(
			"gateway is not registered\n\n" +
				"Run: ashrix-gateway register --cp-url=<url> --token=<string>",
		)
	}

	return nil
}

package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

type App struct {
	ID                    string `mapstructure:"id"`
	Name                  string `mapstructure:"name"`
	Subdomain             string `mapstructure:"subdomain"`
	Upstream              string `mapstructure:"upstream"`
	Protocol              string `mapstructure:"protocol"`
	IsPublic              bool   `mapstructure:"is_public"`
	EnableSecurityHeaders bool   `mapstructure:"enable_security_headers"`
	CheckHealth           bool   `mapstructure:"check_health"`
	CheckInterval         int32  `mapstructure:"check_interval"`
	HealthEndpoint        string `mapstructure:"health_endpoint"`
}

type Config struct {
	ConnectorID     string
	TenantID        string
	GatewayGRPCAddr string
	GatewayQUICAddr string
	OpenSock        bool
	Apps            []App
	DataDir         string
	LogDir          string
	PIDFile         string
}

func LoadFromViper() (*Config, error) {
	apps := make([]App, 0)
	if err := viper.UnmarshalKey("apps", &apps); err != nil {
		return nil, fmt.Errorf("decode apps configuration: %w", err)
	}
	dataDir := viper.GetString("basedir")
	return &Config{
		ConnectorID:     viper.GetString("connector.id"),
		TenantID:        viper.GetString("connector.tenant_id"),
		GatewayGRPCAddr: viper.GetString("gateway.grpc_address"),
		GatewayQUICAddr: viper.GetString("gateway.quic_address"),
		OpenSock:        viper.GetBool("connector.open_sock"),
		Apps:            apps,
		DataDir:         dataDir,
		LogDir:          viper.GetString("logdir"),
		PIDFile:         filepath.Join(dataDir, "connector.pid"),
	}, nil
}

func Validate(cfg *Config) error {
	if cfg.ConnectorID == "" {
		return fmt.Errorf("connector.id is required")
	}
	if cfg.TenantID == "" {
		return fmt.Errorf("connector.tenant_id is required")
	}
	if cfg.GatewayGRPCAddr == "" || cfg.GatewayQUICAddr == "" {
		return fmt.Errorf("gateway.grpc_address and gateway.quic_address are required")
	}
	if len(cfg.Apps) == 0 {
		return fmt.Errorf("at least one app must be configured")
	}
	for i, app := range cfg.Apps {
		if app.ID == "" || app.Subdomain == "" || app.Upstream == "" {
			return fmt.Errorf("apps[%d] requires id, subdomain, and upstream", i)
		}
	}
	return nil
}

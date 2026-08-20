package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	CPURL     string
	Token     string
	GatewayID string
	GatewayName string
	GatewayUrl string
	TenantId string
	DataDir   string
	LogDir    string
	PIDFile   string
	HTTPPort string
	GRPCPort  string
	QUICPort  string
	RedisAddr string
	RedisPassword string
	SessionTTL time.Duration
	CookieSecure bool
}

// Load reads gateway configuration from environment variables.
func LoadFromViper() (*Config, error) {
	viper.SetDefault("http_port", "8000")
	viper.SetDefault("grpc_port", "9444")
	viper.SetDefault("quic_port", "9445")
	viper.SetDefault("session_ttl", "8h")
	viper.SetDefault("cookie_secure", "false")

	

	cfg := &Config{
		CPURL:     viper.GetString("cp_url"),
		Token:     viper.GetString("token"),
		GatewayID: viper.GetString("gateway_id"),
		GatewayName: viper.GetString("gateway_name"),
		GatewayUrl: viper.GetString("gateway_url"),
		TenantId: viper.GetString("tenant_id"),
		LogDir:    viper.GetString("log_dir"),
		DataDir:   viper.GetString("data_dir"),
		PIDFile:   filepath.Join(viper.GetString("data_dir"), "gateway.pid"),
		HTTPPort:  viper.GetString("http_port"),
		GRPCPort:  viper.GetString("grpc_port"),
		QUICPort:  viper.GetString("quic_port"),
		RedisAddr: viper.GetString("redis_url"),
		RedisPassword: viper.GetString("redis_password"),
		SessionTTL: viper.GetDuration("session_ttl"),
		CookieSecure: viper.GetBool("cookie_secure"),
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

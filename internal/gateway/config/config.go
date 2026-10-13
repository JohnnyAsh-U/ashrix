package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	CPURL            string
	Token            string
	GatewayID        string
	GatewayName      string
	GatewayUrl       string
	TenantId         string
	DataDir          string
	LogDir           string
	PIDFile          string
	HTTPSPort        string
	GRPCPort         string
	QUICPort         string
	RedisAddr        string
	RedisPassword    string
	SessionTTL       time.Duration
	CookieSecure     bool
	IsProdEnv        bool
	BundlePublicKey  string
	MaxOfflinePeriod time.Duration

	// ── OpenTelemetry ─────────────────────────────────────────────────────────
	// OTelEnabled controls whether OTel is initialised. Set OTEL_ENABLED=false
	// to disable telemetry entirely (default: true).
	OTelEnabled bool
	// OTelEndpoint is the OTLP gRPC collector address, e.g. "localhost:4317".
	// Maps to env var OTEL_EXPORTER_OTLP_ENDPOINT.
	OTelEndpoint string
	// OTelInsecure disables TLS for the collector connection. Set
	// OTEL_EXPORTER_OTLP_INSECURE=true in development.
	OTelInsecure bool
	// OTelServiceName overrides the service name in telemetry. Defaults to
	// "ashrix-gateway". Maps to OTEL_SERVICE_NAME.
	OTelServiceName string
	// OTelServiceVersion is the binary version reported in resource attributes.
	// Maps to OTEL_SERVICE_VERSION.
	OTelServiceVersion string
	// OTelMetricsInterval is how often metrics are pushed to the collector.
	// Maps to OTEL_METRICS_INTERVAL (duration string, e.g. "15s"). Default 15s.
	OTelMetricsInterval time.Duration
}

// Load reads gateway configuration from environment variables.
func LoadFromViper() (*Config, error) {
	viper.SetDefault("https_port", "8443")
	viper.SetDefault("grpc_port", "8443")
	viper.SetDefault("quic_port", "8443")
	viper.SetDefault("session_ttl", "8h")
	viper.SetDefault("is_prod_env", true)
	viper.SetDefault("cookie_secure", "false")
	viper.SetDefault("max_offline_period", "24h")

	// OTel defaults
	viper.SetDefault("otel_enabled", false)
	viper.SetDefault("otel_service_name", "ashrix-gateway")
	viper.SetDefault("otel_metrics_interval", "15s")

	metricsInterval := viper.GetDuration("otel_metrics_interval")
	if metricsInterval == 0 {
		metricsInterval = 15 * time.Second
	}

	cfg := &Config{
		CPURL:            viper.GetString("cp_url"),
		Token:            viper.GetString("token"),
		GatewayID:        viper.GetString("gateway_id"),
		GatewayName:      viper.GetString("gateway_name"),
		GatewayUrl:       viper.GetString("gateway_url"),
		TenantId:         viper.GetString("tenant_id"),
		LogDir:           viper.GetString("log_dir"),
		DataDir:          viper.GetString("data_dir"),
		PIDFile:          filepath.Join(viper.GetString("data_dir"), "gateway.pid"),
		HTTPSPort:        viper.GetString("https_port"),
		GRPCPort:         viper.GetString("grpc_port"),
		QUICPort:         viper.GetString("quic_port"),
		RedisAddr:        viper.GetString("redis_url"),
		RedisPassword:    viper.GetString("redis_password"),
		SessionTTL:       viper.GetDuration("session_ttl"),
		CookieSecure:     viper.GetBool("cookie_secure"),
		IsProdEnv:        viper.GetBool("is_prod_env"),
		BundlePublicKey:  viper.GetString("bundle_public_key"),
		MaxOfflinePeriod: viper.GetDuration("max_offline_period"),
		// OTel
		OTelEnabled:         viper.GetBool("otel_enabled"),
		OTelEndpoint:        viper.GetString("otel_exporter_otlp_endpoint"),
		OTelInsecure:        viper.GetBool("otel_exporter_otlp_insecure"),
		OTelServiceName:     viper.GetString("otel_service_name"),
		OTelServiceVersion:  viper.GetString("otel_service_version"),
		OTelMetricsInterval: metricsInterval,
	}

	return cfg, nil
}

func Validate(cfg *Config) error {
	if cfg.MaxOfflinePeriod <= 0 {
		return fmt.Errorf("max_offline_period must be greater than zero")
	}
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
	if cfg.BundlePublicKey == "" {
		return fmt.Errorf("BundlePublicKey is required to verify Control Plane policy signatures")
	}

	return nil
}

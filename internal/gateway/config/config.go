package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	GatewayID               string
	DataDir                 string
	LogDir                  string
	PIDFile                 string
	HTTPSPort               string
	GRPCPort                string
	QUICPort                string
	AppTokenHashes          map[string]string
	AuthorizedConnectors    []string
	RevokedConnectorSerials []string

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
	// OTel defaults
	viper.SetDefault("otel_enabled", true)
	viper.SetDefault("otel_service_name", "ashrix-gateway")
	viper.SetDefault("otel_metrics_interval", "15s")

	metricsInterval := viper.GetDuration("otel_metrics_interval")
	if metricsInterval == 0 {
		metricsInterval = 15 * time.Second
	}

	cfg := &Config{
		GatewayID:               viper.GetString("gateway_id"),
		LogDir:                  viper.GetString("log_dir"),
		DataDir:                 viper.GetString("data_dir"),
		PIDFile:                 filepath.Join(viper.GetString("data_dir"), "gateway.pid"),
		HTTPSPort:               viper.GetString("https_port"),
		GRPCPort:                viper.GetString("grpc_port"),
		QUICPort:                viper.GetString("quic_port"),
		AppTokenHashes:          viper.GetStringMapString("app_token_hashes"),
		AuthorizedConnectors:    viper.GetStringSlice("authorized_connectors"),
		RevokedConnectorSerials: viper.GetStringSlice("revoked_connector_serials"),
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
	if cfg.GatewayID == "" {
		return fmt.Errorf("gateway_id is required")
	}
	if cfg.DataDir == "" || cfg.LogDir == "" {
		return fmt.Errorf("data_dir and log_dir are required")
	}
	if len(cfg.AuthorizedConnectors) == 0 {
		return fmt.Errorf("authorized_connectors must contain at least one connector ID")
	}

	return nil
}

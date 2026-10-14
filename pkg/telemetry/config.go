package telemetry

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ConfigFromEnv reads the shared OTLP environment variables for a service.
func ConfigFromEnv(serviceName, serviceVersion string) (Config, error) {
	config := Config{
		Endpoint:        os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:     serviceName,
		ServiceVersion:  serviceVersion,
		MetricsInterval: 15 * time.Second,
		Enabled:         true,
	}

	if value := os.Getenv("OTEL_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid OTEL_ENABLED value %q: %w", value, err)
		}
		config.Enabled = enabled
	}
	if value := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"); value != "" {
		insecure, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_INSECURE value %q: %w", value, err)
		}
		config.Insecure = insecure
	}
	if value := os.Getenv("OTEL_SERVICE_NAME"); value != "" {
		config.ServiceName = value
	}
	if value := os.Getenv("OTEL_SERVICE_VERSION"); value != "" {
		config.ServiceVersion = value
	}
	if value := os.Getenv("OTEL_METRICS_INTERVAL"); value != "" {
		interval, err := time.ParseDuration(value)
		if err != nil || interval <= 0 {
			return Config{}, fmt.Errorf("invalid OTEL_METRICS_INTERVAL value %q", value)
		}
		config.MetricsInterval = interval
	}

	return config, nil
}

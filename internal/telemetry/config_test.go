package telemetry

import (
	"testing"
	"time"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	for _, key := range []string{
		"OTEL_ENABLED",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_INSECURE",
		"OTEL_SERVICE_NAME",
		"OTEL_SERVICE_VERSION",
		"OTEL_METRICS_INTERVAL",
	} {
		t.Setenv(key, "")
	}

	config, err := ConfigFromEnv("ashrix-test", "v1.2.3")
	if err != nil {
		t.Fatalf("ConfigFromEnv returned an error: %v", err)
	}
	if !config.Enabled {
		t.Fatal("telemetry should be enabled by default")
	}
	if config.ServiceName != "ashrix-test" || config.ServiceVersion != "v1.2.3" {
		t.Fatalf("unexpected service identity: %q %q", config.ServiceName, config.ServiceVersion)
	}
	if config.MetricsInterval != 15*time.Second {
		t.Fatalf("unexpected metrics interval: %s", config.MetricsInterval)
	}
}

func TestConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "false")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_SERVICE_NAME", "ashrix-custom")
	t.Setenv("OTEL_SERVICE_VERSION", "v2.0.0")
	t.Setenv("OTEL_METRICS_INTERVAL", "30s")

	config, err := ConfigFromEnv("ashrix-test", "v1.2.3")
	if err != nil {
		t.Fatalf("ConfigFromEnv returned an error: %v", err)
	}
	if config.Enabled || !config.Insecure {
		t.Fatalf("unexpected boolean settings: enabled=%t insecure=%t", config.Enabled, config.Insecure)
	}
	if config.Endpoint != "collector:4317" || config.ServiceName != "ashrix-custom" || config.ServiceVersion != "v2.0.0" {
		t.Fatalf("unexpected endpoint or service identity: %+v", config)
	}
	if config.MetricsInterval != 30*time.Second {
		t.Fatalf("unexpected metrics interval: %s", config.MetricsInterval)
	}
}

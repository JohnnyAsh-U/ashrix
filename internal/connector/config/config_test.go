package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoadAndValidateLocalConnectorConfiguration(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("basedir", t.TempDir())
	viper.Set("logdir", t.TempDir())
	viper.Set("connector.id", "connector-local")
	viper.Set("connector.tenant_id", "tenant-local")
	viper.Set("gateway.grpc_address", "gateway.example.test:9443")
	viper.Set("gateway.quic_address", "gateway.example.test:8443")
	viper.Set("apps", []map[string]any{{
		"id":        "app-local",
		"name":      "Local app",
		"subdomain": "app",
		"upstream":  "127.0.0.1:8080",
		"protocol":  "http",
	}})

	cfg, err := LoadFromViper()
	if err != nil {
		t.Fatalf("LoadFromViper: %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.ConnectorID != "connector-local" || cfg.GatewayQUICAddr != "gateway.example.test:8443" || len(cfg.Apps) != 1 || cfg.Apps[0].Upstream != "127.0.0.1:8080" {
		t.Fatalf("unexpected local connector config: %#v", cfg)
	}
}

func TestValidateRequiresGatewayAndAppConfig(t *testing.T) {
	cfg := &Config{ConnectorID: "connector-local", TenantID: "tenant-local"}
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted missing gateway and app configuration")
	}
}

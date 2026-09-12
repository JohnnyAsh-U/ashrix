package config_test

import (
	"os"
	"testing"

	"github.com/JohnnyAsh-U/ashrix-api/internal/client/config"
)

func TestConfigDefaultsAndEnv(t *testing.T) {
	os.Setenv("ASHRIX_CP_URL", "http://test-cp.local:8080")
	os.Setenv("ASHRIX_GATEWAY_URL", "test-gw.local:8443")
	os.Setenv("ASHRIX_DEBUG", "true")

	cfg := config.DefaultConfig()

	if cfg.ControlPlaneURL != "http://test-cp.local:8080" {
		t.Errorf("expected CP URL from env, got %s", cfg.ControlPlaneURL)
	}
	if cfg.GatewayURL != "test-gw.local:8443" {
		t.Errorf("expected Gateway URL from env, got %s", cfg.GatewayURL)
	}
	if !cfg.Debug {
		t.Errorf("expected Debug to be true")
	}
}

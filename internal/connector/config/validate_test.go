package config

import "testing"

func validConfig() Config {
	return Config{
		Connector:    Connector{ID: "office-01", Certificate: "/tls/client.crt", PrivateKey: "/tls/client.key", CA: "/tls/ca.crt"},
		Gateway:      Gateway{Address: "gateway.example.com:9443", ServerName: "gateway.example.com"},
		Applications: []Application{{Name: "grafana", Target: "127.0.0.1:3000"}},
		Logging:      Logging{Level: "info", Format: "json"},
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidConnectorID(t *testing.T) {
	cfg := validConfig()
	cfg.Connector.ID = "../connector"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid connector ID error")
	}
}

func TestConfigValidateRejectsInvalidGatewayAddress(t *testing.T) {
	cfg := validConfig()
	cfg.Gateway.Address = "gateway.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid gateway address error")
	}
}

func TestConfigValidateRejectsInvalidServerName(t *testing.T) {
	for _, name := range []string{"", " gateway.example.com", "https://gateway.example.com", "gateway.example.com:443", "bad..example.com"} {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Gateway.ServerName = name
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() expected invalid server name error for %q", name)
			}
		})
	}
}

func TestConfigValidateRejectsDuplicateApplicationNames(t *testing.T) {
	cfg := validConfig()
	cfg.Applications = append(cfg.Applications, Application{Name: "grafana", Target: "127.0.0.1:3001"})
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected duplicate application name error")
	}
}

func TestConfigValidateRejectsInvalidTarget(t *testing.T) {
	cfg := validConfig()
	cfg.Applications[0].Target = "127.0.0.1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid target error")
	}
}

func TestConfigValidateRejectsUnsupportedLoggingValues(t *testing.T) {
	cfg := validConfig()
	cfg.Logging.Level = "trace"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid logging level error")
	}
	cfg = validConfig()
	cfg.Logging.Format = "xml"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid logging format error")
	}
}

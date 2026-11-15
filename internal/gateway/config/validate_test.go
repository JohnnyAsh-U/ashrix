package config

import "testing"

func validConfig() Config {
	return Config{
		Gateway: Gateway{ID: "gateway-01", ServerName: "gateway.example.com"},
		Listeners: Listeners{
			HTTP:  HTTPListener{Address: "127.0.0.1:8080"},
			HTTPS: HTTPSListener{Address: "127.0.0.1:8443", Certificate: "/tls/server.crt", PrivateKey: "/tls/server.key"},
			QUIC:  QUICListener{Address: "127.0.0.1:9443"},
		},
		PKI:          PKI{CACertificate: "/pki/ca.crt", CAPrivateKey: "/pki/ca.key"},
		Connectors:   []Connector{{ID: "office-01", CertificateSerial: "01:ab"}},
		Applications: []Application{{Name: "grafana", Domain: "grafana.example.com", Connector: "office-01"}},
		Logging:      Logging{Level: "info", Format: "json"},
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidGatewayIdentity(t *testing.T) {
	cfg := validConfig()
	cfg.Gateway.ID = "../gateway"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid gateway ID error")
	}
}

func TestConfigValidateRejectsDuplicateListenerAddresses(t *testing.T) {
	cfg := validConfig()
	cfg.Listeners.QUIC.Address = cfg.Listeners.HTTP.Address
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected duplicate listener error")
	}
}

func TestConfigValidateRejectsDuplicateConnectorIDs(t *testing.T) {
	cfg := validConfig()
	cfg.Connectors = append(cfg.Connectors, cfg.Connectors[0])
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected duplicate connector error")
	}
}

func TestConfigValidateRejectsUnknownConnectorReference(t *testing.T) {
	cfg := validConfig()
	cfg.Applications[0].Connector = "missing"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected unknown connector error")
	}
}

func TestConfigValidateRejectsDuplicateDomainsCaseInsensitively(t *testing.T) {
	cfg := validConfig()
	cfg.Applications = append(cfg.Applications, Application{Name: "prometheus", Domain: "GRAFANA.example.com", Connector: "office-01"})
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected duplicate domain error")
	}
}

func TestConfigValidateRejectsInvalidCertificateSerial(t *testing.T) {
	cfg := validConfig()
	cfg.Connectors[0].CertificateSerial = "not-a-serial"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid serial error")
	}
}

func TestValidDomain(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"normal", "grafana.example.com", true},
		{"uppercase", "GRAFANA.Example.com", true},
		{"ip", "192.0.2.1", false},
		{"wildcard", "*.example.com", false},
		{"empty label", "grafana..example.com", false},
		{"leading hyphen", "-grafana.example.com", false},
		{"trailing dot", "grafana.example.com.", false},
		{"path", "example.com/path", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validDomain(tc.input); got != tc.want {
				t.Fatalf("validDomain(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

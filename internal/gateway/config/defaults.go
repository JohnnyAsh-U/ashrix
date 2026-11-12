package config


func Default() Config {
	return Config{
		Gateway: Gateway{
			ID: "gateway-01",
		},
		Listeners: Listeners{
			HTTP: HTTPListener{
				Address: "0.0.0.0:80",
			},
			HTTPS: HTTPSListener{
				Address:     "0.0.0.0:443",
				Certificate: "/etc/ashrix-gateway/certs/server.crt",
				PrivateKey:  "/etc/ashrix-gateway/certs/server.key",
			},
			QUIC: QUICListener{
				Address: "0.0.0.0:8443",
			},
		},
		PKI: PKI{
			CACertificate:       "/etc/ashrix-gateway/pki/ca.crt",
			CAPrivateKey:        "/etc/ashrix-gateway/pki/ca.key",
		},
		Connectors:   []Connector{},
		Applications: []Application{},
		Admin: Admin{
			Address: "127.0.0.1:9090",
		},
		Logging: Logging{
			Level:  "info",
			Format: "json",
		},
	}
}

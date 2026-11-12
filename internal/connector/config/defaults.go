
package config


func Defaults() Config {
	return Config{
		Connector: Connector{
			ID: "connector-10",
			Certificate: "/etc/ashrix-connector/certs/connector.crt",
			CA: "/etc/ashrix-connector/certs/ca.crt",
		},
		Gateway: Gateway{
			Address:    "127.0.0.1:8443",
		},
		Applications: []Application{},
		Metrics: Metrics{
			Enabled: true,
			Listen: "localhost:9090",
		},
		Logging: Logging{
			Level:  "info",
			Format: "json",
		},
	}
}

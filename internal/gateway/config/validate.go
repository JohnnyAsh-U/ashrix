package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var connectorIDPattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`,
)

var applicationNamePattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`,
)

var certificateSerialPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func (c Config) Validate() error {
	if !connectorIDPattern.MatchString(c.Gateway.ID) {
		return fmt.Errorf("invalid gateway.id")
	}

	if !validServerName(c.Gateway.ServerName) {
		return fmt.Errorf("invalid gateway.server_name")
	}

	addresses := map[string]string{
		"listeners.http.address":  c.Listeners.HTTP.Address,
		"listeners.https.address": c.Listeners.HTTPS.Address,
		"listeners.quic.address":  c.Listeners.QUIC.Address,
	}

	seenAddresses := make(map[string]string)

	for name, address := range addresses {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return fmt.Errorf("%s: invalid address: %w", name, err)
		}

		// Admin may intentionally share a listener only if a future
		// explicit design allows it. For now reject all duplicates.
		if previous, exists := seenAddresses[address]; exists {
			return fmt.Errorf(
				"%s conflicts with %s on %s",
				name, previous, address,
			)
		}

		seenAddresses[address] = name
	}

	if c.Listeners.HTTPS.Certificate == "" ||
		c.Listeners.HTTPS.PrivateKey == "" {
		return fmt.Errorf("HTTPS certificate paths are required")
	}

	if c.PKI.CACertificate == "" ||
		c.PKI.CAPrivateKey == "" {
		return fmt.Errorf("PKI paths are required")
	}

	connectorIDs := make(map[string]struct{})

	for _, connector := range c.Connectors {
		if !connectorIDPattern.MatchString(connector.ID) {
			return fmt.Errorf(
				"invalid connector ID: %q",
				connector.ID,
			)
		}

		if _, exists := connectorIDs[connector.ID]; exists {
			return fmt.Errorf(
				"duplicate connector ID: %s",
				connector.ID,
			)
		}

		connectorIDs[connector.ID] = struct{}{}

		serial := strings.ReplaceAll(strings.TrimSpace(connector.CertificateSerial), ":", "")
		if !certificateSerialPattern.MatchString(serial) {
			return fmt.Errorf("invalid certificate serial for connector %q", connector.ID)
		}
	}

	applicationNames := make(map[string]struct{})
	domains := make(map[string]struct{})

	for _, app := range c.Applications {
		if !applicationNamePattern.MatchString(app.Name) {
			return fmt.Errorf(
				"invalid application name: %q",
				app.Name,
			)
		}

		if !validDomain(app.Domain) {
			return fmt.Errorf(
				"invalid application domain: %s",
				app.Domain,
			)
		}

		if _, exists := connectorIDs[app.Connector]; !exists {
			return fmt.Errorf(
				"application %s references unknown connector %s",
				app.Name,
				app.Connector,
			)
		}

		if _, exists := applicationNames[app.Name]; exists {
			return fmt.Errorf(
				"duplicate application name: %s",
				app.Name,
			)
		}

		domain := strings.ToLower(app.Domain)

		if _, exists := domains[domain]; exists {
			return fmt.Errorf(
				"duplicate application domain: %s",
				app.Domain,
			)
		}

		applicationNames[app.Name] = struct{}{}
		domains[domain] = struct{}{}
	}

	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid logging.level")
	}

	switch c.Logging.Format {
	case "json", "text":
	default:
		return fmt.Errorf("invalid logging.format")
	}

	return nil
}

func validDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}

	if strings.Contains(domain, "*") {
		return false
	}

	if strings.HasSuffix(domain, ".") {
		return false
	}

	u, err := url.Parse("https://" + domain)
	if err != nil || u.Host != domain {
		return false
	}

	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}

		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, r := range label {
			if !((r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') ||
				r == '-') {
				return false
			}
		}
	}

	return true
}

func validServerName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name {
		return false
	}
	if net.ParseIP(name) != nil {
		return true
	}
	return validDomain(name)
}

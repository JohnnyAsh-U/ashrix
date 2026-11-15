package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`,
)

var appNamePattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`,
)

func (cfg Config) Validate() error {
	if !idPattern.MatchString(cfg.Connector.ID) {
		return errors.New("invalid connector ID")
	}

	if _, _, err := net.SplitHostPort(cfg.Gateway.Address); err != nil {
		return fmt.Errorf("invalid gateway address: %w", err)
	}

	if strings.TrimSpace(cfg.Gateway.ServerName) == "" {
		return errors.New("gateway.server_name is required")
	}
	if !validServerName(cfg.Gateway.ServerName) {
		return errors.New("gateway.server_name must be a valid DNS name or IP address without a port")
	}

	names := make(map[string]struct{}, len(cfg.Applications))

	for _, app := range cfg.Applications {
		if !appNamePattern.MatchString(app.Name) {
			return fmt.Errorf("invalid application name %q", app.Name)
		}

		if _, exists := names[app.Name]; exists {
			return fmt.Errorf("duplicate application name %q", app.Name)
		}
		names[app.Name] = struct{}{}

		if _, _, err := net.SplitHostPort(app.Target); err != nil {
			return fmt.Errorf(
				"invalid target for application %q: %w",
				app.Name,
				err,
			)
		}
	}

	switch cfg.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("unsupported logging level %q", cfg.Logging.Level)
	}

	switch cfg.Logging.Format {
	case "json", "text":
	default:
		return fmt.Errorf("unsupported logging format %q", cfg.Logging.Format)
	}

	return nil
}

func validServerName(name string) bool {
	if name == "" || strings.TrimSpace(name) != name {
		return false
	}
	if net.ParseIP(name) != nil {
		return true
	}
	if len(name) > 253 || strings.ContainsAny(name, "/\\:") {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}

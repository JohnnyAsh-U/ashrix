
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

	if strings.TrimSpace(cfg.Gateway.Address) == "" {
		return errors.New("gateway server_name is required")
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

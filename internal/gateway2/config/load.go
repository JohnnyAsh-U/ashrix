
package config

import (
	"errors"
	"io"
	"os"
	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config

	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}

	// Reject multiple YAML documents.
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("multiple YAML documents are not supported")
		}
		return Config{}, err
	}

	// PID

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

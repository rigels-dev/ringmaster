package config

import (
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// SaveFile marshals config to TOML format and saves to path
func SaveFile(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to TOML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}

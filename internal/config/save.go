package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// SaveFile marshals config to TOML format and saves to path
func SaveFile(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to TOML: %w", err)
	}

	// The user config dir may not exist yet on a fresh install; the fleet config
	// can hold API keys, so keep both private
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}

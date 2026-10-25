package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteJSONAtomic writes data serialized as JSON to path safely using an atomic rename
func WriteJSONAtomic(path string, data any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmpFile, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename %s to %s: %w", tmpFile, path, err)
	}

	return nil
}

func ReadJSON[T any](path string) (*T, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var res T
	if err := json.Unmarshal(bytes, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", path, err)
	}

	return &res, nil
}

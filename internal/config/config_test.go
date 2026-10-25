package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["test"] = ProviderConfig{BaseURL: "http://localhost:8080"}
	cfg.Agents["master"] = AgentConfig{Provider: "test", Model: "gpt-4", Role: "master"}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "no providers",
			mutate: func(c *Config) {
				c.Providers = make(map[string]ProviderConfig)
			},
			wantErr: "[providers]",
		},
		{
			name: "provider missing base_url",
			mutate: func(c *Config) {
				c.Providers["empty"] = ProviderConfig{BaseURL: ""}
			},
			wantErr: "[providers.empty.base_url]",
		},
		{
			name: "agent unknown provider",
			mutate: func(c *Config) {
				c.Providers["local"] = ProviderConfig{BaseURL: "http://localhost:11434"}
				c.Agents["worker"] = AgentConfig{Provider: "missing", Model: "m1", Role: "worker"}
			},
			wantErr: "[agents.worker.provider]",
		},
		{
			name: "agent invalid role",
			mutate: func(c *Config) {
				c.Providers["local"] = ProviderConfig{BaseURL: "http://localhost:11434"}
				c.Agents["worker"] = AgentConfig{Provider: "local", Model: "m1", Role: "supervisor"}
			},
			wantErr: "[agents.worker.role]",
		},
		{
			name: "estimates min greater than max",
			mutate: func(c *Config) {
				c.Providers["local"] = ProviderConfig{BaseURL: "http://localhost:11434"}
				c.Agents["worker"] = AgentConfig{Provider: "local", Model: "m1", Role: "worker"}
				c.Estimates["backend"] = EstimateConfig{TokensIn: [2]int{50000, 10000}}
			},
			wantErr: "[estimates.backend.tokens_in]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(cfg)
			err := Validate(cfg)
			if err == nil {
				t.Fatalf("expected error containing '%s', got nil", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error '%s' to contain '%s'", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	tempDir := t.TempDir()
	tomlPath := filepath.Join(tempDir, "ringmaster.toml")

	content := `
[providers.local]
base_url = "http://localhost:11434/v1"

[agents.master]
provider = "local"
model = "qwen"
role = "master"

[budget]
max_tokens_per_project = 1000000
max_tokens_per_task = 100000
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(tomlPath)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.Budget.MaxTokensPerProject != 1000000 {
		t.Errorf("expected 1000000, got %d", cfg.Budget.MaxTokensPerProject)
	}
	if len(cfg.Sandbox.Allow) == 0 {
		t.Errorf("expected default sandbox allowances to be set")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || filepath.Base(s) != "" && (len(s) > 0 && len(substr) > 0 && (findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

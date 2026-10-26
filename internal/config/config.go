package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

type ProviderConfig struct {
	Type      string `toml:"type,omitempty"` // "openai" (default), "anthropic", "claude_subscription"
	BaseURL   string `toml:"base_url,omitempty"`
	APIKeyEnv string `toml:"api_key_env,omitempty"`
}

type AgentConfig struct {
	Provider    string `toml:"provider"`
	Model       string `toml:"model"`
	Role        string `toml:"role"` // "master" or "worker"
	ToolCalling bool   `toml:"tool_calling"`
}

type RoutingConfig struct {
	DefaultMaster  string `toml:"default_master"`
	DefaultWorker  string `toml:"default_worker"`
	FallbackWorker string `toml:"fallback_worker"`
}

type BudgetConfig struct {
	MaxTokensPerProject int `toml:"max_tokens_per_project"`
	MaxTokensPerTask    int `toml:"max_tokens_per_task"`
}

type SandboxConfig struct {
	Allow []string `toml:"allow"`
}

type EstimateConfig struct {
	TokensIn  [2]int `toml:"tokens_in"`  // [min, max]
	TokensOut [2]int `toml:"tokens_out"` // [min, max]
}

type Config struct {
	Providers map[string]ProviderConfig `toml:"providers"`
	Agents    map[string]AgentConfig    `toml:"agents"`
	Routing   RoutingConfig             `toml:"routing"`
	Budget    BudgetConfig              `toml:"budget"`
	Sandbox   SandboxConfig             `toml:"sandbox"`
	Estimates map[string]EstimateConfig `toml:"estimates"`
}

var DefaultAllowedCommands = []string{
	"pytest",
	"ruff",
	"mypy",
	"python",
	"node",
	"npm",
	"go test",
	"git status",
	"git diff",
}

func DefaultConfig() *Config {
	return &Config{
		Providers: make(map[string]ProviderConfig),
		Agents:    make(map[string]AgentConfig),
		Budget: BudgetConfig{
			MaxTokensPerProject: 2_000_000,
			MaxTokensPerTask:    300_000,
		},
		Sandbox: SandboxConfig{
			Allow: DefaultAllowedCommands,
		},
		Estimates: make(map[string]EstimateConfig),
	}
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if len(cfg.Providers) == 0 {
		return fmt.Errorf("validation error [providers]: at least one provider must be defined")
	}

	for pName, p := range cfg.Providers {
		pType := p.Type
		if pType == "" {
			pType = "openai"
		}
		if pType != "openai" && pType != "anthropic" && pType != "claude_subscription" && pType != "claude_sub" {
			return fmt.Errorf("validation error [providers.%s.type]: invalid provider type '%s', must be 'openai', 'anthropic', or 'claude_subscription'", pName, p.Type)
		}

		if pType == "claude_subscription" || pType == "claude_sub" {
			continue // Claude subscription uses local claude credentials, no baseURL required
		}

		if p.BaseURL == "" {
			if pType == "anthropic" {
				p.BaseURL = "https://api.anthropic.com/v1"
				cfg.Providers[pName] = p
			} else if pName == "openai" {
				p.BaseURL = "https://api.openai.com/v1"
				cfg.Providers[pName] = p
			} else {
				return fmt.Errorf("validation error [providers.%s.base_url]: base_url cannot be empty", pName)
			}
		}
	}

	if len(cfg.Agents) == 0 {
		return fmt.Errorf("validation error [agents]: at least one agent must be defined")
	}

	for aName, a := range cfg.Agents {
		if a.Provider == "" {
			return fmt.Errorf("validation error [agents.%s.provider]: provider must be specified", aName)
		}
		if _, ok := cfg.Providers[a.Provider]; !ok {
			return fmt.Errorf("validation error [agents.%s.provider]: referenced provider '%s' is not defined", aName, a.Provider)
		}
		if a.Model == "" {
			return fmt.Errorf("validation error [agents.%s.model]: model cannot be empty", aName)
		}
		if a.Role != "master" && a.Role != "worker" {
			return fmt.Errorf("validation error [agents.%s.role]: role must be either 'master' or 'worker', got '%s'", aName, a.Role)
		}
	}

	if cfg.Budget.MaxTokensPerProject <= 0 {
		return fmt.Errorf("validation error [budget.max_tokens_per_project]: must be positive")
	}
	if cfg.Budget.MaxTokensPerTask <= 0 {
		return fmt.Errorf("validation error [budget.max_tokens_per_task]: must be positive")
	}
	if cfg.Budget.MaxTokensPerTask > cfg.Budget.MaxTokensPerProject {
		return fmt.Errorf("validation error [budget.max_tokens_per_task]: cannot exceed max_tokens_per_project")
	}

	for eName, est := range cfg.Estimates {
		if est.TokensIn[0] > est.TokensIn[1] {
			return fmt.Errorf("validation error [estimates.%s.tokens_in]: min (%d) cannot be greater than max (%d)", eName, est.TokensIn[0], est.TokensIn[1])
		}
		if est.TokensOut[0] > est.TokensOut[1] {
			return fmt.Errorf("validation error [estimates.%s.tokens_out]: min (%d) cannot be greater than max (%d)", eName, est.TokensOut[0], est.TokensOut[1])
		}
	}

	return nil
}

func FindConfigFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// 1. Search in current working directory
	target := filepath.Join(cwd, "ringmaster.toml")
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	// 2. Search upwards towards git root
	dir := cwd
	for {
		parent := filepath.Dir(dir)
		target = filepath.Join(dir, "ringmaster.toml")
		if _, err := os.Stat(target); err == nil {
			return target, nil
		}
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("ringmaster.toml not found in %s or parent directories", cwd)
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", path, err)
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML from '%s': %w", path, err)
	}

	if len(cfg.Sandbox.Allow) == 0 {
		cfg.Sandbox.Allow = DefaultAllowedCommands
	}

	return cfg, nil
}

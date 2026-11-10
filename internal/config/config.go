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
	Disabled    bool   `toml:"disabled,omitempty"` // true if agent is toggled inactive
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
	// AcceptanceTimeout is how long each acceptance command may run, in seconds
	// (0 = DefaultAcceptanceTimeout). A task's plan may set its own.
	AcceptanceTimeout int `toml:"acceptance_timeout,omitempty"`
	// PassEnv names environment variables handed to worker and acceptance
	// commands on top of the built-in set (PATH, HOME, toolchain paths, ...).
	// A trailing * matches a prefix. Everything not named stays out.
	PassEnv []string `toml:"pass_env,omitempty"`
}

// DefaultAcceptanceTimeout fits a cold docker build or an integration suite.
const DefaultAcceptanceTimeout = 300

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

	// Note: Validation allows 0 agents initially so the user can configure the fleet in the UI
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

// RootDir returns the Ringmaster installation root: $RINGMASTER_ROOT if set,
// otherwise the directory holding the executable (or its parent when the
// binary lives in bin/). Returns "" when no root can be determined.
func RootDir() string {
	if root := os.Getenv("RINGMASTER_ROOT"); root != "" {
		return root
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir)
	}
	return dir
}

// ConfigDir is where the global fleet config lives: $RINGMASTER_CONFIG_DIR,
// else $XDG_CONFIG_HOME/ringmaster, else ~/.config/ringmaster. It is outside
// the install directory, so package-manager upgrades never touch it.
func ConfigDir() string {
	if dir := os.Getenv("RINGMASTER_CONFIG_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ringmaster")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "ringmaster")
}

// LegacyConfigPath is where versions before v0.2 kept the fleet config: next
// to the install (<root>/ringmaster.toml).
func LegacyConfigPath() string {
	root := RootDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "ringmaster.toml")
}

// GlobalConfigPath returns the global fleet config: the user config when it
// exists, else a legacy one still next to the install, else the user config
// path (where a new config is created).
func GlobalConfigPath() string {
	var user string
	if dir := ConfigDir(); dir != "" {
		user = filepath.Join(dir, "ringmaster.toml")
		if fileExists(user) {
			return user
		}
	}
	if legacy := LegacyConfigPath(); legacy != "" && fileExists(legacy) {
		return legacy
	}
	return user
}

// MigrateLegacyConfig copies a pre-v0.2 config from the install directory to
// the user config directory, once, and returns the new path ("" when there was
// nothing to move). The old file is left in place.
func MigrateLegacyConfig() (string, error) {
	dir := ConfigDir()
	legacy := LegacyConfigPath()
	if dir == "" || legacy == "" || !fileExists(legacy) {
		return "", nil
	}
	user := filepath.Join(dir, "ringmaster.toml")
	if fileExists(user) || filepath.Clean(user) == filepath.Clean(legacy) {
		return "", nil
	}
	data, err := os.ReadFile(legacy)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	// The fleet config can hold API keys
	if err := os.WriteFile(user, data, 0600); err != nil {
		return "", err
	}
	return user, nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// ConfigPath returns the fleet config to use: the explicit --config path when
// given, otherwise the global config (see GlobalConfigPath). It never looks in the
// working directory: inside a project, ringmaster.toml is the project
// manifest, which does not define providers or agents.
func ConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return GlobalConfigPath()
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

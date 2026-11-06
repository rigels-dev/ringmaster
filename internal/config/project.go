package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// ProjectManifest is a project's own ringmaster.toml. It describes the
// project and may tune how its tasks run (sandbox commands, token budget),
// but never defines providers or agents: the fleet is global, and a project
// only assigns fleet agents to its tasks (the "worker" of each task in plan.json).
type ProjectManifest struct {
	Name      string `toml:"name"`
	Version   string `toml:"version"`
	Brief     string `toml:"brief"`
	Plan      string `toml:"plan"`
	CreatedAt string `toml:"created_at"`

	// Optional overrides of the global [budget] and [sandbox] for this project
	Budget  BudgetConfig  `toml:"budget"`
	Sandbox SandboxConfig `toml:"sandbox"`
}

// ProjectManifestPath is where a project's manifest lives.
func ProjectManifestPath(projectDir string) string {
	return filepath.Join(projectDir, "ringmaster.toml")
}

// LoadProjectManifest reads the project's manifest. A missing manifest is not
// an error: the project simply has no overrides.
func LoadProjectManifest(projectDir string) (*ProjectManifest, error) {
	m := &ProjectManifest{}
	data, err := os.ReadFile(ProjectManifestPath(projectDir))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read project manifest: %w", err)
	}
	if err := toml.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("failed to parse project manifest %s: %w", ProjectManifestPath(projectDir), err)
	}
	return m, nil
}

// ForProject returns a copy of the fleet config with the project's budget and
// sandbox overrides applied. The receiver is left untouched, so the overrides
// can never be saved back into the global config.
func (c *Config) ForProject(m *ProjectManifest) *Config {
	out := *c
	if m == nil {
		return &out
	}
	if m.Budget.MaxTokensPerProject > 0 {
		out.Budget.MaxTokensPerProject = m.Budget.MaxTokensPerProject
	}
	if m.Budget.MaxTokensPerTask > 0 {
		out.Budget.MaxTokensPerTask = m.Budget.MaxTokensPerTask
	}
	if len(m.Sandbox.Allow) > 0 {
		out.Sandbox.Allow = append([]string(nil), m.Sandbox.Allow...)
	}
	if m.Sandbox.AcceptanceTimeout > 0 {
		out.Sandbox.AcceptanceTimeout = m.Sandbox.AcceptanceTimeout
	}
	return &out
}

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

	// Optional overrides of the global [budget], [sandbox], [web] and [skills] for this project
	Budget  BudgetConfig  `toml:"budget"`
	Sandbox SandboxConfig `toml:"sandbox"`
	Web     WebConfig     `toml:"web,omitempty"`
	Skills  SkillsConfig  `toml:"skills,omitempty"`
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
	// Unlike allow, a project's pass_env adds to the fleet's: what one project
	// needs (its DATABASE_URL) does not replace what every project needs
	if len(m.Sandbox.PassEnv) > 0 {
		out.Sandbox.PassEnv = append(append([]string(nil), c.Sandbox.PassEnv...), m.Sandbox.PassEnv...)
	}
	if m.Sandbox.AllowTestEdits {
		out.Sandbox.AllowTestEdits = true
	}
	if len(m.Sandbox.TestFiles) > 0 {
		out.Sandbox.TestFiles = append([]string(nil), m.Sandbox.TestFiles...)
	}
	if m.Web.Port > 0 {
		out.Web.Port = m.Web.Port
	}
	// Like pass_env, a project's disabled skills add to the fleet's: a project
	// can turn a skill off for itself, not back on against the fleet
	if len(m.Skills.Disabled) > 0 {
		out.Skills.Disabled = append(append([]string(nil), c.Skills.Disabled...), m.Skills.Disabled...)
	}
	return &out
}

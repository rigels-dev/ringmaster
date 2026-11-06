package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A project manifest never contributes providers or agents, even if someone
// puts them there; only its budget and sandbox overrides apply.
func TestProjectManifestOnlyOverridesBudgetAndSandbox(t *testing.T) {
	dir := t.TempDir()
	manifest := `name = "demo"
brief = ".ringmaster/BRIEF.md"

[budget]
max_tokens_per_task = 50000

[sandbox]
allow = ["go test", "go vet"]

[agents.rogue]
provider = "nowhere"
model = "x"
role = "worker"
`
	if err := os.WriteFile(filepath.Join(dir, "ringmaster.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadProjectManifest(dir)
	if err != nil {
		t.Fatalf("LoadProjectManifest: %v", err)
	}
	if m.Name != "demo" {
		t.Errorf("name = %q, want demo", m.Name)
	}

	fleet := DefaultConfig()
	fleet.Providers["claude_sub"] = ProviderConfig{Type: "claude_subscription"}
	fleet.Agents["haiku"] = AgentConfig{Provider: "claude_sub", Model: "claude-haiku-5-5", Role: "worker"}

	got := fleet.ForProject(m)
	if len(got.Agents) != 1 || got.Agents["haiku"].Model == "" {
		t.Errorf("project must not change the fleet, agents = %+v", got.Agents)
	}
	if got.Budget.MaxTokensPerTask != 50000 {
		t.Errorf("task budget override not applied: %d", got.Budget.MaxTokensPerTask)
	}
	if got.Budget.MaxTokensPerProject != fleet.Budget.MaxTokensPerProject {
		t.Errorf("unset project budget must keep the global value")
	}
	if len(got.Sandbox.Allow) != 2 || got.Sandbox.Allow[0] != "go test" {
		t.Errorf("sandbox override not applied: %v", got.Sandbox.Allow)
	}

	// The global config itself is untouched, so saving it never persists overrides
	if fleet.Budget.MaxTokensPerTask == 50000 || len(fleet.Sandbox.Allow) == 2 {
		t.Errorf("ForProject mutated the global config")
	}
}

func TestLoadProjectManifestMissingIsEmpty(t *testing.T) {
	m, err := LoadProjectManifest(t.TempDir())
	if err != nil || m == nil {
		t.Fatalf("missing manifest should load as empty, got %v, %v", m, err)
	}
}

// LoadFile reads exactly the file it is given; it no longer merges another
// config into a file that lacks agents.
func TestLoadFileDoesNotMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ringmaster.toml")
	if err := os.WriteFile(path, []byte("name = \"manifest-only\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(cfg.Agents) != 0 || len(cfg.Providers) != 0 {
		t.Errorf("expected no fleet from a manifest-only file, got %d agents, %d providers", len(cfg.Agents), len(cfg.Providers))
	}
}

func TestConfigPathPrefersExplicit(t *testing.T) {
	if got := ConfigPath("/x/custom.toml"); got != "/x/custom.toml" {
		t.Errorf("ConfigPath(explicit) = %s", got)
	}
	t.Setenv("RINGMASTER_ROOT", "/opt/ringmaster")
	if got := ConfigPath(""); got != "/opt/ringmaster/ringmaster.toml" {
		t.Errorf("ConfigPath(\"\") = %s, want the global config", got)
	}
}

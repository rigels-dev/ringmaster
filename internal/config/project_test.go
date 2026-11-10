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
	t.Setenv("RINGMASTER_ROOT", t.TempDir())
	userDir := t.TempDir()
	t.Setenv("RINGMASTER_CONFIG_DIR", userDir)
	if got := ConfigPath(""); got != filepath.Join(userDir, "ringmaster.toml") {
		t.Errorf("ConfigPath(\"\") = %s, want the user config", got)
	}
}

// The fleet config lives in the user's config dir, so upgrading a packaged
// install never loses it; a config left next to an older install is still
// used, and is copied over once.
func TestGlobalConfigLocationAndMigration(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(t.TempDir(), "ringmaster")
	t.Setenv("RINGMASTER_ROOT", root)
	t.Setenv("RINGMASTER_CONFIG_DIR", userDir)
	user := filepath.Join(userDir, "ringmaster.toml")
	legacy := filepath.Join(root, "ringmaster.toml")

	// Nothing yet: a new config goes to the user dir
	if got := GlobalConfigPath(); got != user {
		t.Errorf("fresh install: %s, want %s", got, user)
	}
	if moved, err := MigrateLegacyConfig(); moved != "" || err != nil {
		t.Errorf("nothing to migrate, got %q, %v", moved, err)
	}

	// Only the legacy config: it is used until migrated
	_ = os.WriteFile(legacy, []byte("[providers.x]\ntype = \"claude_subscription\"\n"), 0644)
	if got := GlobalConfigPath(); got != legacy {
		t.Errorf("legacy only: %s, want %s", got, legacy)
	}
	moved, err := MigrateLegacyConfig()
	if err != nil || moved != user {
		t.Fatalf("migrate: %q, %v", moved, err)
	}
	if fi, err := os.Stat(user); err != nil || fi.Mode().Perm() != 0600 {
		t.Errorf("migrated config should be private (0600): %v %v", fi, err)
	}
	if got := GlobalConfigPath(); got != user {
		t.Errorf("after migration: %s, want %s", got, user)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("the legacy file should be left in place")
	}

	// Migration happens once: a later legacy edit does not overwrite the user config
	_ = os.WriteFile(legacy, []byte("changed"), 0644)
	if moved, _ := MigrateLegacyConfig(); moved != "" {
		t.Errorf("second migration should be a no-op")
	}
	if data, _ := os.ReadFile(user); string(data) == "changed" {
		t.Errorf("user config was overwritten")
	}
}

// On a fresh install the user config dir does not exist yet; saving creates
// it, and the file is private since it can hold API keys.
func TestSaveFileCreatesPrivateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "ringmaster", "ringmaster.toml")
	if err := SaveFile(path, DefaultConfig()); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("config mode = %v, want 0600", fi.Mode().Perm())
	}
}

// A project's pass_env adds to the fleet's instead of replacing it, and the
// fleet config is left as it was.
func TestProjectPassEnvAddsToFleet(t *testing.T) {
	fleet := DefaultConfig()
	fleet.Sandbox.PassEnv = []string{"AWS_*"}

	merged := fleet.ForProject(&ProjectManifest{Sandbox: SandboxConfig{PassEnv: []string{"DATABASE_URL"}}})
	if got := merged.Sandbox.PassEnv; len(got) != 2 || got[0] != "AWS_*" || got[1] != "DATABASE_URL" {
		t.Errorf("merged pass_env = %v, want [AWS_* DATABASE_URL]", got)
	}
	if got := fleet.Sandbox.PassEnv; len(got) != 1 {
		t.Errorf("fleet pass_env was changed by the project: %v", got)
	}

	if got := fleet.ForProject(&ProjectManifest{}).Sandbox.PassEnv; len(got) != 1 || got[0] != "AWS_*" {
		t.Errorf("pass_env without a project override = %v, want [AWS_*]", got)
	}
}

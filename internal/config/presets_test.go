package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEffectiveSandbox(t *testing.T) {
	cfg := DefaultConfig()

	// No presets: the old built-in list, as before
	sb := cfg.EffectiveSandbox()
	if !reflect.DeepEqual(sb.Allow, DefaultAllowedCommands) {
		t.Errorf("without presets allow = %v, want the built-in list", sb.Allow)
	}
	if !hasEntry(sb.AlwaysAsk, "git push") || !hasEntry(sb.AlwaysAsk, "rm") {
		t.Errorf("the default always-ask list is missing: %v", sb.AlwaysAsk)
	}

	// Presets replace the built-in list
	cfg.Sandbox.Presets = []string{"go", "docker"}
	sb = cfg.EffectiveSandbox()
	for _, want := range []string{"git status", "go build", "go test", "gofmt", "docker build", "docker compose"} {
		if !hasEntry(sb.Allow, want) {
			t.Errorf("allow lacks %q: %v", want, sb.Allow)
		}
	}
	for _, unwanted := range []string{"npm", "pytest"} {
		if hasEntry(sb.Allow, unwanted) {
			t.Errorf("a Go and Docker project got %q", unwanted)
		}
	}
	for _, want := range []string{"docker compose down", "go clean", "git push"} {
		if !hasEntry(sb.AlwaysAsk, want) {
			t.Errorf("always_ask lacks %q: %v", want, sb.AlwaysAsk)
		}
	}

	// An explicit allow list is kept, and presets add to it
	cfg.Sandbox.Allow = []string{"make test"}
	sb = cfg.EffectiveSandbox()
	if !hasEntry(sb.Allow, "make test") || !hasEntry(sb.Allow, "go build") || hasEntry(sb.Allow, "git status") {
		t.Errorf("explicit allow with presets = %v", sb.Allow)
	}

	// The fleet's preset of the same name wins over the built-in one
	cfg.Presets = map[string]PresetConfig{"go": {Allow: []string{"go test"}}}
	sb = cfg.EffectiveSandbox()
	if hasEntry(sb.Allow, "go build") || !hasEntry(sb.Allow, "go test") {
		t.Errorf("fleet preset did not replace the built-in one: %v", sb.Allow)
	}

	// Unknown names are ignored here and reported by UnknownPresets
	cfg.Sandbox.Presets = []string{"go", "cobol"}
	_ = cfg.EffectiveSandbox()
	if got := cfg.UnknownPresets(cfg.Sandbox.Presets); !reflect.DeepEqual(got, []string{"cobol"}) {
		t.Errorf("unknown = %v", got)
	}
}

func TestProjectPresetsAddToTheFleets(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Sandbox.Presets = []string{"docker"}
	cfg.Sandbox.AlwaysAsk = []string{"make deploy"}
	out := cfg.ForProject(&ProjectManifest{Sandbox: SandboxConfig{Presets: []string{"go", "docker"}, AlwaysAsk: []string{"make release"}}})
	if !reflect.DeepEqual(out.Sandbox.Presets, []string{"docker", "go"}) {
		t.Errorf("presets = %v", out.Sandbox.Presets)
	}
	if !reflect.DeepEqual(out.Sandbox.AlwaysAsk, []string{"make deploy", "make release"}) {
		t.Errorf("always_ask = %v", out.Sandbox.AlwaysAsk)
	}
	if !reflect.DeepEqual(cfg.Sandbox.Presets, []string{"docker"}) {
		t.Error("the fleet config was changed")
	}
}

func TestAddProjectPresets(t *testing.T) {
	cases := []struct {
		name, before, want string
	}{
		{
			"no file",
			"",
			"[sandbox]\npresets = [\"go\", \"docker\"]\n",
		},
		{
			"no sandbox table; comments kept",
			"# my project\nname = \"shorty\"\n\n[web]\nport = 4000\n",
			"# my project\nname = \"shorty\"\n\n[web]\nport = 4000\n\n[sandbox]\npresets = [\"go\", \"docker\"]\n",
		},
		{
			"sandbox table without presets",
			"name = \"shorty\"\n\n[sandbox]\nallow = [\"make test\"]\n\n[web]\nport = 4000\n",
			"name = \"shorty\"\n\n[sandbox]\npresets = [\"go\", \"docker\"]\nallow = [\"make test\"]\n\n[web]\nport = 4000\n",
		},
		{
			"presets already there: only the new ones are added",
			"[sandbox]\npresets = [\"go\"]  # chosen by hand\npass_env = [\"DATABASE_URL\"]\n",
			"[sandbox]\npresets = [\"go\", \"docker\"]\npass_env = [\"DATABASE_URL\"]\n",
		},
		{
			"presets over several lines",
			"[sandbox]\npresets = [\n  \"go\",\n]\nallow = [\"x\"]\n",
			"[sandbox]\npresets = [\"go\", \"docker\"]\nallow = [\"x\"]\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if c.before != "" {
				if err := os.WriteFile(filepath.Join(dir, "ringmaster.toml"), []byte(c.before), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := AddProjectPresets(dir, []string{"go", "Docker", "go", "bad name!"}); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(filepath.Join(dir, "ringmaster.toml"))
			if string(got) != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
			m, err := LoadProjectManifest(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !hasEntry(m.Sandbox.Presets, "go") || !hasEntry(m.Sandbox.Presets, "docker") {
				t.Errorf("read back presets = %v", m.Sandbox.Presets)
			}
		})
	}

	// Nothing new: the file is not touched
	dir := t.TempDir()
	body := "[sandbox]\npresets = [\"go\"]\n"
	_ = os.WriteFile(filepath.Join(dir, "ringmaster.toml"), []byte(body), 0644)
	added, err := AddProjectPresets(dir, []string{"go"})
	if err != nil || added != nil {
		t.Fatalf("added = %v, err = %v", added, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "ringmaster.toml")); string(got) != body {
		t.Errorf("file changed:\n%s", got)
	}
}

func hasEntry(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

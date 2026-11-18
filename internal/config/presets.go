package config

import (
	"sort"
	"strings"
)

// PresetConfig is a named package of commands for one language or tool:
// what automatons may run without asking, and what must still be asked about
// although it starts like an allowed command (npm publish under npm).
type PresetConfig struct {
	Allow     []string `toml:"allow"`
	AlwaysAsk []string `toml:"always_ask,omitempty"`
}

// BuiltinPresets ship with Ringmaster. A preset of the same name in the
// fleet config replaces the built-in one.
var BuiltinPresets = map[string]PresetConfig{
	"go": {
		Allow:     []string{"go build", "go test", "go vet", "go mod", "gofmt"},
		AlwaysAsk: []string{"go clean"},
	},
	"python": {
		Allow:     []string{"python", "python3", "pytest", "ruff", "mypy"},
		AlwaysAsk: []string{"pip uninstall"},
	},
	"node": {
		Allow:     []string{"node", "npm", "npx"},
		AlwaysAsk: []string{"npm publish", "npm unpublish", "npm deprecate", "npm login", "npm adduser", "npm owner", "npm access"},
	},
	"rust": {
		Allow:     []string{"cargo build", "cargo test", "cargo check", "cargo clippy", "cargo fmt"},
		AlwaysAsk: []string{"cargo publish", "cargo yank", "cargo login"},
	},
	"docker": {
		Allow: []string{"docker build", "docker compose"},
		AlwaysAsk: []string{
			"docker compose down", "docker compose rm", "docker compose push",
			"docker push", "docker system prune", "docker volume rm", "docker volume prune",
			"docker image rm", "docker image prune", "docker rm", "docker rmi",
		},
	},
}

// DefaultAlwaysAsk applies to every project: commands that reach outside the
// machine, discard work or delete files. They are asked about even when an
// allowed entry (a bare "git", say) would cover them.
var DefaultAlwaysAsk = []string{
	"git push", "git reset --hard", "git clean", "git checkout --", "git restore",
	"git branch -D", "git rebase", "git filter-branch",
	"rm", "rmdir", "sudo", "chmod", "chown",
	"curl", "wget", "ssh", "scp", "rsync",
}

// baseWithPresets is what a project that names presets starts from: reading
// the repository is always fine.
var baseWithPresets = []string{"git status", "git diff", "git log"}

// Preset returns the preset of that name: the fleet's own, else the
// built-in one.
func (c *Config) Preset(name string) (PresetConfig, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if p, ok := c.Presets[name]; ok {
		return p, true
	}
	p, ok := BuiltinPresets[name]
	return p, ok
}

// PresetNames lists every preset that can be used, built-in and the fleet's.
func (c *Config) PresetNames() []string {
	seen := map[string]bool{}
	var names []string
	for n := range BuiltinPresets {
		seen[n] = true
		names = append(names, n)
	}
	for n := range c.Presets {
		if !seen[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// EffectiveSandbox is the sandbox the project's tasks run in: the commands
// allowed explicitly, plus those of its presets, and the always-ask list
// made of the defaults, the configured entries and the presets' own.
//
// The old built-in allowlist (pytest, npm, go test, ...) only applies to a
// project without presets. Once presets say what the project uses, they
// replace it, so a Go project does not also get npm.
func (c *Config) EffectiveSandbox() SandboxConfig {
	sb := c.Sandbox
	var presets []PresetConfig
	for _, name := range sb.Presets {
		if p, ok := c.Preset(name); ok {
			presets = append(presets, p)
		}
	}

	base := sb.Allow
	if len(base) == 0 || (len(presets) > 0 && sameSet(base, DefaultAllowedCommands)) {
		if len(presets) > 0 {
			base = baseWithPresets
		} else {
			base = DefaultAllowedCommands
		}
	}
	allow := append([]string{}, base...)
	ask := append(append([]string{}, DefaultAlwaysAsk...), sb.AlwaysAsk...)
	for _, p := range presets {
		allow = append(allow, p.Allow...)
		ask = append(ask, p.AlwaysAsk...)
	}
	sb.Allow = dedupe(allow)
	sb.AlwaysAsk = dedupe(ask)
	return sb
}

// UnknownPresets lists the names in names that are no preset.
func (c *Config) UnknownPresets(names []string) []string {
	var unknown []string
	for _, n := range names {
		if _, ok := c.Preset(n); !ok {
			unknown = append(unknown, n)
		}
	}
	return unknown
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	in := map[string]bool{}
	for _, s := range a {
		in[s] = true
	}
	for _, s := range b {
		if !in[s] {
			return false
		}
	}
	return true
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, s := range list {
		s = strings.Join(strings.Fields(s), " ")
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

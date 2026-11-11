package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"ringmaster/internal/skills"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(t *testing.T, m ConfigModel, keys ...string) ConfigModel {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(ConfigModel)
	}
	return m
}

func skillsModel(t *testing.T) (ConfigModel, string) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Providers["p"] = config.ProviderConfig{Type: "openai", BaseURL: "http://localhost:1"}
	cfg.Agents["a"] = config.AgentConfig{Provider: "p", Model: "m", Role: "worker"}
	path := filepath.Join(t.TempDir(), "ringmaster.toml")
	m := NewConfigModel(cfg, path).WithSkills([]skills.Skill{
		{Name: "deploy", Description: "Deploy steps", Source: skills.SourceProject},
		{Name: "go-style", Description: "House rules for Go code", Source: skills.SourceGlobal},
	}, []string{"deploy"})
	return m, path
}

// Tab walks Providers → Agents → Skills and back; 3 jumps to Skills.
func TestSkillsTabNavigation(t *testing.T) {
	m, _ := skillsModel(t)
	if m = press(t, m, "tab", "tab"); m.activeTab != tabSkills {
		t.Fatalf("two tabs from Providers: tab %d", m.activeTab)
	}
	if m = press(t, m, "tab"); m.activeTab != tabProviders {
		t.Errorf("tab from Skills: tab %d", m.activeTab)
	}
	if m = press(t, m, "3"); m.activeTab != tabSkills {
		t.Errorf("key 3: tab %d", m.activeTab)
	}

	view := m.View()
	for _, want := range []string{"[3] Skills", "go-style", "House rules for Go code", "[x] on", "off (proj)"} {
		if !strings.Contains(view, want) {
			t.Errorf("skills tab lacks %q:\n%s", want, view)
		}
	}
	if m = press(t, m, "down", "down", "down"); m.cursor != 1 {
		t.Errorf("cursor ran past the last skill: %d", m.cursor)
	}
}

// Space or Enter flips the highlighted skill and saves the fleet config.
func TestSkillsTabToggleSaves(t *testing.T) {
	m, path := skillsModel(t)
	m = press(t, m, "3", "down", " ")

	if !skills.IsDisabled("go-style", m.cfg.Skills.Disabled) {
		t.Fatalf("go-style not disabled: %v", m.cfg.Skills.Disabled)
	}
	saved, err := config.LoadFile(path)
	if err != nil || strings.Join(saved.Skills.Disabled, ",") != "go-style" {
		t.Fatalf("not saved: %+v, err %v", saved, err)
	}
	if !strings.Contains(m.View(), "[ ] off") {
		t.Errorf("view does not show the skill as off:\n%s", m.View())
	}

	m = press(t, m, "enter")
	saved, _ = config.LoadFile(path)
	if len(saved.Skills.Disabled) != 0 || skills.IsDisabled("go-style", m.cfg.Skills.Disabled) {
		t.Errorf("enter did not turn it back on: %v", saved.Skills.Disabled)
	}
}

// Keys that add, delete or re-role providers and agents do nothing on the Skills tab.
func TestSkillsTabIgnoresOtherActions(t *testing.T) {
	m, _ := skillsModel(t)
	m = press(t, m, "3", "a", "d", "x", "m", "w", "p")
	if m.mode != modeList || m.activeTab != tabSkills {
		t.Errorf("left the skills list: mode %d, tab %d", m.mode, m.activeTab)
	}
	if len(m.cfg.Agents) != 1 || len(m.cfg.Providers) != 1 || m.cfg.Agents["a"].Role != "worker" {
		t.Errorf("config changed: %+v", m.cfg)
	}
	// With no skills installed, toggling is a no-op and the tab explains itself
	empty := press(t, NewConfigModel(config.DefaultConfig(), filepath.Join(t.TempDir(), "c.toml")), "3", " ", "enter")
	if !strings.Contains(empty.View(), "No skills installed yet") {
		t.Errorf("empty skills tab:\n%s", empty.View())
	}
}

package tui

import (
	"fmt"
	"reflect"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
)

func TestDeterministicAgentOrdering(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents["coder-sonnet"] = config.AgentConfig{Provider: "p", Model: "sonnet", Role: "worker"}
	cfg.Agents["coder-haiku"] = config.AgentConfig{Provider: "p", Model: "haiku", Role: "worker"}
	cfg.Agents["coder-opus"] = config.AgentConfig{Provider: "p", Model: "opus", Role: "worker"}
	cfg.Agents["master"] = config.AgentConfig{Provider: "p", Model: "sonnet-3-7", Role: "master"}

	model := NewConfigModel(cfg, "ringmaster.toml")

	firstList := model.getAgentAliases()
	if len(firstList) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(firstList))
	}

	// Verify master is ALWAYS at index 0!
	if firstList[0] != "master" {
		t.Errorf("expected master to be at index 0, got %s", firstList[0])
	}

	// Run 100 iterations - order must be 100% frozen and stable!
	for i := 0; i < 100; i++ {
		iterList := model.getAgentAliases()
		if !reflect.DeepEqual(firstList, iterList) {
			t.Fatalf("iteration %d had different order: %v vs %v", i, iterList, firstList)
		}
	}

	fmt.Println("--- Deterministic Agent Ordering Test Passed (100 iterations perfectly stable)! ---")
}

func TestNavigationNoLoopAndLeftRight(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents["agent1"] = config.AgentConfig{Provider: "p", Model: "m1", Role: "worker"}
	cfg.Agents["agent2"] = config.AgentConfig{Provider: "p", Model: "m2", Role: "worker"}
	cfg.Agents["agent3"] = config.AgentConfig{Provider: "p", Model: "m3", Role: "worker"}

	model := NewConfigModel(cfg, "ringmaster.toml")
	m, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	if m.(ConfigModel).cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.(ConfigModel).cursor)
	}

	m, _ = m.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.(ConfigModel).cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.(ConfigModel).cursor)
	}

	m, _ = m.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.(ConfigModel).cursor != 2 {
		t.Errorf("expected cursor 2 (bottom), got %d", m.(ConfigModel).cursor)
	}

	m, _ = m.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.(ConfigModel).cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.(ConfigModel).cursor)
	}

	m, _ = m.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.(ConfigModel).cursor != 0 {
		t.Errorf("expected cursor 0 (top), got %d", m.(ConfigModel).cursor)
	}
}

func TestDeleteProviderConfirmation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["test_prov"] = config.ProviderConfig{Type: "openai", BaseURL: "http://localhost:8080"}
	cfg.Agents["agent1"] = config.AgentConfig{Provider: "test_prov", Model: "gpt-4", Role: "master"}
	cfg.Agents["agent2"] = config.AgentConfig{Provider: "test_prov", Model: "gpt-3.5", Role: "worker"}

	model := NewConfigModel(cfg, "ringmaster.toml")

	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	v1 := m1.(ConfigModel).View()

	if !strings.Contains(v1, "CONFIRM PROVIDER DELETION") {
		t.Errorf("expected CONFIRM PROVIDER DELETION prompt, got:\n%s", v1)
	}
	if !strings.Contains(v1, "WARNING: Deleting this provider will also delete 2 dependent agent(s)") {
		t.Errorf("expected warning about 2 dependent agents, got:\n%s", v1)
	}

	mCancel, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(mCancel.(ConfigModel).cfg.Providers) != 1 {
		t.Errorf("expected provider not deleted on cancel")
	}

	m2, _ := mCancel.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mConfirm, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if len(mConfirm.(ConfigModel).cfg.Providers) != 0 {
		t.Errorf("expected provider to be deleted")
	}
	if len(mConfirm.(ConfigModel).cfg.Agents) != 0 {
		t.Errorf("expected all 2 bound agents to be deleted as well, got: %d", len(mConfirm.(ConfigModel).cfg.Agents))
	}
}

func TestClaudeOAuthURLView(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2_oauth, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m3, _ := m2_oauth.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v3 := m3.(ConfigModel).View()

	if !strings.Contains(v3, "Claude Subscription Authorization") {
		t.Errorf("expected Claude Subscription Authorization view, got:\n%s", v3)
	}
	if !strings.Contains(v3, "https://claude.com/cai/oauth/authorize") {
		t.Errorf("expected generated authorize URL, got:\n%s", v3)
	}
}

func TestClaudeExistingSessionAndVersionedModels(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2_down1, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m2_down2, _ := m2_down1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m3, _ := m2_down2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})

	m4, _ := m3.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})
	m5, _ := m4.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m6, _ := m5.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})

	m7, _ := m6.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v7 := m7.(ConfigModel).View()

	if !strings.Contains(v7, "coder-sonnet") || !strings.Contains(v7, "master") {
		t.Errorf("expected agents in list view, got:\n%s", v7)
	}

	m8, _ := m7.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	v8 := m8.(ConfigModel).View()

	if !strings.Contains(v8, "is now the MASTER Planner") {
		t.Errorf("expected master change notification, got:\n%s", v8)
	}
}

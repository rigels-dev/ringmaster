package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
)

func TestDeleteProviderConfirmation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["test_prov"] = config.ProviderConfig{Type: "openai", BaseURL: "http://localhost:8080"}
	cfg.Agents["agent1"] = config.AgentConfig{Provider: "test_prov", Model: "gpt-4", Role: "master"}
	cfg.Agents["agent2"] = config.AgentConfig{Provider: "test_prov", Model: "gpt-3.5", Role: "worker"}

	model := NewConfigModel(cfg, "ringmaster.toml")

	// Press 'd' to trigger delete confirmation
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	v1 := m1.(ConfigModel).View()

	if !strings.Contains(v1, "CONFIRM PROVIDER DELETION") {
		t.Errorf("expected CONFIRM PROVIDER DELETION prompt, got:\n%s", v1)
	}
	if !strings.Contains(v1, "WARNING: Deleting this provider will also delete 2 dependent agent(s)") {
		t.Errorf("expected warning about 2 dependent agents, got:\n%s", v1)
	}

	fmt.Println("--- Delete Confirmation Prompt View: ---")
	fmt.Println(v1)

	// Press 'n' -> cancel
	mCancel, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(mCancel.(ConfigModel).cfg.Providers) != 1 {
		t.Errorf("expected provider not deleted on cancel")
	}

	// Press 'd' again and press 'y' -> confirm delete
	m2, _ := mCancel.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	mConfirm, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if len(mConfirm.(ConfigModel).cfg.Providers) != 0 {
		t.Errorf("expected provider to be deleted")
	}
	if len(mConfirm.(ConfigModel).cfg.Agents) != 0 {
		t.Errorf("expected all 2 bound agents to be deleted as well, got: %d", len(mConfirm.(ConfigModel).cfg.Agents))
	}

	fmt.Println("--- Delete Confirmed: Provider and Bound Agents Successfully Deleted! ---")
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

	fmt.Println("--- Claude OAuth View Test Passed! ---")
}

func TestClaudeExistingSessionAndVersionedModels(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2_down1, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m2_down2, _ := m2_down1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m3, _ := m2_down2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Check models with Space
	m4, _ := m3.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})
	m5, _ := m4.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m6, _ := m5.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})

	// Press Enter to confirm and add to agent list -> returns directly to modeList!
	m7, _ := m6.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v7 := m7.(ConfigModel).View()

	if !strings.Contains(v7, "coder-sonnet") || !strings.Contains(v7, "master") {
		t.Errorf("expected agents in list view, got:\n%s", v7)
	}

	// Press 'm' to set highlighted agent as Master!
	m8, _ := m7.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	v8 := m8.(ConfigModel).View()

	if !strings.Contains(v8, "is now the MASTER Planner") {
		t.Errorf("expected master change notification, got:\n%s", v8)
	}

	fmt.Println("--- Final Screen After Pressing [m] to Set Master: ---")
}

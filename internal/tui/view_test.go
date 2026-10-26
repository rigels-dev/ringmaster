package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
)

func TestClaudeOAuthURLView(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	// Step 1: press 'a'
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// Step 2: press Enter on Anthropic
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Step 3: press Enter on Claude Subscription (Login via Browser URL & Authorized Code)
	m3, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v3 := m3.(ConfigModel).View()

	if !strings.Contains(v3, "Claude Subscription Authorization") {
		t.Errorf("expected Claude Subscription Authorization view, got:\n%s", v3)
	}
	if !strings.Contains(v3, "https://claude.ai/oauth/authorize") {
		t.Errorf("expected generated authorize URL, got:\n%s", v3)
	}
	if !strings.Contains(v3, "Auth Code:") {
		t.Errorf("expected Auth Code input, got:\n%s", v3)
	}

	fmt.Println("--- Claude OAuth View Test Passed! ---")
	fmt.Println(v3)
}

func TestClaudeExistingSessionAndVersionedModels(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	// Step 1: press 'a'
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// Step 2: press Enter on Anthropic
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Move down to option 1 (Reuse existing local ~/.claude session)
	m2_down, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	// Press Enter -> directly lists models with FULL VERSIONS!
	m3, _ := m2_down.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v3 := m3.(ConfigModel).View()

	// Verify full versions are listed!
	if !strings.Contains(v3, "claude-3-7-sonnet-20250219") {
		t.Errorf("expected full version claude-3-7-sonnet-20250219, got:\n%s", v3)
	}
	if !strings.Contains(v3, "claude-3-5-sonnet-20241022") {
		t.Errorf("expected full version claude-3-5-sonnet-20241022, got:\n%s", v3)
	}
	if !strings.Contains(v3, "claude-3-5-haiku-20241022") {
		t.Errorf("expected full version claude-3-5-haiku-20241022, got:\n%s", v3)
	}

	// Check models with Space
	m4, _ := m3.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace}) // check claude-3-7-sonnet
	m5, _ := m4.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m6, _ := m5.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace}) // check claude-3-5-sonnet
	v6 := m6.(ConfigModel).View()

	if !strings.Contains(v6, "[x] claude-3-7-sonnet-20250219") {
		t.Errorf("expected claude-3-7-sonnet-20250219 to be checked [x], got:\n%s", v6)
	}

	fmt.Println("--- Model Checklist with Full Versions Passed! ---")
	fmt.Println(v6)

	// Press Enter to confirm and add to agent list
	m7, _ := m6.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v7 := m7.(ConfigModel).View()

	fmt.Println("--- Final Agent Roster with Versioned Models: ---")
	fmt.Println(v7)
}

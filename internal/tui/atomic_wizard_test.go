package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"ringmaster/internal/config"
	"testing"
)

func TestAtomicWizardProviderDiscardOnEsc(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, filepath.Join(t.TempDir(), "ringmaster.toml"))

	// Step 1: Add provider -> select Anthropic
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// Step 2: Choose Claude CLI / Existing session
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2_down1, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m3, _ := m2_down1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Now in Step 3 (Pick Models)
	if m3.(ConfigModel).mode != modePickModel {
		t.Fatalf("expected modePickModel, got %v", m3.(ConfigModel).mode)
	}

	// At this point, provider must NOT be committed to cfg.Providers yet!
	if _, exists := m3.(ConfigModel).cfg.Providers["claude_subscription"]; exists {
		t.Fatalf("provider should be staged in pendingProviderConfig, not in cfg.Providers yet!")
	}

	// Hit ESC to cancel
	mEsc, _ := m3.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEscape})

	// Must be back to modeList, and provider was NEVER added!
	if mEsc.(ConfigModel).mode != modeList {
		t.Errorf("expected modeList after Esc, got %v", mEsc.(ConfigModel).mode)
	}
	if _, exists := mEsc.(ConfigModel).cfg.Providers["claude_subscription"]; exists {
		t.Errorf("provider should not exist in cfg.Providers after Esc cancel!")
	}
	if len(mEsc.(ConfigModel).cfg.Agents) != 0 {
		t.Errorf("no agents should exist after Esc cancel!")
	}
}

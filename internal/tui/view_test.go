package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
)

func TestWizardFullFlow(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	// Step 1: press 'a'
	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	v1 := m1.(ConfigModel).View()
	if !strings.Contains(v1, "Model/auth provider") {
		t.Errorf("Step 1 missing Model/auth provider header")
	}

	// Step 2: press Enter on Anthropic
	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v2 := m2.(ConfigModel).View()
	if !strings.Contains(v2, "Anthropic auth method") {
		t.Errorf("Step 2 missing Anthropic auth method header")
	}

	// Step 3: press Enter on Anthropic Claude CLI -> Should fetch models & show Step 3
	m3, _ := m2.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v3 := m3.(ConfigModel).View()
	if !strings.Contains(v3, "Select models") {
		t.Errorf("Step 3 missing models selection header, got:\n%s", v3)
	}

	// Check sonnet with Space
	m4, _ := m3.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})
	v4 := m4.(ConfigModel).View()
	if !strings.Contains(v4, "[x] sonnet") {
		t.Errorf("expected sonnet to be checked [x], got:\n%s", v4)
	}

	// Move down and check haiku with Space
	m5, _ := m4.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	m6, _ := m5.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeySpace})
	v6 := m6.(ConfigModel).View()
	if !strings.Contains(v6, "[x] haiku") {
		t.Errorf("expected haiku to be checked [x], got:\n%s", v6)
	}

	fmt.Println("--- V6 (With [x] Checkboxes Checked): ---")
	fmt.Println(v6)

	// Confirm with Enter -> Puts checked models on agent list!
	m7, _ := m6.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v7 := m7.(ConfigModel).View()

	fmt.Println("--- V7 (Final Agents Tab with Added Models): ---")
	fmt.Println(v7)

	cfgFinal := m7.(ConfigModel).cfg
	if len(cfgFinal.Agents) < 2 {
		t.Errorf("expected at least 2 agents added, got %d", len(cfgFinal.Agents))
	}
}

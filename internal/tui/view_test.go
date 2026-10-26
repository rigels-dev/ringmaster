package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
)

func TestViewOutputStep1And2(t *testing.T) {
	cfg := config.DefaultConfig()
	model := NewConfigModel(cfg, "ringmaster.toml")

	m1, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	v1 := m1.(ConfigModel).View()
	if !strings.Contains(v1, "Model/auth provider") {
		t.Errorf("Step 1 missing Model/auth provider header")
	}
	fmt.Println("--- V1 (Step 1 output): ---")
	fmt.Println(v1)

	m2, _ := m1.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	v2 := m2.(ConfigModel).View()
	if !strings.Contains(v2, "Anthropic auth method") {
		t.Errorf("Step 2 missing Anthropic auth method header")
	}

	fmt.Println("--- V2 (Step 2 output): ---")
	fmt.Println(v2)
}

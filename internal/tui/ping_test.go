package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"strings"
	"testing"
	"time"
)

func TestPingKeyHandling(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["test_prov"] = config.ProviderConfig{Type: "openai", BaseURL: "http://localhost:8080"}
	cfg.Agents["tester"] = config.AgentConfig{Provider: "test_prov", Model: "gpt-4o", Role: "worker"}

	model := NewConfigModel(cfg, "ringmaster.toml")

	// Switch to Agents tab
	mAgents, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	// Press 'p' to ping
	mPing, cmd := mAgents.(ConfigModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatalf("expected ping command to be triggered")
	}

	if !strings.Contains(mPing.(ConfigModel).statusMsg, "Pinging agent 'tester'") {
		t.Errorf("expected statusMsg to show pinging, got: %s", mPing.(ConfigModel).statusMsg)
	}

	// Simulate pingResultMsg success
	resMsg := pingResultMsg{
		target:  "tester",
		latency: 150 * time.Millisecond,
		tokens:  14,
		err:     nil,
	}

	mDone, _ := mPing.(ConfigModel).Update(resMsg)
	vDone := mDone.(ConfigModel).View()

	if !strings.Contains(vDone, "Ping succeeded for 'tester' in 150ms! (Tokens: 14)") {
		t.Errorf("expected ping success message in view, got:\n%s", vDone)
	}
}

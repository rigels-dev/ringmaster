package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/auth"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
)

// updatePickProvider is wizard step 1: search and choose a provider.
func (m ConfigModel) updatePickProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.getFilteredProviders()
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "up":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
		return m, nil
	case "down":
		if m.providerCursor < len(filtered)-1 {
			m.providerCursor++
		}
		return m, nil
	case "left", "home":
		m.providerCursor = 0
		return m, nil
	case "right", "end":
		if len(filtered) > 0 {
			m.providerCursor = len(filtered) - 1
		}
		return m, nil
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.providerCursor = 0
		}
		return m, nil
	case "enter":
		if len(filtered) > 0 && m.providerCursor < len(filtered) {
			prov := filtered[m.providerCursor]
			m.selectedProvider = &prov
			m.authMethodCursor = 0
			m.mode = modePickProviderStep2
		}
		return m, nil
	default:
		// Type to search
		if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
			m.searchQuery += msg.String()
			m.providerCursor = 0
			return m, nil
		}
	}

	return m, nil
}

// updatePickAuthMethod is wizard step 2: choose how to authenticate with the selected provider.
func (m ConfigModel) updatePickAuthMethod(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.selectedProvider == nil {
		m.mode = modePickProviderStep1
		return m, nil
	}

	// Options include methods + "Back"
	totalOptions := len(m.selectedProvider.AuthMethods) + 1 // last is Back
	switch msg.String() {
	case "esc":
		m.mode = modePickProviderStep1
		return m, nil
	case "up", "k":
		if m.authMethodCursor > 0 {
			m.authMethodCursor--
		}
		return m, nil
	case "down", "j":
		if m.authMethodCursor < totalOptions-1 {
			m.authMethodCursor++
		}
		return m, nil
	case "enter":
		if m.authMethodCursor == totalOptions-1 {
			// Back
			m.mode = modePickProviderStep1
			return m, nil
		}

		method := m.selectedProvider.AuthMethods[m.authMethodCursor]
		p := config.ProviderConfig{
			Type:      method.Type,
			BaseURL:   method.BaseURL,
			APIKeyEnv: method.DefaultEnv,
		}

		providerKey := m.selectedProvider.ID
		if method.Type == "claude_subscription" {
			providerKey = "claude_subscription"
		}

		if method.ID == "claude_login_cli" {
			cmd := exec.Command("claude", "auth", "login")
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return claudeLoginDoneMsg{err: err}
			})
		}

		if method.ID == "claude_oauth" {
			authURL, verifier, state, err := auth.GenerateAuthURL()
			if err != nil {
				m.statusMsg = fmt.Sprintf("Error generating auth URL: %v", err)
				return m, nil
			}
			m.oauthURL = authURL
			m.oauthVerifier = verifier
			m.oauthState = state

			// Save URL to file so user can always copy it easily
			if home, err := os.UserHomeDir(); err == nil {
				_ = os.MkdirAll(filepath.Join(home, ".claude"), 0700)
				_ = os.WriteFile(filepath.Join(home, ".claude", "auth_url.txt"), []byte(authURL), 0600)
			}

			ti := textinput.New()
			ti.Placeholder = "Paste authorization code or callback URL here..."
			ti.Prompt = "Auth Code: "
			ti.Width = 60
			ti.Focus()
			m.oauthInput = ti
			m.mode = modeClaudeOAuth
			return m, textinput.Blink
		}

		if method.NeedsInput {
			m.initProviderInputs(providerKey, p)
			m.mode = modeAddProvider
			return m, m.focusCurrentInput()
		}

		// Stage provider in pending state without saving to file yet
		m.pendingProviderConfig = &p
		m.selectedAuthMethod = &method
		m.configuredProviderKey = providerKey

		// Fetch models live
		client := llm.NewProvider(p.Type, p.BaseURL, p.APIKeyEnv)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		models, err := client.ListModels(ctx)
		cancel()

		if err == nil && len(models) > 0 {
			m.fetchedModels = models
			m.checkedModels = make(map[string]bool)
			m.modelCursor = 0
			m.modelSearchQuery = ""
			m.statusMsg = fmt.Sprintf("✓ Provider '%s' connected! Check models to put on your list.", providerKey)
			m.mode = modePickModel
			return m, nil
		}

		// If no models fetched or manual provider, commit now
		m.cfg.Providers[providerKey] = p
		_ = config.SaveFile(m.configPath, m.cfg)
		m.pendingProviderConfig = nil
		m.statusMsg = fmt.Sprintf("✓ Configured provider '%s' (%s) successfully!", providerKey, method.Label)
		m.mode = modeList
		return m, nil
	}

	return m, nil
}

// updateClaudeOAuth runs the interactive Claude OAuth flow (open the URL, paste the code).
func (m ConfigModel) updateClaudeOAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modePickProviderStep2
		return m, nil
	case "ctrl+c":
		// Copy URL using OSC 52
		fmt.Printf("\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(m.oauthURL)))
		m.statusMsg = "✓ Copied full authorization URL to clipboard!"
		return m, nil
	case "enter":
		code := strings.TrimSpace(m.oauthInput.Value())
		if code == "" {
			m.statusMsg = "Please enter or paste the authorization code!"
			return m, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		tokens, err := auth.ExchangeAuthCode(ctx, code, m.oauthVerifier, m.oauthState)
		cancel()

		if err != nil {
			m.statusMsg = fmt.Sprintf("Authorization exchange failed: %v", err)
			return m, nil
		}

		// Save credentials locally
		_ = auth.SaveClaudeCredentials(tokens)

		providerKey := "claude_subscription"
		p := config.ProviderConfig{
			Type: "claude_subscription",
		}
		m.pendingProviderConfig = &p
		m.configuredProviderKey = providerKey

		// Fetch models
		client := llm.NewProvider(p.Type, "", "")
		ctx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
		models, _ := client.ListModels(ctx2)
		cancel2()

		m.fetchedModels = models
		m.checkedModels = make(map[string]bool)
		m.modelCursor = 0
		m.modelSearchQuery = ""
		m.statusMsg = "✓ Claude Subscription authorized successfully! Check models to add to list."
		m.mode = modePickModel
		return m, nil
	default:
		var cmd tea.Cmd
		m.oauthInput, cmd = m.oauthInput.Update(msg)
		return m, cmd
	}
}

// updateConfirmDeleteProvider asks before removing a provider and the agents bound to it.
func (m ConfigModel) updateConfirmDeleteProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		provName := m.deletingProviderName
		if provName != "" {
			delete(m.cfg.Providers, provName)

			// Delete all agents bound to this provider as well
			deletedAgentsCount := 0
			for aName, a := range m.cfg.Agents {
				if a.Provider == provName {
					delete(m.cfg.Agents, aName)
					deletedAgentsCount++
				}
			}

			// Update routing defaults if they pointed to deleted agents
			if m.cfg.Routing.DefaultMaster != "" {
				if _, exists := m.cfg.Agents[m.cfg.Routing.DefaultMaster]; !exists {
					m.cfg.Routing.DefaultMaster = ""
				}
			}
			if m.cfg.Routing.DefaultWorker != "" {
				if _, exists := m.cfg.Agents[m.cfg.Routing.DefaultWorker]; !exists {
					m.cfg.Routing.DefaultWorker = ""
				}
			}

			_ = config.SaveFile(m.configPath, m.cfg)
			if deletedAgentsCount > 0 {
				m.statusMsg = fmt.Sprintf("✓ Deleted provider '%s' and %d dependent agent(s)", provName, deletedAgentsCount)
			} else {
				m.statusMsg = fmt.Sprintf("✓ Deleted provider '%s'", provName)
			}

			if m.cursor > 0 {
				m.cursor--
			}
		}
		m.deletingProviderName = ""
		m.mode = modeList
		return m, nil

	case "n", "esc", "q":
		m.deletingProviderName = ""
		m.statusMsg = "Deletion cancelled."
		m.mode = modeList
		return m, nil
	}

	return m, nil
}

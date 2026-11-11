package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/auth"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
)

type tab int

const (
	tabProviders tab = iota
	tabAgents
)

type viewMode int

const (
	modeList                  viewMode = iota
	modePickProviderStep1              // Step 1: Choose Provider
	modePickProviderStep2              // Step 2: Choose Auth Method
	modeClaudeOAuth                    // Interactive OAuth URL + Code paste
	modePickModel                      // Step 3: Pick Model from fetched list
	modePickModelRole                  // Step 4: Pick Role for chosen model
	modeConfirmDeleteProvider          // Step: Confirm provider deletion
	modeEditProvider
	modeAddProvider
	modePickAgentTemplate
	modeEditAgent
	modeAddAgent
)

type claudeLoginDoneMsg struct {
	err error
}

type pingResultMsg struct {
	target  string
	latency time.Duration
	err     error
	tokens  int
}

type ConfigModel struct {
	cfg                   *config.Config
	configPath            string
	activeTab             tab
	mode                  viewMode
	cursor                int
	providerCursor        int
	authMethodCursor      int
	templateCursor        int
	searchQuery           string
	selectedProvider      *ProviderOption
	selectedAuthMethod    *AuthMethodOption
	configuredProviderKey string
	pendingProviderConfig *config.ProviderConfig
	fetchedModels         []llm.ModelInfo
	checkedModels         map[string]bool // checkbox state per model ID
	modelCursor           int
	modelSearchQuery      string
	selectedModelID       string
	roleCursor            int
	deletingProviderName  string
	oauthURL              string
	oauthVerifier         string
	oauthState            string
	oauthInput            textinput.Model
	inputs                []textinput.Model
	statusMsg             string
	err                   error
	width                 int
	height                int
}

func NewConfigModel(cfg *config.Config, configPath string) ConfigModel {
	return ConfigModel{
		cfg:        cfg,
		configPath: configPath,
		activeTab:  tabProviders,
		mode:       modeList,
		cursor:     0,
	}
}

func (m ConfigModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConfigModel) getFilteredModels() []llm.ModelInfo {
	if strings.TrimSpace(m.modelSearchQuery) == "" {
		return m.fetchedModels
	}
	q := strings.ToLower(strings.TrimSpace(m.modelSearchQuery))
	var res []llm.ModelInfo
	for _, mod := range m.fetchedModels {
		if strings.Contains(strings.ToLower(mod.ID), q) || strings.Contains(strings.ToLower(mod.Desc), q) {
			res = append(res, mod)
		}
	}
	return res
}

func (m ConfigModel) getFilteredProviders() []ProviderOption {
	if strings.TrimSpace(m.searchQuery) == "" {
		return popularProviders
	}
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))
	var res []ProviderOption
	for _, p := range popularProviders {
		if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Description), q) {
			res = append(res, p)
		}
	}
	return res
}

func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case claudeLoginDoneMsg:
		status := auth.CheckClaudeCLIStatus()
		if !status.LoggedIn {
			m.statusMsg = "Claude login was not completed. Please try again."
			m.mode = modePickProviderStep2
			return m, nil
		}

		providerKey := "claude_subscription"
		p := config.ProviderConfig{
			Type: "claude_subscription",
		}
		m.pendingProviderConfig = &p
		m.configuredProviderKey = providerKey

		// Fetch models
		client := llm.NewProvider(p.Type, "", "")
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		models, _ := client.ListModels(ctx)
		cancel()

		m.fetchedModels = models
		m.checkedModels = make(map[string]bool)
		m.modelCursor = 0
		m.modelSearchQuery = ""
		m.statusMsg = fmt.Sprintf("✓ Logged in as %s (%s)! Check models to add to list.", status.Email, status.SubscriptionType)
		m.mode = modePickModel
		return m, nil

	case pingResultMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ Ping failed for '%s' (%s): %v", msg.target, msg.latency.Round(time.Millisecond), msg.err)
		} else {
			if msg.tokens > 0 {
				m.statusMsg = fmt.Sprintf("✓ Ping succeeded for '%s' in %s! (Tokens: %d)", msg.target, msg.latency.Round(time.Millisecond), msg.tokens)
			} else {
				m.statusMsg = fmt.Sprintf("✓ Ping succeeded for '%s' in %s!", msg.target, msg.latency.Round(time.Millisecond))
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modePickProviderStep1:
			return m.updatePickProvider(msg)
		case modePickProviderStep2:
			return m.updatePickAuthMethod(msg)
		case modeClaudeOAuth:
			return m.updateClaudeOAuth(msg)
		case modePickModel:
			return m.updatePickModel(msg)
		case modeConfirmDeleteProvider:
			return m.updateConfirmDeleteProvider(msg)
		case modePickModelRole:
			return m.updatePickModelRole(msg)
		case modePickAgentTemplate:
			return m.updatePickAgentTemplate(msg)
		case modeAddProvider, modeEditProvider, modeAddAgent, modeEditAgent:
			return m.updateForm(msg)
		}
	}

	return m, nil
}
func (m ConfigModel) getProviderNames() []string {
	var names []string
	for k := range m.cfg.Providers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func (m ConfigModel) getAgentAliases() []string {
	var aliases []string
	for k := range m.cfg.Agents {
		aliases = append(aliases, k)
	}
	sort.Strings(aliases)
	return aliases
}

func (m ConfigModel) currentListLen() int {
	if m.activeTab == tabProviders {
		return len(m.cfg.Providers)
	}
	return len(m.cfg.Agents)
}

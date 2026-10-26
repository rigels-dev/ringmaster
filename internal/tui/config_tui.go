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
	"github.com/charmbracelet/lipgloss"
	"ringmaster/internal/auth"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#29F2EC"})

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#0EA5E9"}).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#8493A0"}).
			Background(lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#1B2229"}).
			Padding(0, 2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#DEAA3B"}).
			Bold(true)

	stepDoneBullet = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#22C55E"}).
			Bold(true)

	stepActiveBullet = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}).
			Bold(true)

	stepTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#F8FAFC"}).
			Bold(true)

	stepDimTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}).
			Bold(true)

	stepAnswerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#29F2EC"}).
			Bold(true)

	selectedBulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#22C55E"}).
				Bold(true)

	normalBulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"})

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"})

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}).
			Italic(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"}).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#5BF28D"}).
			Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#E05436"}).
			Bold(true)
)
type tab int

const (
	tabProviders tab = iota
	tabAgents
)

type viewMode int

const (
	modeList viewMode = iota
	modePickProviderStep1 // Step 1: Choose Provider
	modePickProviderStep2 // Step 2: Choose Auth Method
	modeClaudeOAuth       // Interactive OAuth URL + Code paste
	modePickModel         // Step 3: Pick Model from fetched list
	modePickModelRole     // Step 4: Pick Role for chosen model
	modeEditProvider
	modeAddProvider
	modePickAgentTemplate
	modeEditAgent
	modeAddAgent
)

type claudeLoginDoneMsg struct {
	err error
}

type AuthMethodOption struct {
	ID          string
	Label       string
	Description string
	Type        string
	BaseURL     string
	DefaultEnv  string
	NeedsInput  bool
}

type ProviderOption struct {
	ID          string
	Name        string
	Description string
	AuthMethods []AuthMethodOption
}

var popularProviders = []ProviderOption{
	{
		ID:          "anthropic",
		Name:        "Anthropic",
		Description: "Claude 5.5 Sonnet, Opus, Haiku",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "claude_login_cli",
				Label:       "Launch Official Claude Login in Terminal (claude auth login)",
				Description: "Suspends TUI, opens official login in browser, then returns here",
				Type:        "claude_subscription",
				BaseURL:     "",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
			{
				ID:          "claude_oauth",
				Label:       "Manual Browser OAuth URL & Paste Code",
				Description: "Generates auth link to claude.com and exchanges code",
				Type:        "claude_subscription",
				BaseURL:     "",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
			{
				ID:          "claude_cli",
				Label:       "Reuse Existing Session (Already logged in on this host)",
				Description: "Checks local ~/.claude credentials and uses them directly",
				Type:        "claude_subscription",
				BaseURL:     "",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
			{
				ID:          "api_key",
				Label:       "Anthropic API key (ANTHROPIC_API_KEY)",
				Description: "Direct Anthropic Messages API (api.anthropic.com/v1)",
				Type:        "anthropic",
				BaseURL:     "https://api.anthropic.com/v1",
				DefaultEnv:  "ANTHROPIC_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "openai",
		Name:        "OpenAI",
		Description: "GPT-4o, GPT-4o-mini, o1",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "OpenAI API key",
				Description: "Official api.openai.com endpoint (OPENAI_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://api.openai.com/v1",
				DefaultEnv:  "OPENAI_API_KEY",
				NeedsInput:  false,
			},
			{
				ID:          "custom_url",
				Label:       "Custom OpenAI-compatible URL",
				Description: "Reverse proxy, gateway, or enterprise OpenAI deployment",
				Type:        "openai",
				BaseURL:     "https://api.openai.com/v1",
				DefaultEnv:  "OPENAI_API_KEY",
				NeedsInput:  true,
			},
		},
	},
	{
		ID:          "openrouter",
		Name:        "OpenRouter",
		Description: "Unified gateway to Claude, Llama, Gemini, etc.",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "OpenRouter API key",
				Description: "openrouter.ai/api/v1 (OPENROUTER_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://openrouter.ai/api/v1",
				DefaultEnv:  "OPENROUTER_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "ollama",
		Name:        "Ollama",
		Description: "Local LLMs running on localhost:11434",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "local",
				Label:       "Local Ollama instance",
				Description: "http://localhost:11434/v1 (No API key needed)",
				Type:        "openai",
				BaseURL:     "http://localhost:11434/v1",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "9router",
		Name:        "9router",
		Description: "Local multi-provider proxy on localhost:20128",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "local",
				Label:       "Local 9router instance",
				Description: "http://localhost:20128/v1 (NINEROUTER_API_KEY)",
				Type:        "openai",
				BaseURL:     "http://localhost:20128/v1",
				DefaultEnv:  "NINEROUTER_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "deepseek",
		Name:        "DeepSeek",
		Description: "DeepSeek-V3, DeepSeek-R1",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "DeepSeek API key",
				Description: "api.deepseek.com/v1 (DEEPSEEK_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://api.deepseek.com/v1",
				DefaultEnv:  "DEEPSEEK_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "custom",
		Name:        "Custom Provider",
		Description: "Custom self-hosted model or private gateway",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "custom_openai",
				Label:       "OpenAI-compatible Endpoint",
				Description: "Manually specify base URL and API key environment variable",
				Type:        "openai",
				BaseURL:     "http://localhost:8000/v1",
				DefaultEnv:  "",
				NeedsInput:  true,
			},
			{
				ID:          "custom_anthropic",
				Label:       "Anthropic-compatible Endpoint",
				Description: "Manually specify Anthropic /v1/messages URL and API key",
				Type:        "anthropic",
				BaseURL:     "https://api.anthropic.com/v1",
				DefaultEnv:  "",
				NeedsInput:  true,
			},
		},
	},
}

type AgentTemplate struct {
	Title       string
	Description string
	Alias       string
	Role        string
	Model       string
	ToolCalling bool
}

var agentTemplates = []AgentTemplate{
	{
		Title:       "Master Planner (Sonnet / Opus)",
		Description: "Control Plane Ringmaster agent that analyzes briefs and breaks tasks into DAG",
		Alias:       "master",
		Role:        "master",
		Model:       "sonnet",
		ToolCalling: false,
	},
	{
		Title:       "Worker: Fast Coder (Haiku / Mini)",
		Description: "Fast, token-efficient worker for scaffold and smaller edits",
		Alias:       "coder-fast",
		Role:        "worker",
		Model:       "haiku",
		ToolCalling: true,
	},
	{
		Title:       "Worker: Local / Cheap Coder (Qwen / DeepSeek)",
		Description: "Cheap or local worker executing code within isolated worktrees",
		Alias:       "coder-cheap",
		Role:        "worker",
		Model:       "qwen2.5-coder:7b",
		ToolCalling: true,
	},
	{
		Title:       "Custom Agent",
		Description: "Manually specify alias, provider binding, model name, and role",
		Alias:       "",
		Role:        "worker",
		Model:       "",
		ToolCalling: true,
	},
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
	fetchedModels         []llm.ModelInfo
	checkedModels         map[string]bool // checkbox state per model ID
	modelCursor           int
	modelSearchQuery      string
	selectedModelID       string
	roleCursor            int
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
		m.cfg.Providers[providerKey] = p
		_ = config.SaveFile(m.configPath, m.cfg)
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				if m.activeTab == tabProviders {
					m.activeTab = tabAgents
				} else {
					m.activeTab = tabProviders
				}
				m.cursor = 0
				m.statusMsg = ""
				return m, nil
			case "1":
				m.activeTab = tabProviders
				m.cursor = 0
				return m, nil
			case "2":
				m.activeTab = tabAgents
				m.cursor = 0
				return m, nil
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case "down", "j":
				maxLen := m.currentListLen()
				if m.cursor < maxLen-1 {
					m.cursor++
				}
				return m, nil
			case "a":
				m.statusMsg = ""
				if m.activeTab == tabProviders {
					m.mode = modePickProviderStep1
					m.providerCursor = 0
					m.searchQuery = ""
				} else {
					m.mode = modePickAgentTemplate
					m.templateCursor = 0
				}
				return m, nil
			case "enter", "e":
				maxLen := m.currentListLen()
				if maxLen == 0 {
					return m, nil
				}
				if m.activeTab == tabProviders {
					names := m.getProviderNames()
					if m.cursor < len(names) {
						name := names[m.cursor]
						m.initProviderInputs(name, m.cfg.Providers[name])
						m.mode = modeEditProvider
						return m, m.focusCurrentInput()
					}
				} else {
					aliases := m.getAgentAliases()
					if m.cursor < len(aliases) {
						alias := aliases[m.cursor]
						m.initAgentInputs(alias, m.cfg.Agents[alias])
						m.mode = modeEditAgent
						return m, m.focusCurrentInput()
					}
				}
			case "d", "x":
				if m.activeTab == tabProviders {
					names := m.getProviderNames()
					if m.cursor < len(names) {
						delete(m.cfg.Providers, names[m.cursor])
						_ = config.SaveFile(m.configPath, m.cfg)
						m.statusMsg = fmt.Sprintf("Deleted provider '%s'", names[m.cursor])
						if m.cursor > 0 {
							m.cursor--
						}
					}
				} else {
					aliases := m.getAgentAliases()
					if m.cursor < len(aliases) {
						delete(m.cfg.Agents, aliases[m.cursor])
						_ = config.SaveFile(m.configPath, m.cfg)
						m.statusMsg = fmt.Sprintf("Deleted agent '%s'", aliases[m.cursor])
						if m.cursor > 0 {
							m.cursor--
						}
					}
				}
			case "s":
				if err := config.SaveFile(m.configPath, m.cfg); err != nil {
					m.err = err
				} else {
					m.statusMsg = fmt.Sprintf("✓ Configuration successfully saved to %s", m.configPath)
				}
			}

		case modePickProviderStep1: // Step 1: Search & Choose Provider
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

		case modePickProviderStep2: // Step 2: Choose Auth Method for selected provider
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
						_ = os.WriteFile(filepath.Join(home, ".claude", "auth_url.txt"), []byte(authURL), 0644)
					}
					_ = os.WriteFile("/tmp/claude_auth_url.txt", []byte(authURL), 0644)

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

				// Direct auto-configure and save
				m.cfg.Providers[providerKey] = p
				_ = config.SaveFile(m.configPath, m.cfg)
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

				m.statusMsg = fmt.Sprintf("✓ Configured provider '%s' (%s) successfully!", providerKey, method.Label)
				m.mode = modeList
				return m, nil
			}

		case modeClaudeOAuth: // Interactive OAuth flow
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
				m.cfg.Providers[providerKey] = p
				_ = config.SaveFile(m.configPath, m.cfg)
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

		case modePickModel: // Step 3: Check / Select models to put on list
			filtered := m.getFilteredModels()
			totalItems := len(filtered)

			switch msg.String() {
			case "esc":
				m.mode = modeList
				return m, nil
			case "up":
				if m.modelCursor > 0 {
					m.modelCursor--
				}
				return m, nil
			case "down":
				if m.modelCursor < totalItems-1 {
					m.modelCursor++
				}
				return m, nil
			case " ": // Space: Toggle checkbox for highlighted model!
				if m.checkedModels == nil {
					m.checkedModels = make(map[string]bool)
				}
				if len(filtered) > 0 && m.modelCursor < len(filtered) {
					id := filtered[m.modelCursor].ID
					m.checkedModels[id] = !m.checkedModels[id]
				}
				return m, nil
			case "backspace":
				if len(m.modelSearchQuery) > 0 {
					m.modelSearchQuery = m.modelSearchQuery[:len(m.modelSearchQuery)-1]
					m.modelCursor = 0
				}
				return m, nil
			case "enter": // Enter: Confirm all checked models and put them on agent list!
				var selected []string
				for id, checked := range m.checkedModels {
					if checked {
						selected = append(selected, id)
					}
				}

				// If nothing checked with Space, auto-check the currently highlighted model
				if len(selected) == 0 && len(filtered) > 0 && m.modelCursor < len(filtered) {
					selected = append(selected, filtered[m.modelCursor].ID)
				}

				if len(selected) == 0 {
					m.mode = modeList
					return m, nil
				}

				// Put selected models onto agent list
				addedCount := 0
				for _, modelID := range selected {
					role := "worker"
					toolCalling := true
					alias := ""

					// If no master exists yet, first selected becomes master
					if _, hasMaster := m.cfg.Agents["master"]; !hasMaster && addedCount == 0 {
						alias = "master"
						role = "master"
						toolCalling = false
					} else {
						// Clean short alias naming
						clean := strings.ToLower(modelID)
						base := "coder"
						if strings.Contains(clean, "sonnet") {
							if strings.Contains(clean, "3-7") {
								base = "coder-sonnet"
							} else if strings.Contains(clean, "5-5") || strings.Contains(clean, "55") {
								base = "coder-sonnet-55"
							} else {
								base = "coder-sonnet"
							}
						} else if strings.Contains(clean, "haiku") {
							if strings.Contains(clean, "5-5") || strings.Contains(clean, "55") {
								base = "coder-haiku-55"
							} else {
								base = "coder-haiku"
							}
						} else if strings.Contains(clean, "opus") {
							if strings.Contains(clean, "5-5") || strings.Contains(clean, "55") {
								base = "coder-opus-55"
							} else {
								base = "coder-opus"
							}
						} else if strings.Contains(clean, "flash") {
							base = "coder-flash"
						} else if strings.Contains(clean, "mini") {
							base = "coder-mini"
						} else if strings.Contains(clean, "qwen") {
							base = "coder-qwen"
						} else {
							parts := strings.Split(clean, "/")
							last := parts[len(parts)-1]
							if len(last) > 10 {
								last = last[:10]
							}
							base = "coder-" + last
						}

						alias = base
						idx := 2
						for {
							if _, exists := m.cfg.Agents[alias]; !exists {
								break
							}
							alias = fmt.Sprintf("%s-%d", base, idx)
							idx++
						}
					}

					m.cfg.Agents[alias] = config.AgentConfig{
						Provider:    m.configuredProviderKey,
						Model:       modelID,
						Role:        role,
						ToolCalling: toolCalling,
					}
					addedCount++
				}

				_ = config.SaveFile(m.configPath, m.cfg)
				m.statusMsg = fmt.Sprintf("✓ Successfully added %d model(s) to your agent list!", addedCount)
				m.activeTab = tabAgents
				m.mode = modeList
				m.cursor = 0
				return m, nil

			default:
				if len(msg.String()) == 1 && msg.String() >= "a" && msg.String() <= "z" || (msg.String() >= "0" && msg.String() <= "9") || msg.String() == "-" || msg.String() == "_" || msg.String() == "/" || msg.String() == "." {
					m.modelSearchQuery += msg.String()
					m.modelCursor = 0
					return m, nil
				}
			}

		case modePickModelRole: // Step 4: Pick Role (Master or Worker) for selected model
			switch msg.String() {
			case "esc":
				m.mode = modePickModel
				return m, nil
			case "up", "k":
				if m.roleCursor > 0 {
					m.roleCursor--
				}
				return m, nil
			case "down", "j":
				if m.roleCursor < 2 { // 0: Master, 1: Worker, 2: Back
					m.roleCursor++
				}
				return m, nil
			case "enter":
				if m.roleCursor == 2 {
					// Back
					m.mode = modePickModel
					return m, nil
				}

				if m.roleCursor == 0 {
					// Master
					m.cfg.Agents["master"] = config.AgentConfig{
						Provider:    m.configuredProviderKey,
						Model:       m.selectedModelID,
						Role:        "master",
						ToolCalling: false,
					}
					_ = config.SaveFile(m.configPath, m.cfg)
					m.statusMsg = fmt.Sprintf("✓ Configured 'master' agent with model '%s'!", m.selectedModelID)
					m.mode = modePickModel
					return m, nil
				}

				// Worker
				alias := "coder-fast"
				if _, exists := m.cfg.Agents[alias]; exists {
					alias = "coder-cheap"
					if _, exists2 := m.cfg.Agents[alias]; exists2 {
						cleanModel := strings.ReplaceAll(m.selectedModelID, "/", "-")
						cleanModel = strings.ReplaceAll(cleanModel, ":", "-")
						alias = fmt.Sprintf("worker-%s", cleanModel)
					}
				}

				m.cfg.Agents[alias] = config.AgentConfig{
					Provider:    m.configuredProviderKey,
					Model:       m.selectedModelID,
					Role:        "worker",
					ToolCalling: true,
				}
				_ = config.SaveFile(m.configPath, m.cfg)
				m.statusMsg = fmt.Sprintf("✓ Added worker agent '%s' with model '%s'!", alias, m.selectedModelID)
				m.mode = modePickModel
				return m, nil
			}

		case modePickAgentTemplate:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeList
				return m, nil
			case "up", "k":
				if m.templateCursor > 0 {
					m.templateCursor--
				}
				return m, nil
			case "down", "j":
				if m.templateCursor < len(agentTemplates)-1 {
					m.templateCursor++
				}
				return m, nil
			case "1", "2", "3", "4":
				idx := int(msg.Runes[0] - '1')
				if idx >= 0 && idx < len(agentTemplates) {
					m.templateCursor = idx
					return m.selectAgentTemplate()
				}
			case "enter":
				return m.selectAgentTemplate()
			}

		case modeAddProvider, modeEditProvider, modeAddAgent, modeEditAgent:
			switch msg.String() {
			case "esc":
				m.mode = modeList
				return m, nil
			case "tab", "down":
				m.focusNextInput()
				return m, nil
			case "shift+tab", "up":
				m.focusPrevInput()
				return m, nil
			case "enter":
				if m.isLastInputFocused() {
					return m.submitForm()
				}
				m.focusNextInput()
				return m, nil
			}

			var cmds []tea.Cmd
			for i := range m.inputs {
				var cmd tea.Cmd
				m.inputs[i], cmd = m.inputs[i].Update(msg)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	return m, nil
}

func (m *ConfigModel) selectAgentTemplate() (tea.Model, tea.Cmd) {
	tmpl := agentTemplates[m.templateCursor]
	provName := ""
	for p := range m.cfg.Providers {
		provName = p
		break
	}
	a := config.AgentConfig{
		Provider:    provName,
		Model:       tmpl.Model,
		Role:        tmpl.Role,
		ToolCalling: tmpl.ToolCalling,
	}
	m.initAgentInputs(tmpl.Alias, a)
	m.mode = modeAddAgent
	return m, m.focusCurrentInput()
}

func (m *ConfigModel) initProviderInputs(name string, p config.ProviderConfig) {
	m.inputs = make([]textinput.Model, 4)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "e.g. claude_subscription, claude, openai, 9router"
	m.inputs[0].SetValue(name)
	m.inputs[0].Prompt = "Provider Name: "

	m.inputs[1] = textinput.New()
	pType := p.Type
	if pType == "" {
		pType = "openai"
	}
	m.inputs[1].Placeholder = "openai | anthropic | claude_subscription"
	m.inputs[1].SetValue(pType)
	m.inputs[1].Prompt = "Type:          "

	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "e.g. http://localhost:11434/v1 (optional for claude_sub)"
	m.inputs[2].SetValue(p.BaseURL)
	m.inputs[2].Prompt = "Base URL:      "

	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "e.g. OPENROUTER_API_KEY (optional for local/claude_sub)"
	m.inputs[3].SetValue(p.APIKeyEnv)
	m.inputs[3].Prompt = "API Key Env:   "

	m.inputs[0].Focus()
}

func (m *ConfigModel) initAgentInputs(alias string, a config.AgentConfig) {
	m.inputs = make([]textinput.Model, 5)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "e.g. master, coder-cheap, coder-fast"
	m.inputs[0].SetValue(alias)
	m.inputs[0].Prompt = "Agent Alias:   "

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "Registered provider name (e.g. claude_subscription)"
	m.inputs[1].SetValue(a.Provider)
	m.inputs[1].Prompt = "Provider:      "

	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "e.g. sonnet, haiku, gpt-4o-mini"
	m.inputs[2].SetValue(a.Model)
	m.inputs[2].Prompt = "Model Name:    "

	m.inputs[3] = textinput.New()
	role := a.Role
	if role == "" {
		role = "worker"
	}
	m.inputs[3].Placeholder = "master or worker"
	m.inputs[3].SetValue(role)
	m.inputs[3].Prompt = "Role:          "

	m.inputs[4] = textinput.New()
	tc := "true"
	if !a.ToolCalling && a.Role == "master" {
		tc = "false"
	}
	m.inputs[4].Placeholder = "true or false"
	m.inputs[4].SetValue(tc)
	m.inputs[4].Prompt = "Tool Calling:  "

	m.inputs[0].Focus()
}

func (m *ConfigModel) focusNextInput() {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			m.inputs[i].Blur()
			next := (i + 1) % len(m.inputs)
			m.inputs[next].Focus()
			return
		}
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func (m *ConfigModel) focusPrevInput() {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			m.inputs[i].Blur()
			prev := (i - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[prev].Focus()
			return
		}
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func (m *ConfigModel) focusCurrentInput() tea.Cmd {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			return m.inputs[i].Focus()
		}
	}
	if len(m.inputs) > 0 {
		return m.inputs[0].Focus()
	}
	return nil
}

func (m *ConfigModel) isLastInputFocused() bool {
	if len(m.inputs) == 0 {
		return true
	}
	return m.inputs[len(m.inputs)-1].Focused()
}

func (m *ConfigModel) submitForm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAddProvider, modeEditProvider:
		name := strings.TrimSpace(m.inputs[0].Value())
		pType := strings.TrimSpace(m.inputs[1].Value())
		baseURL := strings.TrimSpace(m.inputs[2].Value())
		apiEnv := strings.TrimSpace(m.inputs[3].Value())

		if pType == "" {
			pType = "openai"
		}

		if name == "" {
			m.statusMsg = "Provider name is required!"
			return m, nil
		}
		if pType != "claude_subscription" && pType != "claude_sub" && baseURL == "" {
			if pType == "anthropic" {
				baseURL = "https://api.anthropic.com/v1"
			} else if name == "openai" {
				baseURL = "https://api.openai.com/v1"
			} else {
				m.statusMsg = "Base URL is required for standard OpenAI/Anthropic providers!"
				return m, nil
			}
		}

		m.cfg.Providers[name] = config.ProviderConfig{
			Type:      pType,
			BaseURL:   baseURL,
			APIKeyEnv: apiEnv,
		}

		if err := config.SaveFile(m.configPath, m.cfg); err != nil {
			m.err = err
			m.mode = modeList
			return m, nil
		}

		m.configuredProviderKey = name
		client := llm.NewProvider(pType, baseURL, apiEnv)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		models, err := client.ListModels(ctx)
		cancel()

		if err == nil && len(models) > 0 {
			m.fetchedModels = models
			m.checkedModels = make(map[string]bool)
			m.modelCursor = 0
			m.modelSearchQuery = ""
			m.statusMsg = fmt.Sprintf("✓ Provider '%s' connected! Check models to put on your list.", name)
			m.mode = modePickModel
			return m, nil
		}

		m.statusMsg = fmt.Sprintf("✓ Provider '%s' (%s) saved successfully!", name, pType)
		m.mode = modeList

	case modeAddAgent, modeEditAgent:
		alias := strings.TrimSpace(m.inputs[0].Value())
		prov := strings.TrimSpace(m.inputs[1].Value())
		model := strings.TrimSpace(m.inputs[2].Value())
		role := strings.ToLower(strings.TrimSpace(m.inputs[3].Value()))
		tcStr := strings.ToLower(strings.TrimSpace(m.inputs[4].Value()))

		if alias == "" || prov == "" || model == "" {
			m.statusMsg = "Alias, Provider, and Model are required!"
			return m, nil
		}

		if role != "master" && role != "worker" {
			m.statusMsg = "Role must be 'master' or 'worker'!"
			return m, nil
		}

		if _, ok := m.cfg.Providers[prov]; !ok {
			m.statusMsg = fmt.Sprintf("Provider '%s' does not exist! Add it in Providers tab first.", prov)
			return m, nil
		}

		tc := tcStr == "true" || tcStr == "yes" || tcStr == "1"

		m.cfg.Agents[alias] = config.AgentConfig{
			Provider:    prov,
			Model:       model,
			Role:        role,
			ToolCalling: tc,
		}

		if err := config.SaveFile(m.configPath, m.cfg); err != nil {
			m.err = err
		} else {
			m.statusMsg = fmt.Sprintf("✓ Agent '%s' saved successfully!", alias)
		}
		m.mode = modeList
	}

	return m, nil
}

func (m ConfigModel) getProviderNames() []string {
	var names []string
	for k := range m.cfg.Providers {
		names = append(names, k)
	}
	return names
}

func (m ConfigModel) getAgentAliases() []string {
	var aliases []string
	for k := range m.cfg.Agents {
		aliases = append(aliases, k)
	}
	return aliases
}

func (m ConfigModel) currentListLen() int {
	if m.activeTab == tabProviders {
		return len(m.cfg.Providers)
	}
	return len(m.cfg.Agents)
}

func (m ConfigModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(" RINGMASTER · Multi-Provider & Agent Configuration "))
	b.WriteString("\n\n")

	tabProv := inactiveTabStyle.Render(" [1] Providers ")
	tabAg := inactiveTabStyle.Render(" [2] Agents ")
	if m.activeTab == tabProviders {
		tabProv = activeTabStyle.Render(" [1] Providers ")
	} else {
		tabAg = activeTabStyle.Render(" [2] Agents ")
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabProv, " ", tabAg))
	b.WriteString("\n\n")

	// ==========================================
	// STEP 1: CHOOSE MODEL / AUTH PROVIDER
	// ==========================================
	if m.mode == modePickProviderStep1 {
		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render("Model/auth provider") + "\n")

		searchPrompt := "  Search: "
		if m.searchQuery != "" {
			searchPrompt += m.searchQuery
		} else {
			searchPrompt += "(type to filter...)"
		}
		b.WriteString(headerStyle.Render(searchPrompt) + "\n\n")

		filtered := m.getFilteredProviders()
		if len(filtered) == 0 {
			b.WriteString(hintStyle.Render("  No matching providers found.") + "\n\n")
		} else {
			for i, p := range filtered {
				if i == m.providerCursor {
					b.WriteString(selectedBulletStyle.Render("  ● ") + selectedItemStyle.Render(p.Name) + " " + descStyle.Render("("+p.Description+")") + "\n")
				} else {
					b.WriteString(normalBulletStyle.Render("  ○ ") + normalItemStyle.Render(p.Name) + " " + descStyle.Render("("+p.Description+")") + "\n")
				}
			}
		}

		b.WriteString("\n" + hintStyle.Render("↑/↓ to select · Enter: confirm · Type: to search · Esc: cancel") + "\n")
		return b.String()
	}

	// ==========================================
	// STEP 2: CHOOSE AUTH METHOD FOR SELECTED PROVIDER
	// ==========================================
	if m.mode == modePickProviderStep2 && m.selectedProvider != nil {
		// Completed step 1 header
		b.WriteString(stepDoneBullet.Render("●") + " " + stepDimTitleStyle.Render("Model/auth provider") + "\n")
		b.WriteString("  " + stepAnswerStyle.Render(m.selectedProvider.Name) + "\n\n")

		// Active step 2 header
		headerText := fmt.Sprintf("%s auth method", m.selectedProvider.Name)
		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render(headerText) + "\n")

		totalOptions := len(m.selectedProvider.AuthMethods) + 1
		for i := 0; i < totalOptions; i++ {
			if i < len(m.selectedProvider.AuthMethods) {
				mth := m.selectedProvider.AuthMethods[i]
				if i == m.authMethodCursor {
					b.WriteString(selectedBulletStyle.Render("  ● ") + selectedItemStyle.Render(mth.Label) + "\n")
				} else {
					b.WriteString(normalBulletStyle.Render("  ○ ") + normalItemStyle.Render(mth.Label) + "\n")
				}
			} else {
				// Back button
				if i == m.authMethodCursor {
					b.WriteString(selectedBulletStyle.Render("  ● ") + selectedItemStyle.Render("Back") + "\n")
				} else {
					b.WriteString(normalBulletStyle.Render("  ○ ") + normalItemStyle.Render("Back") + "\n")
				}
			}
		}

		b.WriteString("\n" + hintStyle.Render("↑/↓ to navigate · Enter: confirm · Esc: back") + "\n")
		return b.String()
	}

	// ==========================================
	// CLAUDE OAUTH INTERACTIVE FLOW
	// ==========================================
	if m.mode == modeClaudeOAuth {
		b.WriteString(stepDoneBullet.Render("●") + " " + stepDimTitleStyle.Render("Model/auth provider") + "\n")
		b.WriteString("  " + stepAnswerStyle.Render("Anthropic") + "\n\n")

		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render("Claude Subscription Authorization") + "\n\n")

		b.WriteString("  1. " + normalItemStyle.Render("Open this authorization URL in your browser:") + "\n")
		b.WriteString("     " + selectedItemStyle.Render(m.oauthURL) + "\n\n")
		b.WriteString("     " + hintStyle.Render("(Saved to: ~/.claude/auth_url.txt · Press [Ctrl+C] to copy)") + "\n\n")

		b.WriteString("  2. " + normalItemStyle.Render("Log in to Claude and click 'Approve'.") + "\n")
		b.WriteString("  3. " + normalItemStyle.Render("Copy the authorized code from the browser and paste it below:") + "\n\n")

		b.WriteString("  " + m.oauthInput.View() + "\n\n")

		if m.statusMsg != "" {
			b.WriteString("  " + errStyle.Render(m.statusMsg) + "\n\n")
		}

		b.WriteString(hintStyle.Render("Ctrl+C: Copy full URL to clipboard · [Enter] Submit code · [Esc] Cancel") + "\n")
		return b.String()
	}

	// ==========================================
	// STEP 3: PICK MODEL FROM FETCHED PROVIDER MODELS
	// ==========================================
	if m.mode == modePickModel {
		provLabel := m.configuredProviderKey
		if m.selectedProvider != nil {
			provLabel = m.selectedProvider.Name
		}

		b.WriteString(stepDoneBullet.Render("●") + " " + stepDimTitleStyle.Render("Model/auth provider") + "\n")
		b.WriteString("  " + stepAnswerStyle.Render(provLabel) + "\n\n")

		headerText := fmt.Sprintf("Select models to add to your agent roster (%d available)", len(m.fetchedModels))
		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render(headerText) + "\n")

		searchPrompt := "  Search: "
		if m.modelSearchQuery != "" {
			searchPrompt += m.modelSearchQuery
		} else {
			searchPrompt += hintStyle.Render("(type to filter...)")
		}
		b.WriteString(headerStyle.Render(searchPrompt) + "\n\n")

		filtered := m.getFilteredModels()
		totalItems := len(filtered)

		maxVisible := 12
		startIdx := 0
		if m.modelCursor >= maxVisible {
			startIdx = m.modelCursor - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > totalItems {
			endIdx = totalItems
		}

		for i := startIdx; i < endIdx; i++ {
			if i >= len(filtered) {
				break
			}
			mod := filtered[i]
			desc := ""
			if mod.Desc != "" {
				desc = " " + descStyle.Render("("+mod.Desc+")")
			}

			isChecked := m.checkedModels != nil && m.checkedModels[mod.ID]
			box := "[ ] "
			if isChecked {
				box = "[x] "
			}

			if i == m.modelCursor {
				boxStyle := selectedBulletStyle
				if isChecked {
					boxStyle = selectedBulletStyle
				}
				b.WriteString("  " + boxStyle.Render(box) + selectedItemStyle.Render(mod.ID) + desc + "\n")
			} else {
				boxStyle := normalBulletStyle
				if isChecked {
					boxStyle = selectedBulletStyle
				}
				b.WriteString("  " + boxStyle.Render(box) + normalItemStyle.Render(mod.ID) + desc + "\n")
			}
		}

		if totalItems > maxVisible {
			b.WriteString(hintStyle.Render(fmt.Sprintf("  ... and %d more models (type to filter)", totalItems-endIdx)) + "\n")
		}

		checkedCount := 0
		for _, v := range m.checkedModels {
			if v {
				checkedCount++
			}
		}

		statusSummary := fmt.Sprintf("(%d model(s) checked)", checkedCount)
		b.WriteString("\n" + stepDoneBullet.Render(statusSummary) + " " + hintStyle.Render("[Space] Check/Uncheck · [Enter] Put selected on list · Type: filter · [Esc] Done") + "\n")
		return b.String()
	}

	// ==========================================
	// STEP 4: ASSIGN AGENT ROLE FOR SELECTED MODEL
	// ==========================================
	if m.mode == modePickModelRole {
		provLabel := m.configuredProviderKey
		if m.selectedProvider != nil {
			provLabel = m.selectedProvider.Name
		}

		b.WriteString(stepDoneBullet.Render("●") + " " + stepDimTitleStyle.Render("Model/auth provider") + "\n")
		b.WriteString("  " + stepAnswerStyle.Render(provLabel) + "\n\n")

		b.WriteString(stepDoneBullet.Render("●") + " " + stepDimTitleStyle.Render("Selected Model") + "\n")
		b.WriteString("  " + stepAnswerStyle.Render(m.selectedModelID) + "\n\n")

		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render("Assign Agent Role in Ringmaster:") + "\n")

		roleOptions := []string{
			"Master Planner (Ringmaster Control Plane · breaks tasks into DAG)",
			"Worker Coder (Executes code in isolated git worktrees · Tool Calling: true)",
			"Back to Model List",
		}

		for i, opt := range roleOptions {
			if i == m.roleCursor {
				b.WriteString(selectedBulletStyle.Render("  ● ") + selectedItemStyle.Render(opt) + "\n")
			} else {
				b.WriteString(normalBulletStyle.Render("  ○ ") + normalItemStyle.Render(opt) + "\n")
			}
		}

		b.WriteString("\n" + hintStyle.Render("↑/↓ to navigate · Enter: confirm & bind agent · Esc: back") + "\n")
		return b.String()
	}

	// AGENT TEMPLATE PICKER
	if m.mode == modePickAgentTemplate {
		b.WriteString(stepActiveBullet.Render("●") + " " + stepTitleStyle.Render("Choose Agent Role / Template") + "\n\n")

		for i, tmpl := range agentTemplates {
			numBadge := fmt.Sprintf("[%d] ", i+1)
			if i == m.templateCursor {
				b.WriteString(selectedBulletStyle.Render("  ● ") + selectedItemStyle.Render(numBadge+tmpl.Title) + "\n")
				b.WriteString("      " + descStyle.Render(tmpl.Description) + "\n")
			} else {
				b.WriteString(normalBulletStyle.Render("  ○ ") + normalItemStyle.Render(numBadge+tmpl.Title) + "\n")
				b.WriteString("      " + descStyle.Render(tmpl.Description) + "\n")
			}
		}
		b.WriteString("\n" + hintStyle.Render("Press [1-4] or ↑/↓ and Enter to confirm · Esc: cancel") + "\n")
		return b.String()
	}

	// FORM VIEW
	if m.mode == modeAddProvider || m.mode == modeEditProvider || m.mode == modeAddAgent || m.mode == modeEditAgent {
		title := "Add Provider"
		if m.mode == modeEditProvider {
			title = "Edit Provider"
		} else if m.mode == modeAddAgent {
			title = "Add Agent"
		} else if m.mode == modeEditAgent {
			title = "Edit Agent"
		}

		b.WriteString(headerStyle.Render("★ " + strings.ToUpper(title)) + "\n")
		b.WriteString(hintStyle.Render("Press [Tab] or [↓] to switch fields, [Enter] on last field to submit, [Esc] to cancel.") + "\n\n")

		for i := range m.inputs {
			b.WriteString(m.inputs[i].View() + "\n")
		}

		b.WriteString("\n")
		if m.statusMsg != "" {
			b.WriteString(statusStyle.Render(m.statusMsg) + "\n")
		}
		return b.String()
	}

	// LIST VIEW: PROVIDERS
	if m.activeTab == tabProviders {
		names := m.getProviderNames()
		if len(names) == 0 {
			b.WriteString(hintStyle.Render("No providers registered yet. Press [a] to choose a provider.") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("%-20s %-20s %-34s %-18s\n", headerStyle.Render("PROVIDER NAME"), headerStyle.Render("TYPE"), headerStyle.Render("ENDPOINT / BASE URL"), headerStyle.Render("API KEY ENV")))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

			for i, name := range names {
				p := m.cfg.Providers[name]
				prefix := "  "
				lineStyle := normalItemStyle
				if i == m.cursor {
					prefix = "▸ "
					lineStyle = selectedItemStyle
				}

				pType := p.Type
				if pType == "" {
					pType = "openai"
				}
				displayURL := p.BaseURL
				if pType == "claude_subscription" || pType == "claude_sub" {
					displayURL = "local (claude-cli / subscription)"
				}

				row := fmt.Sprintf("%s%-18s %-20s %-34s %-18s", prefix, name, pType, displayURL, p.APIKeyEnv)
				b.WriteString(lineStyle.Render(row) + "\n")
			}
		}
	} else {
		// LIST VIEW: AGENTS
		aliases := m.getAgentAliases()
		if len(aliases) == 0 {
			b.WriteString(hintStyle.Render("No agents configured yet. Press [a] to add an agent.") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("%-20s %-22s %-32s %-10s %-10s\n", headerStyle.Render("ALIAS"), headerStyle.Render("PROVIDER"), headerStyle.Render("MODEL"), headerStyle.Render("ROLE"), headerStyle.Render("TOOL CALL")))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

			for i, alias := range aliases {
				a := m.cfg.Agents[alias]
				prefix := "  "
				lineStyle := normalItemStyle
				if i == m.cursor {
					prefix = "▸ "
					lineStyle = selectedItemStyle
				}

				tc := "no"
				if a.ToolCalling {
					tc = "yes"
				}

				row := fmt.Sprintf("%s%-18s %-22s %-32s %-10s %-10s", prefix, alias, a.Provider, a.Model, a.Role, tc)
				b.WriteString(lineStyle.Render(row) + "\n")
			}
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n\n")
	}

	help := hintStyle.Render("[a] Add · [e/Enter] Edit · [d] Delete · [s] Save · [Tab] Switch Tab · [q] Quit")
	b.WriteString(help + "\n")

	return b.String()
}

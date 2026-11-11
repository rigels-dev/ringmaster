package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/auth"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/registry"
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
				if maxLen > 0 && m.cursor < maxLen-1 {
					m.cursor++
				}
				return m, nil
			case "left", "h", "home":
				m.cursor = 0
				return m, nil
			case "right", "l", "end":
				maxLen := m.currentListLen()
				if maxLen > 0 {
					m.cursor = maxLen - 1
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
						m.deletingProviderName = names[m.cursor]
						m.mode = modeConfirmDeleteProvider
						return m, nil
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
			case "m": // Set highlighted agent as Master
				if m.activeTab == tabAgents {
					aliases := m.getAgentAliases()
					if m.cursor < len(aliases) {
						selectedAlias := aliases[m.cursor]
						// Set all other agents to worker if they were master
						for aName, a := range m.cfg.Agents {
							if a.Role == "master" && aName != selectedAlias {
								a.Role = "worker"
								a.ToolCalling = true
								m.cfg.Agents[aName] = a
							}
						}
						// Set selected as master
						target := m.cfg.Agents[selectedAlias]
						target.Role = "master"
						target.ToolCalling = false
						m.cfg.Agents[selectedAlias] = target
						m.cfg.Routing.DefaultMaster = selectedAlias
						_ = config.SaveFile(m.configPath, m.cfg)
						m.statusMsg = fmt.Sprintf("★ '%s' (%s) is now the MASTER Planner!", selectedAlias, target.Model)
						return m, nil
					}
				}

			case "w": // Set highlighted agent as Default Worker
				if m.activeTab == tabAgents {
					aliases := m.getAgentAliases()
					if m.cursor < len(aliases) {
						selectedAlias := aliases[m.cursor]
						target := m.cfg.Agents[selectedAlias]
						target.Role = "worker"
						target.ToolCalling = true
						m.cfg.Agents[selectedAlias] = target
						m.cfg.Routing.DefaultWorker = selectedAlias
						_ = config.SaveFile(m.configPath, m.cfg)
						m.statusMsg = fmt.Sprintf("⚡ '%s' (%s) is now the DEFAULT Worker!", selectedAlias, target.Model)
						return m, nil
					}
				}

			case "p": // Ping / Test selected agent or provider
				if m.activeTab == tabAgents {
					aliases := m.getAgentAliases()
					if m.cursor < len(aliases) {
						selectedAlias := aliases[m.cursor]
						agentCfg := m.cfg.Agents[selectedAlias]
						m.statusMsg = fmt.Sprintf("⏳ Pinging agent '%s' (%s via %s)...", selectedAlias, agentCfg.Model, agentCfg.Provider)

						cfgCopy := m.cfg
						return m, func() tea.Msg {
							reg := registry.NewRegistry(cfgCopy)
							provider, resolvedAgent, err := reg.GetProviderForAgent(selectedAlias)
							if err != nil {
								return pingResultMsg{target: selectedAlias, err: err}
							}

							start := time.Now()
							ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
							defer cancel()

							resp, err := provider.Chat(ctx, resolvedAgent.Model, llm.ChatRequest{
								Messages: []llm.Message{
									{Role: "user", Content: "Respond with exactly the single word 'PONG'."},
								},
							})
							latency := time.Since(start)

							tokens := 0
							if resp != nil {
								tokens = resp.Usage.TotalTokens
							}
							return pingResultMsg{
								target:  selectedAlias,
								latency: latency,
								err:     err,
								tokens:  tokens,
							}
						}
					}
				} else {
					names := m.getProviderNames()
					if m.cursor < len(names) {
						selectedProv := names[m.cursor]
						p := m.cfg.Providers[selectedProv]
						m.statusMsg = fmt.Sprintf("⏳ Pinging provider '%s' (%s)...", selectedProv, p.Type)

						return m, func() tea.Msg {
							start := time.Now()
							client := llm.NewProvider(p.Type, p.BaseURL, p.APIKeyEnv)
							ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
							defer cancel()

							models, err := client.ListModels(ctx)
							latency := time.Since(start)

							if err != nil {
								return pingResultMsg{target: selectedProv, latency: latency, err: err}
							}
							return pingResultMsg{
								target:  fmt.Sprintf("%s (%d models reachable)", selectedProv, len(models)),
								latency: latency,
								err:     nil,
							}
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

		case modePickModel: // Step 3: Check / Select models to put on list
			filtered := m.getFilteredModels()
			totalItems := len(filtered)

			switch msg.String() {
			case "esc":
				// If provider was pending, cancel and discard it without saving!
				m.pendingProviderConfig = nil
				m.mode = modeList
				m.statusMsg = "Model selection canceled. Provider setup was not saved."
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
			case "left", "home":
				m.modelCursor = 0
				return m, nil
			case "right", "end":
				if totalItems > 0 {
					m.modelCursor = totalItems - 1
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
				// Walk the list in display order (not the map), so the first checked
				// model reliably becomes the master when there is none yet
				var selected []string
				for _, mod := range m.fetchedModels {
					if m.checkedModels[mod.ID] {
						selected = append(selected, mod.ID)
					}
				}

				// If nothing checked with Space, auto-check the currently highlighted model
				if len(selected) == 0 && len(filtered) > 0 && m.modelCursor < len(filtered) {
					selected = append(selected, filtered[m.modelCursor].ID)
				}

				if len(selected) == 0 {
					m.pendingProviderConfig = nil
					m.mode = modeList
					return m, nil
				}

				// Atomically commit pending provider to config now that models are confirmed!
				if m.pendingProviderConfig != nil && m.configuredProviderKey != "" {
					m.cfg.Providers[m.configuredProviderKey] = *m.pendingProviderConfig
					m.pendingProviderConfig = nil
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

		case modeConfirmDeleteProvider:
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

				// Atomically commit pending provider if entering via role picker
				if m.pendingProviderConfig != nil && m.configuredProviderKey != "" {
					m.cfg.Providers[m.configuredProviderKey] = *m.pendingProviderConfig
					m.pendingProviderConfig = nil
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

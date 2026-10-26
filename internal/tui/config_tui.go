package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ringmaster/internal/config"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#DEAA3B")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#29F2EC"))

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#192026")).
			Background(lipgloss.Color("#29F2EC")).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8493A0")).
			Background(lipgloss.Color("#1B2229")).
			Padding(0, 2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DEAA3B")).
			Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#29F2EC")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EFE4CE"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51606D")).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5BF28D")).
			Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E05436")).
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
	modePickProviderTemplate
	modeEditProvider
	modeAddProvider
	modePickAgentTemplate
	modeEditAgent
	modeAddAgent
)

type ProviderTemplate struct {
	Title       string
	Description string
	Name        string
	Type        string
	BaseURL     string
	APIKeyEnv   string
}

var providerTemplates = []ProviderTemplate{
	{
		Title:       "Claude Subscription",
		Description: "Uses local ~/.claude session / Claude Code CLI (No API key or URL required)",
		Name:        "claude_sub",
		Type:        "claude_subscription",
		BaseURL:     "",
		APIKeyEnv:   "",
	},
	{
		Title:       "OpenAI Compatible",
		Description: "Standard OpenAI endpoint (OpenAI, OpenRouter, vLLM)",
		Name:        "openai",
		Type:        "openai",
		BaseURL:     "https://api.openai.com/v1",
		APIKeyEnv:   "OPENAI_API_KEY",
	},
	{
		Title:       "Claude Official API",
		Description: "Direct Anthropic Messages API (api.anthropic.com)",
		Name:        "claude",
		Type:        "anthropic",
		BaseURL:     "https://api.anthropic.com/v1",
		APIKeyEnv:   "ANTHROPIC_API_KEY",
	},
	{
		Title:       "9router (Local Proxy)",
		Description: "Local multi-provider proxy on http://localhost:20128/v1",
		Name:        "9router",
		Type:        "openai",
		BaseURL:     "http://localhost:20128/v1",
		APIKeyEnv:   "NINEROUTER_API_KEY",
	},
	{
		Title:       "Ollama (Local LLM)",
		Description: "Self-hosted local models on http://localhost:11434/v1",
		Name:        "ollama_local",
		Type:        "openai",
		BaseURL:     "http://localhost:11434/v1",
		APIKeyEnv:   "",
	},
	{
		Title:       "Custom Manual Entry",
		Description: "Fill provider name, type, endpoint URL, and API key environment variable",
		Name:        "",
		Type:        "openai",
		BaseURL:     "",
		APIKeyEnv:   "",
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
	cfg            *config.Config
	configPath     string
	activeTab      tab
	mode           viewMode
	cursor         int
	templateCursor int
	inputs         []textinput.Model
	statusMsg      string
	err            error
	width          int
	height         int
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

func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
				m.templateCursor = 0
				if m.activeTab == tabProviders {
					m.mode = modePickProviderTemplate
				} else {
					m.mode = modePickAgentTemplate
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

		case modePickProviderTemplate:
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
				if m.templateCursor < len(providerTemplates)-1 {
					m.templateCursor++
				}
				return m, nil
			case "1", "2", "3", "4", "5", "6":
				idx := int(msg.Runes[0] - '1')
				if idx >= 0 && idx < len(providerTemplates) {
					m.templateCursor = idx
					return m.selectProviderTemplate()
				}
			case "enter":
				return m.selectProviderTemplate()
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

func (m *ConfigModel) selectProviderTemplate() (tea.Model, tea.Cmd) {
	tmpl := providerTemplates[m.templateCursor]
	p := config.ProviderConfig{
		Type:      tmpl.Type,
		BaseURL:   tmpl.BaseURL,
		APIKeyEnv: tmpl.APIKeyEnv,
	}
	m.initProviderInputs(tmpl.Name, p)
	m.mode = modeAddProvider
	return m, m.focusCurrentInput()
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
		} else {
			m.statusMsg = fmt.Sprintf("✓ Provider '%s' (%s) saved successfully!", name, pType)
		}
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

	if m.mode == modePickProviderTemplate {
		b.WriteString(headerStyle.Render("★ CHOOSE A PROVIDER TEMPLATE"))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("Press [1-6] or use [↑/↓] and press [Enter] to select. Press [Esc] to cancel."))
		b.WriteString("\n\n")

		for i, tmpl := range providerTemplates {
			prefix := "  "
			itemStyle := normalItemStyle
			if i == m.templateCursor {
				prefix = "▸ "
				itemStyle = selectedItemStyle
			}

			numBadge := fmt.Sprintf("[%d] ", i+1)
			b.WriteString(prefix)
			b.WriteString(itemStyle.Render(numBadge + tmpl.Title))
			b.WriteString("\n")
			b.WriteString("    " + hintStyle.Render(tmpl.Description) + "\n\n")
		}
		return b.String()
	}

	if m.mode == modePickAgentTemplate {
		b.WriteString(headerStyle.Render("★ CHOOSE AN AGENT TEMPLATE"))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("Press [1-4] or use [↑/↓] and press [Enter] to select. Press [Esc] to cancel."))
		b.WriteString("\n\n")

		for i, tmpl := range agentTemplates {
			prefix := "  "
			itemStyle := normalItemStyle
			if i == m.templateCursor {
				prefix = "▸ "
				itemStyle = selectedItemStyle
			}

			numBadge := fmt.Sprintf("[%d] ", i+1)
			b.WriteString(prefix)
			b.WriteString(itemStyle.Render(numBadge + tmpl.Title))
			b.WriteString("\n")
			b.WriteString("    " + hintStyle.Render(tmpl.Description) + "\n\n")
		}
		return b.String()
	}

	if m.mode == modeAddProvider || m.mode == modeEditProvider || m.mode == modeAddAgent || m.mode == modeEditAgent {
		title := "Add Provider"
		if m.mode == modeEditProvider {
			title = "Edit Provider"
		} else if m.mode == modeAddAgent {
			title = "Add Agent"
		} else if m.mode == modeEditAgent {
			title = "Edit Agent"
		}

		b.WriteString(headerStyle.Render(fmt.Sprintf("★ %s", strings.ToUpper(title))))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("Press [Tab] or [↓] to switch fields, [Enter] on last field to submit, [Esc] to cancel."))
		b.WriteString("\n\n")

		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			b.WriteString("\n")
		}

		b.WriteString("\n")
		if m.statusMsg != "" {
			b.WriteString(statusStyle.Render(m.statusMsg))
			b.WriteString("\n")
		}
		return b.String()
	}

	if m.activeTab == tabProviders {
		names := m.getProviderNames()
		if len(names) == 0 {
			b.WriteString(hintStyle.Render("No providers registered yet. Press [a] to choose a template."))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("%-18s %-16s %-32s %-20s\n", headerStyle.Render("PROVIDER NAME"), headerStyle.Render("TYPE"), headerStyle.Render("BASE URL"), headerStyle.Render("API KEY ENV")))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("──────────────────────────────────────────────────────────────────────────────────────────\n"))

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

				row := fmt.Sprintf("%s%-16s %-16s %-32s %-20s", prefix, name, pType, displayURL, p.APIKeyEnv)
				b.WriteString(lineStyle.Render(row))
				b.WriteString("\n")
			}
		}
	} else {
		aliases := m.getAgentAliases()
		if len(aliases) == 0 {
			b.WriteString(hintStyle.Render("No agents configured yet. Press [a] to choose a template."))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("%-16s %-16s %-24s %-10s %-12s\n", headerStyle.Render("ALIAS"), headerStyle.Render("PROVIDER"), headerStyle.Render("MODEL"), headerStyle.Render("ROLE"), headerStyle.Render("TOOL CALL")))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("──────────────────────────────────────────────────────────────────────────────────────────\n"))

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

				row := fmt.Sprintf("%s%-14s %-16s %-24s %-10s %-12s", prefix, alias, a.Provider, a.Model, a.Role, tc)
				b.WriteString(lineStyle.Render(row))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg))
		b.WriteString("\n\n")
	}

	help := hintStyle.Render("[a] Add with Template · [e/Enter] Edit · [d] Delete · [s] Save to TOML · [Tab] Switch Tab · [q] Quit")
	b.WriteString(help)
	b.WriteString("\n")

	return b.String()
}

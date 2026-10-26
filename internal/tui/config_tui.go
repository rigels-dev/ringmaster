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

	savedStyle = lipgloss.NewStyle().
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
	modeEditProvider
	modeAddProvider
	modeEditAgent
	modeAddAgent
)

type ConfigModel struct {
	cfg        *config.Config
	configPath string
	activeTab  tab
	mode       viewMode
	cursor     int
	inputs     []textinput.Model
	statusMsg  string
	err        error
	width      int
	height     int
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
		m.statusMsg = ""
		switch m.mode {
		case modeList:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "tab":
				if m.activeTab == tabProviders {
					m.activeTab = tabAgents
				} else {
					m.activeTab = tabProviders
				}
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
			case "a": // Add new
				if m.activeTab == tabProviders {
					m.initProviderInputs("", config.ProviderConfig{})
					m.mode = modeAddProvider
				} else {
					m.initAgentInputs("", config.AgentConfig{})
					m.mode = modeAddAgent
				}
				return m, m.focusCurrentInput()
			case "enter", "e": // Edit
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
			case "d", "x": // Delete
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
				return m, nil
			case "s": // Save
				if err := config.SaveFile(m.configPath, m.cfg); err != nil {
					m.err = err
				} else {
					m.statusMsg = fmt.Sprintf("✓ Saved to %s", m.configPath)
				}
				return m, nil
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
				// If on last input, submit form
				if m.isLastInputFocused() {
					return m.submitForm()
				}
				m.focusNextInput()
				return m, nil
			}

			// Update active textinput
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

func (m *ConfigModel) initProviderInputs(name string, p config.ProviderConfig) {
	m.inputs = make([]textinput.Model, 3)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "e.g. ollama_local, openrouter, 9router"
	m.inputs[0].SetValue(name)
	m.inputs[0].Prompt = "Provider Name: "

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "e.g. http://localhost:11434/v1"
	m.inputs[1].SetValue(p.BaseURL)
	m.inputs[1].Prompt = "Base URL:      "

	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "e.g. OPENROUTER_API_KEY (optional for local)"
	m.inputs[2].SetValue(p.APIKeyEnv)
	m.inputs[2].Prompt = "API Key Env:   "

	m.inputs[0].Focus()
}

func (m *ConfigModel) initAgentInputs(alias string, a config.AgentConfig) {
	m.inputs = make([]textinput.Model, 5)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "e.g. master, coder-cheap, coder-fast"
	m.inputs[0].SetValue(alias)
	m.inputs[0].Prompt = "Agent Alias:   "

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "Registered provider name (e.g. ollama_local)"
	m.inputs[1].SetValue(a.Provider)
	m.inputs[1].Prompt = "Provider:      "

	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "e.g. qwen2.5-coder:7b, claude-3.5-haiku"
	m.inputs[2].SetValue(a.Model)
	m.inputs[2].Prompt = "Model:         "

	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "master / worker"
	role := a.Role
	if role == "" {
		role = "worker"
	}
	m.inputs[3].SetValue(role)
	m.inputs[3].Prompt = "Role:          "

	m.inputs[4] = textinput.New()
	m.inputs[4].Placeholder = "true / false"
	tc := "true"
	if !a.ToolCalling && alias != "" {
		tc = "false"
	}
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
}

func (m *ConfigModel) focusCurrentInput() tea.Cmd {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			return textinput.Blink
		}
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
	return textinput.Blink
}

func (m *ConfigModel) isLastInputFocused() bool {
	if len(m.inputs) == 0 {
		return false
	}
	return m.inputs[len(m.inputs)-1].Focused()
}

func (m *ConfigModel) submitForm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAddProvider, modeEditProvider:
		name := strings.TrimSpace(m.inputs[0].Value())
		baseURL := strings.TrimSpace(m.inputs[1].Value())
		apiEnv := strings.TrimSpace(m.inputs[2].Value())

		if name == "" || baseURL == "" {
			m.statusMsg = "Provider name and Base URL are required!"
			return m, nil
		}

		m.cfg.Providers[name] = config.ProviderConfig{
			BaseURL:   baseURL,
			APIKeyEnv: apiEnv,
		}

		if err := config.SaveFile(m.configPath, m.cfg); err != nil {
			m.err = err
		} else {
			m.statusMsg = fmt.Sprintf("✓ Provider '%s' saved successfully!", name)
		}
		m.mode = modeList

	case modeAddAgent, modeEditAgent:
		alias := strings.TrimSpace(m.inputs[0].Value())
		prov := strings.TrimSpace(m.inputs[1].Value())
		model := strings.TrimSpace(m.inputs[2].Value())
		role := strings.ToLower(strings.TrimSpace(m.inputs[3].Value()))
		toolCalling := strings.ToLower(strings.TrimSpace(m.inputs[4].Value())) == "true"

		if alias == "" || prov == "" || model == "" {
			m.statusMsg = "Alias, Provider, and Model are required!"
			return m, nil
		}

		if role != "master" && role != "worker" {
			role = "worker"
		}

		m.cfg.Agents[alias] = config.AgentConfig{
			Provider:    prov,
			Model:       model,
			Role:        role,
			ToolCalling: toolCalling,
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

	// Title
	b.WriteString(titleStyle.Render("⚙ RINGMASTER — PROVIDER & MODEL REGISTRY"))
	b.WriteString("\n\n")

	// Tabs
	tabProv := inactiveTabStyle.Render("1. PROVIDERS")
	tabAg := inactiveTabStyle.Render("2. AGENTS & MODELS")

	if m.activeTab == tabProviders {
		tabProv = activeTabStyle.Render("1. PROVIDERS")
	} else {
		tabAg = activeTabStyle.Render("2. AGENTS & MODELS")
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabProv, " ", tabAg))
	b.WriteString("\n\n")

	// Form Mode
	if m.mode != modeList {
		formTitle := "Add New Provider"
		if m.mode == modeEditProvider {
			formTitle = "Edit Provider"
		} else if m.mode == modeAddAgent {
			formTitle = "Add New Agent / Model"
		} else if m.mode == modeEditAgent {
			formTitle = "Edit Agent / Model"
		}

		b.WriteString(headerStyle.Render(fmt.Sprintf("━━━ %s ━━━", formTitle)))
		b.WriteString("\n\n")

		for _, in := range m.inputs {
			b.WriteString(in.View())
			b.WriteString("\n\n")
		}

		b.WriteString(hintStyle.Render("Press [Tab] to navigate fields • [Enter] to submit • [Esc] to cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// List Mode
	if m.activeTab == tabProviders {
		names := m.getProviderNames()
		if len(names) == 0 {
			b.WriteString(hintStyle.Render("No providers registered yet. Press [a] to add one."))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("%-20s %-36s %-20s\n", headerStyle.Render("PROVIDER NAME"), headerStyle.Render("BASE URL"), headerStyle.Render("API KEY ENV")))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("──────────────────────────────────────────────────────────────────────────\n"))

			for i, name := range names {
				p := m.cfg.Providers[name]
				prefix := "  "
				lineStyle := normalItemStyle
				if i == m.cursor {
					prefix = "▸ "
					lineStyle = selectedItemStyle
				}

				row := fmt.Sprintf("%s%-18s %-36s %-20s", prefix, name, p.BaseURL, p.APIKeyEnv)
				b.WriteString(lineStyle.Render(row))
				b.WriteString("\n")
			}
		}
	} else {
		aliases := m.getAgentAliases()
		if len(aliases) == 0 {
			b.WriteString(hintStyle.Render("No agents registered yet. Press [a] to add one."))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("%-18s %-16s %-28s %-10s %-12s\n",
				headerStyle.Render("ALIAS"),
				headerStyle.Render("PROVIDER"),
				headerStyle.Render("MODEL"),
				headerStyle.Render("ROLE"),
				headerStyle.Render("TOOL CALLING"),
			))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#2E3842")).Render("────────────────────────────────────────────────────────────────────────────────────────\n"))

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

				row := fmt.Sprintf("%s%-16s %-16s %-28s %-10s %-12s", prefix, alias, a.Provider, a.Model, a.Role, tc)
				b.WriteString(lineStyle.Render(row))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(savedStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("[Tab] Switch Tab • [↑/↓] Navigate • [a] Add • [Enter/e] Edit • [d/x] Delete • [s] Save • [q] Quit"))
	b.WriteString("\n")

	return b.String()
}

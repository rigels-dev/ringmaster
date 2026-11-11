package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
)

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

		client := llm.NewProvider(pType, baseURL, apiEnv)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		models, err := client.ListModels(ctx)
		cancel()

		pCfg := config.ProviderConfig{
			Type:      pType,
			BaseURL:   baseURL,
			APIKeyEnv: apiEnv,
		}

		if err == nil && len(models) > 0 {
			m.pendingProviderConfig = &pCfg
			m.configuredProviderKey = name
			m.fetchedModels = models
			m.checkedModels = make(map[string]bool)
			m.modelCursor = 0
			m.modelSearchQuery = ""
			m.statusMsg = fmt.Sprintf("✓ Provider '%s' connected! Check models to put on your list.", name)
			m.mode = modePickModel
			return m, nil
		}

		// If no models fetched or direct edit/add, save directly
		m.cfg.Providers[name] = pCfg
		if err := config.SaveFile(m.configPath, m.cfg); err != nil {
			m.err = err
			m.mode = modeList
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

// updatePickAgentTemplate chooses a template to prefill the add-agent form.
func (m ConfigModel) updatePickAgentTemplate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	return m, nil
}

// updateForm handles keys in the add/edit forms for providers and agents.
func (m ConfigModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

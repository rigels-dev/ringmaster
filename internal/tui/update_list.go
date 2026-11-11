package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/registry"
)

// updateList handles keys on the provider and agent lists.
func (m ConfigModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.activeTab == tabSkills {
		if next, handled := m.updateSkills(msg); handled {
			return next, nil
		}
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.activeTab = (m.activeTab + 1) % (tabSkills + 1)
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
	case "3":
		m.activeTab = tabSkills
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

	return m, nil
}

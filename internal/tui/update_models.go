package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
)

// updatePickModel is wizard step 3: check the models to add as agents.
func (m ConfigModel) updatePickModel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	return m, nil
}

// updatePickModelRole is wizard step 4: pick master or worker for the selected model.
func (m ConfigModel) updatePickModelRole(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	return m, nil
}

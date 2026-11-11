package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	// CONFIRM DELETE PROVIDER MODAL
	// ==========================================
	if m.mode == modeConfirmDeleteProvider {
		provName := m.deletingProviderName
		var boundAgents []string
		for aName, a := range m.cfg.Agents {
			if a.Provider == provName {
				boundAgents = append(boundAgents, fmt.Sprintf("%s (%s)", aName, a.Model))
			}
		}

		b.WriteString(errStyle.Render("⚠ CONFIRM PROVIDER DELETION") + "\n\n")
		b.WriteString(normalItemStyle.Render(fmt.Sprintf("Are you sure you want to delete provider '%s'?", provName)) + "\n\n")

		if len(boundAgents) > 0 {
			b.WriteString(errStyle.Render(fmt.Sprintf("WARNING: Deleting this provider will also delete %d dependent agent(s):", len(boundAgents))) + "\n")
			for _, ag := range boundAgents {
				b.WriteString(hintStyle.Render("  • "+ag) + "\n")
			}
			b.WriteString("\n")
		} else {
			b.WriteString(hintStyle.Render("No agents are currently using this provider.") + "\n\n")
		}

		b.WriteString(selectedBulletStyle.Render("[y / Enter] Confirm Delete") + "   " + normalItemStyle.Render("[n / Esc] Cancel") + "\n")
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

		b.WriteString(headerStyle.Render("★ "+strings.ToUpper(title)) + "\n")
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
			plainHeader := fmt.Sprintf("  %-18s %-20s %-38s %-18s", "PROVIDER NAME", "TYPE", "ENDPOINT / BASE URL", "API KEY ENV")
			b.WriteString(headerStyle.Render(plainHeader) + "\n")
			b.WriteString(tableBorderLine.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

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

				row := fmt.Sprintf("%-18s %-20s %-38s %-18s", name, pType, displayURL, p.APIKeyEnv)
				b.WriteString(prefix + lineStyle.Render(row) + "\n")
			}
		}
	} else {
		// LIST VIEW: AGENTS
		aliases := m.getAgentAliases()
		if len(aliases) == 0 {
			b.WriteString(hintStyle.Render("No agents configured yet. Press [a] to add an agent.") + "\n")
		} else {
			currentMaster := m.cfg.Routing.DefaultMaster
			if currentMaster == "" {
				for aName, a := range m.cfg.Agents {
					if a.Role == "master" {
						currentMaster = aName
						break
					}
				}
			}
			currentWorker := m.cfg.Routing.DefaultWorker

			masterInfo := fmt.Sprintf(" RINGMASTER ROUTING: [Master Planner: %s]  [Default Worker: %s] ", currentMaster, currentWorker)
			routingBox := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#2E3842"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
				Bold(true).
				Render(masterInfo)

			b.WriteString(routingBox + "\n")
			b.WriteString(hintStyle.Render(" [m] Set highlighted as Master · [w] Set as Default Worker") + "\n\n")

			// Table Header with wider columns for Model
			plainHeader := fmt.Sprintf("  %-18s %-32s %-16s %-14s %-6s", "ALIAS", "MODEL", "PROVIDER", "ROLE", "TOOLS")
			b.WriteString(headerStyle.Render(plainHeader) + "\n")
			b.WriteString(tableBorderLine.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

			for i, alias := range aliases {
				a := m.cfg.Agents[alias]
				prefix := "  "
				lineStyle := normalItemStyle
				isCursor := i == m.cursor
				if isCursor {
					prefix = cursorIndicator.Render("▸ ")
					lineStyle = selectedItemStyle
				}

				roleText := "[ worker ]"
				roleStyled := badgeWorkerStyle.Render(roleText)
				if a.Role == "master" {
					roleText = "[★ master]"
					roleStyled = badgeMasterStyle.Render(roleText)
				}

				toolsBadge := badgeToolNo.Render("no")
				if a.ToolCalling {
					toolsBadge = badgeToolYes.Render("yes")
				}

				dispProv := a.Provider
				if dispProv == "claude_subscription" {
					dispProv = "claude_sub"
				}

				aliasCol := fmt.Sprintf("%-18s", alias)
				modelCol := fmt.Sprintf("%-32s", a.Model)
				provCol := fmt.Sprintf("%-16s", dispProv)

				if isCursor {
					colsRendered := lineStyle.Render(fmt.Sprintf("%s %s %s", aliasCol, modelCol, provCol))
					b.WriteString(prefix + colsRendered + " " + fmt.Sprintf("%-14s", roleStyled) + " " + toolsBadge + "\n")
				} else {
					b.WriteString(prefix + fmt.Sprintf("%s %s %s %-14s %s", aliasCol, modelCol, provCol, roleStyled, toolsBadge) + "\n")
				}
			}
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n\n")
	}

	var helpText string
	if m.activeTab == tabAgents {
		helpText = "[↑/↓] Move · [p] Ping · [m] Set Master · [w] Set Worker · [e] Edit · [d] Del · [Tab] Switch · [q] Quit"
	} else {
		helpText = "[↑/↓] Move · [p] Ping · [a] Add · [e] Edit · [d] Del · [s] Save · [Tab] Switch · [q] Quit"
	}
	help := hintStyle.Render(helpText)
	b.WriteString(help + "\n")

	return b.String()
}

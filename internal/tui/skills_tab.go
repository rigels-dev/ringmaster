package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"ringmaster/internal/config"
	"ringmaster/internal/skills"
)

// updateSkills handles the keys that mean something else on the Skills tab.
// Navigation and tab switching are left to updateList (handled = false).
func (m ConfigModel) updateSkills(msg tea.KeyMsg) (ConfigModel, bool) {
	switch msg.String() {
	case "enter", "e", " ":
		if m.cursor >= len(m.skills) {
			return m, true
		}
		name := m.skills[m.cursor].Name
		off := !skills.IsDisabled(name, m.cfg.Skills.Disabled)
		m.cfg.Skills.Disabled = skills.SetDisabled(m.cfg.Skills.Disabled, name, off)
		if err := config.SaveFile(m.configPath, m.cfg); err != nil {
			m.statusMsg = fmt.Sprintf("✗ Could not save: %v", err)
			return m, true
		}
		switch {
		case off:
			m.statusMsg = fmt.Sprintf("Skill '%s' is off: workers no longer get it", name)
		case skills.IsDisabled(name, m.projectDisabled):
			m.statusMsg = fmt.Sprintf("Skill '%s' is on for the fleet, but this project's ringmaster.toml still disables it", name)
		default:
			m.statusMsg = fmt.Sprintf("✓ Skill '%s' is on", name)
		}
		return m, true
	case "a", "d", "x", "m", "w", "p":
		// Provider and agent actions; skills are installed as folders, not here
		return m, true
	}
	return m, false
}

// viewSkills renders the Skills tab: every installed skill and whether workers get it.
func (m ConfigModel) viewSkills(b *strings.Builder) {
	if len(m.skills) == 0 {
		b.WriteString(hintStyle.Render("No skills installed yet.") + "\n\n")
	} else {
		plainHeader := fmt.Sprintf("  %-24s %-14s %-8s %s", "SKILL", "STATE", "SOURCE", "DESCRIPTION")
		b.WriteString(headerStyle.Render(plainHeader) + "\n")
		b.WriteString(tableBorderLine.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

		for i, s := range m.skills {
			prefix := "  "
			lineStyle := normalItemStyle
			if i == m.cursor {
				prefix = cursorIndicator.Render("▸ ")
				lineStyle = selectedItemStyle
			}

			state := "[x] on"
			stateStyle := badgeToolYes
			switch {
			case skills.IsDisabled(s.Name, m.cfg.Skills.Disabled):
				state, stateStyle = "[ ] off", badgeToolNo
			case skills.IsDisabled(s.Name, m.projectDisabled):
				state, stateStyle = "[ ] off (proj)", badgeToolNo
			}

			desc := s.Description
			if r := []rune(desc); len(r) > 44 {
				desc = string(r[:43]) + "…"
			}
			nameCol := lineStyle.Render(fmt.Sprintf("%-24s", s.Name))
			// Pad the state before styling it, so the columns line up
			stateCol := stateStyle.Render(fmt.Sprintf("%-14s", state))
			b.WriteString(prefix + nameCol + " " + stateCol + " " + fmt.Sprintf("%-8s %s", s.Source, desc) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(hintStyle.Render("A skill is a folder holding a SKILL.md. Install one by copying it into:") + "\n")
	b.WriteString(hintStyle.Render("  "+skills.GlobalDir()+"  (every project)") + "\n")
	b.WriteString(hintStyle.Render("  <project>/.ringmaster/skills  (one project)") + "\n")
}

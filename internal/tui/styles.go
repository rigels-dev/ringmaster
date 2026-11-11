package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Ringmaster Circus / Steampunk Orchestration Palette
	// Gear Gold: #DEAA3B, Coat Crimson: #B6432A, Cyan Energy: #29F2EC, Slate Deep: #192026, Parchment: #EFE4CE

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#B6432A", Dark: "#DEAA3B"})

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#192026")).
			Background(lipgloss.AdaptiveColor{Light: "#DEAA3B", Dark: "#DEAA3B"}).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"}).
				Background(lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#232B33"}).
				Padding(0, 2)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
			Bold(true)

	stepDoneBullet = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#29F2EC"}).
			Bold(true)

	stepActiveBullet = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#B6432A", Dark: "#E05436"}).
				Bold(true)

	stepTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#EFE4CE"}).
			Bold(true)

	stepDimTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"}).
				Bold(true)

	stepAnswerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#29F2EC"}).
			Bold(true)

	selectedBulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#DEAA3B", Dark: "#29F2EC"}).
				Bold(true)

	normalBulletStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#51606D"})

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#FFFFFF"}).
				Background(lipgloss.AdaptiveColor{Light: "#E2E8F0", Dark: "#2A3642"}).
				Bold(true)

	cursorIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#DEAA3B"}).
			Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#EFE4CE"})

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"}).
			Italic(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"}).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#29F2EC"}).
			Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#E05436"}).
			Bold(true)

	badgeMasterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
				Bold(true)

	badgeWorkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"})

	badgeToolYes = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0D9488", Dark: "#29F2EC"}).
			Bold(true)

	badgeToolNo = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#94A3B8", Dark: "#51606D"})

	tableBorderLine = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#2E3842"})
)

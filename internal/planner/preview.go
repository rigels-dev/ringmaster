package planner

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"})

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#2E3842"})

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#FFFFFF"})

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"})

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#E05436"})

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#29F2EC"})
)

// RenderPreview outputs a rich formatted terminal table of the DAG plan.
func RenderPreview(plan *schemas.Plan, estimates ProjectEstimate, budget config.BudgetConfig) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(fmt.Sprintf(" 🎪 RINGMASTER PLAN PREVIEW · Project: %s (%d tasks)", plan.Project, len(plan.Tasks))))
	sb.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("  %-6s %-26s %-10s %-12s %-16s %-18s", "ID", "TITLE", "TYPE", "DEPENDS ON", "WORKER", "TOKEN ESTIMATE")
	sb.WriteString(headerStyle.Render(header) + "\n")
	sb.WriteString(borderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

	estMap := make(map[string]TaskEstimate)
	for _, e := range estimates.Tasks {
		estMap[e.TaskID] = e
	}

	for _, t := range plan.Tasks {
		depStr := "-"
		if len(t.DependsOn) > 0 {
			depStr = strings.Join(t.DependsOn, ", ")
		}

		est := estMap[t.ID]
		estStr := fmt.Sprintf("%dk - %dk", est.Min/1000, est.Max/1000)

		row := fmt.Sprintf("  %-6s %-26s %-10s %-12s %-16s %-18s",
			t.ID,
			truncate(t.Title, 26),
			t.Type,
			truncate(depStr, 12),
			truncate(t.Worker, 16),
			estStr,
		)
		sb.WriteString(row + "\n")
	}

	sb.WriteString(borderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

	// Cumulative summary
	summary := fmt.Sprintf("  Cumulative Token Range: %dk – %dk tokens", estimates.TotalMin/1000, estimates.TotalMax/1000)
	sb.WriteString(successStyle.Render(summary) + "\n")

	if budget.MaxTokensPerProject > 0 {
		budgetLine := fmt.Sprintf("  Project Budget Limit:   %dk tokens", budget.MaxTokensPerProject/1000)
		sb.WriteString(dimStyle.Render(budgetLine) + "\n")

		if estimates.TotalMax > budget.MaxTokensPerProject {
			warn := fmt.Sprintf("  ⚠ WARNING: Maximum estimated tokens (%dk) exceeds project budget (%dk)!",
				estimates.TotalMax/1000, budget.MaxTokensPerProject/1000)
			sb.WriteString(warningStyle.Render(warn) + "\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

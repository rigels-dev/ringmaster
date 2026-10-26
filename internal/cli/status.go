package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"ringmaster/internal/planner"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

var (
	statusDoneStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#22C55E"})

	statusRunningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#EAB308"})

	statusBlockedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#F97316"})

	statusReviewStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#EF4444"})

	statusSkippedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#64748B"})

	statusPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"})

	headerStyleStatus = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"})

	boxBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#2E3842"})
)

var (
	statusWatch bool
	statusJSON  bool
)

var statusCmd = &cobra.Command{
	Use:   "status [project_name]",
	Short: "Display DAG visual execution tree, task status, and blockers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := "demo"
		if len(args) > 0 {
			projectName = args[0]
		}
		projectDir := filepath.Join("projects", projectName)

		if statusWatch {
			// Periodic loop
			for {
				output, isHalted, err := buildStatusView(projectDir, projectName, statusJSON)
				if err != nil {
					return err
				}
				// Clear screen
				fmt.Print("\033[H\033[2J")
				fmt.Print(output)
				if isHalted {
					break
				}
				time.Sleep(2 * time.Second)
			}
			return nil
		}

		output, _, err := buildStatusView(projectDir, projectName, statusJSON)
		if err != nil {
			return err
		}
		fmt.Print(output)
		return nil
	},
}

func buildStatusView(projectDir, projectName string, asJSON bool) (string, bool, error) {
	planPath := filepath.Join(projectDir, "plan.json")
	plan, err := planner.ReadPlanFile(planPath)
	if err != nil {
		return "", false, fmt.Errorf("project '%s' has no plan.json. Run 'ringmaster plan' first", projectName)
	}

	statePath := filepath.Join(projectDir, ".ringmaster", "state.json")
	state, err := storage.ReadJSON[schemas.ProjectState](statePath)
	if err != nil || state == nil {
		state = schemas.NewProjectState(projectName, "")
	}

	if asJSON {
		payload := map[string]any{
			"project":      projectName,
			"status":       state.Status,
			"plan_hash":    state.PlanHash,
			"tasks":        plan.Tasks,
			"task_states":  state.TaskStates,
			"planning":     state.Planning,
			"usage_tokens": calcTotalTokens(state),
			"updated_at":   state.UpdatedAt,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data) + "\n", state.Status == "COMPLETED" || state.Status == "HALTED", nil
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyleStatus.Render(fmt.Sprintf(" 🎪 RINGMASTER STATUS · Project: %s [State: %s]", projectName, state.Status)) + "\n\n")

	// Tasks Table Header
	header := fmt.Sprintf("  %-6s %-26s %-14s %-8s %-16s %-14s", "ID", "TITLE", "STATUS", "ATTEMPT", "WORKER", "TOKENS (IN/OUT)")
	sb.WriteString(headerStyleStatus.Render(header) + "\n")
	sb.WriteString(boxBorderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

	completedCount := 0
	var blockers []string
	var reviews []string

	for _, t := range plan.Tasks {
		ts, exists := state.TaskStates[t.ID]
		if !exists {
			ts = schemas.TaskState{Status: schemas.TaskStatePending}
		}

		statusBadge := formatStatusBadge(ts.Status)
		attemptStr := fmt.Sprintf("%d/%d", ts.Attempt, t.MaxRetries)
		tokensStr := fmt.Sprintf("%d / %d", ts.Usage.TokensIn, ts.Usage.TokensOut)

		if ts.Status == schemas.TaskStateCompleted {
			completedCount++
		} else if ts.Status == schemas.TaskStateBlocked {
			blockers = append(blockers, fmt.Sprintf("  • Task %s: %s\n    Action: ringmaster answer %s --text \"<your answer>\"", t.ID, ts.Error, t.ID))
		} else if ts.Status == schemas.TaskStateReview {
			reviews = append(reviews, fmt.Sprintf("  • Task %s: %s (Check outputs/%s.md)", t.ID, ts.Error, t.ID))
		}

		row := fmt.Sprintf("  %-6s %-26s %-14s %-8s %-16s %-14s",
			t.ID,
			truncate(t.Title, 26),
			statusBadge,
			attemptStr,
			truncate(t.Worker, 16),
			tokensStr,
		)
		sb.WriteString(row + "\n")
	}

	sb.WriteString(boxBorderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

	// Progress summary
	totalTokens := calcTotalTokens(state)
	progress := fmt.Sprintf("  Progress: %d/%d tasks completed · Total Tokens: %d", completedCount, len(plan.Tasks), totalTokens)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(progress) + "\n")

	// Actionable Blockers
	if len(blockers) > 0 {
		sb.WriteString("\n" + statusBlockedStyle.Render("  ⚠ ACTIONABLE BLOCKERS:") + "\n")
		for _, b := range blockers {
			sb.WriteString(b + "\n")
		}
	}

	// Needs Review
	if len(reviews) > 0 {
		sb.WriteString("\n" + statusReviewStyle.Render("  🔍 TASKS REQUIRING REVIEW:") + "\n")
		for _, r := range reviews {
			sb.WriteString(r + "\n")
		}
	}

	sb.WriteString("\n")
	isHalted := state.Status == "COMPLETED" || state.Status == "HALTED" || state.Status == "PAUSED"
	return sb.String(), isHalted, nil
}

func formatStatusBadge(status schemas.TaskExecutionStatus) string {
	switch status {
	case schemas.TaskStateCompleted:
		return statusDoneStyle.Render("✓ DONE")
	case schemas.TaskStateRunning:
		return statusRunningStyle.Render("▶ RUNNING")
	case schemas.TaskStateBlocked:
		return statusBlockedStyle.Render("⏸ BLOCKED")
	case schemas.TaskStateReview:
		return statusReviewStyle.Render("! REVIEW")
	case schemas.TaskStateFailed:
		return statusReviewStyle.Render("✗ FAILED")
	default:
		return statusPendingStyle.Render("○ PENDING")
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func calcTotalTokens(state *schemas.ProjectState) int {
	total := state.Planning.TokensTotal()
	for _, ts := range state.TaskStates {
		total += ts.Usage.TokensTotal()
	}
	return total
}

func init() {
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "Periodically refresh status view (every 2s)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output status as JSON")
	RootCmd.AddCommand(statusCmd)
}

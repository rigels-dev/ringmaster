package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"ringmaster/internal/config"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"ringmaster/internal/planner"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

var usageJSON bool

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Display accurate token usage breakdown per agent, role, and task vs estimates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		projectName := filepath.Base(projectDir)

		output, err := buildUsageView(projectDir, projectName, usageJSON)
		if err != nil {
			return err
		}
		fmt.Print(output)
		return nil
	},
}

func buildUsageView(projectDir, projectName string, asJSON bool) (string, error) {
	planPath := filepath.Join(projectDir, ".ringmaster", "plan.json")
	plan, err := planner.ReadPlanFile(planPath)
	if err != nil {
		return "", fmt.Errorf("project '%s' has no plan.json. Run 'ringmaster plan' first", projectName)
	}

	statePath := filepath.Join(projectDir, ".ringmaster", "state.json")
	state, err := storage.ReadJSON[schemas.ProjectState](statePath)
	if err != nil || state == nil {
		state = schemas.NewProjectState(projectName, "")
	}

	var estimateCfg map[string]config.EstimateConfig
	if cfg != nil {
		estimateCfg = cfg.Estimates
	}
	estimates := planner.CalculateEstimates(plan, estimateCfg)
	estMap := make(map[string]planner.TaskEstimate)
	for _, e := range estimates.Tasks {
		estMap[e.TaskID] = e
	}

	// Calculate per-agent totals
	agentUsage := make(map[string]schemas.Usage)
	for _, ts := range state.TaskStates {
		w := ts.Worker
		if w == "" {
			w = "worker"
		}
		u := agentUsage[w]
		u.Add(ts.Usage)
		agentUsage[w] = u
	}

	if asJSON {
		payload := map[string]any{
			"project":         projectName,
			"planning_usage":  state.Planning,
			"agent_usage":     agentUsage,
			"task_states":     state.TaskStates,
			"total_tokens":    calcTotalTokens(state),
			"estimated_range": fmt.Sprintf("%d - %d", estimates.TotalMin, estimates.TotalMax),
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data) + "\n", nil
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyleStatus.Render(fmt.Sprintf(" 🎪 RINGMASTER USAGE BREAKDOWN · Project: %s", projectName)) + "\n\n")

	// 1. Planning vs Execution Summary
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("  1. CONTROL PLANE & ROLE BREAKDOWN:") + "\n")
	sb.WriteString(fmt.Sprintf("     • Master Planning: %d calls · %d tokens in · %d tokens out (Total: %d)\n",
		state.Planning.Calls, state.Planning.TokensIn, state.Planning.TokensOut, state.Planning.TokensTotal()))

	for w, u := range agentUsage {
		sb.WriteString(fmt.Sprintf("     • Worker Agent '%s': %d calls · %d in · %d out (Total: %d)\n",
			w, u.Calls, u.TokensIn, u.TokensOut, u.TokensTotal()))
	}
	sb.WriteString("\n")

	// 2. Per-Task Actual vs Estimate Table
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("  2. PER-TASK ACCURACY (ACTUAL vs ESTIMATED TOKENS):") + "\n")
	header := fmt.Sprintf("  %-6s %-10s %-14s %-8s %-18s %-18s %-10s",
		"TASK", "TYPE", "WORKER", "ATTEMPTS", "ACTUAL TOKENS", "ESTIMATED RANGE", "RATIO")
	sb.WriteString(headerStyleStatus.Render(header) + "\n")
	sb.WriteString(boxBorderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")

	totalActual := 0
	for _, t := range plan.Tasks {
		ts := state.TaskStates[t.ID]
		est := estMap[t.ID]
		actualTok := ts.Usage.TokensTotal()
		totalActual += actualTok

		ratioStr := "-"
		midEstimate := (est.Min + est.Max) / 2
		if midEstimate > 0 && actualTok > 0 {
			ratio := float64(actualTok) / float64(midEstimate)
			ratioStr = fmt.Sprintf("%.2fx", ratio)
		}

		estRangeStr := fmt.Sprintf("%dk - %dk", est.Min/1000, est.Max/1000)
		actualStr := fmt.Sprintf("%d", actualTok)

		row := fmt.Sprintf("  %-6s %-10s %-14s %-8d %-18s %-18s %-10s",
			t.ID,
			t.Type,
			truncate(t.Worker, 14),
			ts.Attempt,
			actualStr,
			estRangeStr,
			ratioStr,
		)
		sb.WriteString(row + "\n")
	}

	sb.WriteString(boxBorderStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────") + "\n")
	sb.WriteString(fmt.Sprintf("  Grand Total Actual: %d tokens (Estimated: %dk – %dk)\n\n",
		totalActual+state.Planning.TokensTotal(), estimates.TotalMin/1000, estimates.TotalMax/1000))

	return sb.String(), nil
}

func init() {
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "Output usage breakdown as JSON")
	RootCmd.AddCommand(usageCmd)
}

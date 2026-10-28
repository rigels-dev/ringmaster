package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
	"ringmaster/internal/worker"
)

// TaskAttemptLoopResult records the final outcome after one or more attempts.
type TaskAttemptLoopResult struct {
	TaskID        string                `json:"task_id"`
	Decision      Decision              `json:"decision"`
	TotalAttempts int                   `json:"total_attempts"`
	TotalUsage    schemas.Usage         `json:"total_usage"`
	Duration      time.Duration         `json:"duration"`
	FinalReport   *schemas.WorkerReport `json:"final_report,omitempty"`
}

// AuditEvent represents an entry in .ringmaster/logs/events.jsonl
type AuditEvent struct {
	Timestamp string `json:"timestamp"`
	TaskID    string `json:"task_id"`
	Attempt   int    `json:"attempt"`
	Action    string `json:"action"` // ATTEMPT_STARTED, ATTEMPT_FINISHED, ACCEPTANCE_CHECKED, DECISION
	Payload   any    `json:"payload,omitempty"`
}

// RunTaskLoop executes attempts for task until DONE, BLOCKED, NEEDS_REVIEW, or PAUSED.
func RunTaskLoop(
	ctx context.Context,
	task schemas.TaskSpec,
	worktree string,
	agent config.AgentConfig,
	provider llm.Provider,
	budget config.BudgetConfig,
	sandboxCfg config.SandboxConfig,
	briefContent string,
	eventsPath string,
) (*TaskAttemptLoopResult, error) {
	start := time.Now()
	res := &TaskAttemptLoopResult{
		TaskID: task.ID,
	}

	maxRetries := task.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 2
	}

	appendEvent := func(attempt int, action string, payload any) {
		if eventsPath == "" {
			return
		}
		_ = os.MkdirAll(filepath.Dir(eventsPath), 0755)
		fd, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			defer fd.Close()
			ev := AuditEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				TaskID:    task.ID,
				Attempt:   attempt,
				Action:    action,
				Payload:   payload,
			}
			data, _ := json.Marshal(ev)
			_, _ = fd.WriteString(string(data) + "\n")
		}
	}

	attempt := 1
	var currentBrief = briefContent

	for {
		res.TotalAttempts = attempt

		appendEvent(attempt, "ATTEMPT_STARTED", map[string]any{
			"model": agent.Model,
		})

		attemptRes, err := worker.RunAttempt(
			ctx,
			task,
			worktree,
			agent,
			provider,
			budget,
			sandboxCfg,
			currentBrief,
			attempt,
		)
		if err != nil {
			return nil, fmt.Errorf("worker attempt execution failed: %w", err)
		}

		res.TotalUsage.Add(attemptRes.Usage)
		appendEvent(attempt, "ATTEMPT_FINISHED", map[string]any{
			"status": attemptRes.Status,
			"reason": attemptRes.Reason,
			"usage":  attemptRes.Usage,
		})

		// Check if budget was exceeded during attempt
		budgetExceeded := attemptRes.Reason == "BUDGET"

		var checks []CheckResult
		if attemptRes.Status == schemas.ReportStatusCompleted && !budgetExceeded {
			var checkErr error
			checks, checkErr = RunAcceptance(ctx, task, worktree, sandboxCfg.Allow)
			if checkErr != nil {
				return nil, fmt.Errorf("failed to run acceptance criteria: %w", checkErr)
			}
			appendEvent(attempt, "ACCEPTANCE_CHECKED", checks)
		}

		decision := Decide(attemptRes.Report, checks, attempt, maxRetries, budgetExceeded)
		res.Decision = decision
		res.FinalReport = attemptRes.Report

		appendEvent(attempt, "DECISION", decision)

		// Render verification log to outputs/Txx.md if report exists
		if attemptRes.Report != nil {
			var verLines []string
			for _, chk := range checks {
				passStr := "PASS"
				if !chk.Passed {
					passStr = fmt.Sprintf("FAIL (exit %d)", chk.ExitCode)
				}
				verLines = append(verLines, fmt.Sprintf("- `%s`: %s", chk.Command, passStr))
			}
			attemptRes.Report.Verification = strings.Join(verLines, "\n")
			rendered, rErr := attemptRes.Report.RenderMarkdown()
			if rErr == nil {
				outPath := filepath.Join(worktree, "outputs", fmt.Sprintf("%s.md", task.ID))
				_ = os.WriteFile(outPath, []byte(rendered), 0644)
			}
		}

		if decision.Type == DecisionRETRY {
			attempt++
			// Feed failure logs into prompt context for next attempt
			currentBrief = fmt.Sprintf("%s\n\n# PREVIOUS ATTEMPT FAILED (Attempt %d):\n%s\nPlease fix the issues above so acceptance criteria pass.",
				briefContent, attempt-1, decision.FailureLogs)
			continue
		}

		// Terminal states: DONE, BLOCKED, NEEDS_REVIEW, PAUSED
		break
	}

	res.Duration = time.Since(start)
	return res, nil
}

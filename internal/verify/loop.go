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
// restoreTests, when set, runs before each acceptance check: it undoes the
// worker's changes to existing test files and returns their paths, so the
// checks run against tests the worker could not weaken.
// afterAttempt, when set, runs once each attempt has been verified.
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
	restoreTests func(ctx context.Context) ([]string, error),
	afterAttempt func(attempt int, report *schemas.WorkerReport),
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
		var restored []string
		if attemptRes.Status == schemas.ReportStatusCompleted && !budgetExceeded {
			if restoreTests != nil {
				var restoreErr error
				if restored, restoreErr = restoreTests(ctx); restoreErr != nil {
					return nil, fmt.Errorf("failed to restore protected test files: %w", restoreErr)
				}
				if len(restored) > 0 {
					appendEvent(attempt, "TESTS_RESTORED", restored)
				}
			}
			var checkErr error
			checks, checkErr = RunAcceptance(ctx, task, worktree, sandboxCfg.Allow, sandboxCfg.PassEnv, acceptanceTimeout(task, sandboxCfg))
			if checkErr != nil {
				return nil, fmt.Errorf("failed to run acceptance criteria: %w", checkErr)
			}
			appendEvent(attempt, "ACCEPTANCE_CHECKED", checks)
		}

		decision := Decide(attemptRes.Report, checks, attempt, maxRetries, budgetExceeded)
		if len(restored) > 0 && decision.Type == DecisionRETRY {
			// The worker's next attempt must know why its test changes are gone
			decision.FailureLogs = restoredTestsNote(restored) + "\n\n" + decision.FailureLogs
		}
		res.Decision = decision
		res.FinalReport = attemptRes.Report

		appendEvent(attempt, "DECISION", decision)

		// Render verification log to .ringmaster/outputs/Txx.md if report exists
		if attemptRes.Report != nil {
			var verLines []string
			for _, chk := range checks {
				passStr := "PASS"
				if !chk.Passed {
					passStr = fmt.Sprintf("FAIL (exit %d)", chk.ExitCode)
				}
				verLines = append(verLines, fmt.Sprintf("- `%s`: %s", chk.Command, passStr))
			}
			if len(restored) > 0 {
				verLines = append(verLines, fmt.Sprintf("- Existing test files the worker changed were restored before these checks: %s", strings.Join(restored, ", ")))
			}
			attemptRes.Report.Verification = strings.Join(verLines, "\n")
			rendered, rErr := attemptRes.Report.RenderMarkdown()
			if rErr == nil {
				outPath := filepath.Join(worktree, ".ringmaster", "outputs", fmt.Sprintf("%s.md", task.ID))
				_ = os.WriteFile(outPath, []byte(rendered), 0644)
			}
		}

		// Let the caller record this attempt (the scheduler commits it) before a retry starts
		if afterAttempt != nil {
			afterAttempt(attempt, attemptRes.Report)
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

// restoredTestsNote tells a worker that its edits to existing tests were undone.
func restoredTestsNote(restored []string) string {
	return "Ringmaster restored these existing test files to their original content before running the acceptance checks, " +
		"because this task may not change or delete tests that already exist: " + strings.Join(restored, ", ") + ".\n" +
		"Make the code pass the tests as they are; you may add new test files. " +
		"If an existing test is itself wrong for this task, do not edit it again: report AWAITING_INPUT and ask the user."
}

// acceptanceTimeout is the task's own limit, else the sandbox's, else the default.
func acceptanceTimeout(task schemas.TaskSpec, sandboxCfg config.SandboxConfig) time.Duration {
	seconds := task.AcceptanceTimeout
	if seconds <= 0 {
		seconds = sandboxCfg.AcceptanceTimeout
	}
	if seconds <= 0 {
		seconds = config.DefaultAcceptanceTimeout
	}
	return time.Duration(seconds) * time.Second
}

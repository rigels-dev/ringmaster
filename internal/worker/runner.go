package worker

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
)

const (
	defaultMaxTurns = 30
)

// AttemptResult contains the summary and outcome of a worker attempt.
type AttemptResult struct {
	TaskID   string                `json:"task_id"`
	Status   schemas.ReportStatus  `json:"status"` // COMPLETED, BLOCKED, NEEDS_REVIEW
	Reason   string                `json:"reason"` // COMPLETED, MAX_TURNS, BUDGET, ERROR, TIMEOUT
	Usage    schemas.Usage         `json:"usage"`
	Report   *schemas.WorkerReport `json:"report,omitempty"`
	Turns    int                   `json:"turns"`
	Duration time.Duration         `json:"duration"`
	Worktree string                `json:"worktree"`
	LogsPath string                `json:"logs_path"`
}

// TranscriptEntry records one turn in the attempt transcript.
type TranscriptEntry struct {
	Turn      int            `json:"turn"`
	Timestamp string         `json:"timestamp"`
	Role      string         `json:"role"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolResp  string         `json:"tool_response,omitempty"`
}

// RunAttempt coordinates executing a task specification inside a worktree sandbox.
func RunAttempt(
	ctx context.Context,
	task schemas.TaskSpec,
	worktree string,
	agent config.AgentConfig,
	provider llm.Provider,
	budget config.BudgetConfig,
	sandboxCfg config.SandboxConfig,
	briefContent string,
	attemptNum int,
) (*AttemptResult, error) {
	start := time.Now()

	// Ensure log directory exists
	logsDir := filepath.Join(worktree, ".ringmaster", "logs")
	_ = os.MkdirAll(logsDir, 0755)
	logFile := filepath.Join(logsDir, fmt.Sprintf("%s-attempt%d.jsonl", task.ID, attemptNum))
	logFd, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if logFd != nil {
		defer logFd.Close()
	}

	recordLog := func(entry TranscriptEntry) {
		if logFd != nil {
			entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
			line, err := json.Marshal(entry)
			if err == nil {
				_, _ = logFd.WriteString(string(line) + "\n")
			}
		}
	}

	toolSet := NewToolSet(worktree, sandboxCfg.Allow)
	tools := toolSet.Definitions()

	humanAnswersSection := ""
	if task.HumanAnswers != "" {
		humanAnswersSection = fmt.Sprintf("\n# HUMAN ANSWERS & FEEDBACK:\n%s\n", task.HumanAnswers)
	}

	systemPrompt := fmt.Sprintf(`You are an autonomous AI software engineer working for Ringmaster.
Your role is to implement task '%s' within your isolated workspace.

# PROJECT BRIEF:
%s

# TASK SPECIFICATION:
ID: %s
Title: %s
Description:
%s
%s
# ACCEPTANCE CRITERIA:
- %s

# INSTRUCTIONS:
1. Explore the workspace with read_file, list_dir, search.
2. Implement required code and tests using write_file.
3. Validate your implementation using run_cmd (e.g. run test suites).
4. When done, write your final structured report to 'outputs/%s.md'.
The report must have YAML frontmatter:
---
status: COMPLETED
attempts: %d
calls: 0
tokens_in: 0
tokens_out: 0
files_changed: []
---

## Summary
Brief description of what was done.

## Verification
Test command and results.
`, task.ID, briefContent, task.ID, task.Title, task.Description, humanAnswersSection, strings.Join(task.Acceptance, "\n- "), task.ID, attemptNum)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Begin implementation for task %s. Write your final report to outputs/%s.md when finished.", task.ID, task.ID)},
	}

	res := &AttemptResult{
		TaskID:   task.ID,
		Worktree: worktree,
		LogsPath: logFile,
	}

	maxTokens := budget.MaxTokensPerTask
	if maxTokens == 0 {
		maxTokens = 300000
	}

	for turn := 1; turn <= defaultMaxTurns; turn++ {
		res.Turns = turn

		// Check context timeout
		if ctx.Err() != nil {
			res.Status = schemas.ReportStatusBlocked
			res.Reason = "TIMEOUT"
			break
		}

		// Check budget limit
		if res.Usage.TokensTotal() >= maxTokens {
			res.Status = schemas.ReportStatusNeedsReview
			res.Reason = "BUDGET"
			break
		}

		req := llm.ChatRequest{
			Messages: messages,
			Tools:    tools,
		}

		resp, err := provider.Chat(ctx, agent.Model, req)
		if err != nil {
			res.Status = schemas.ReportStatusBlocked
			res.Reason = fmt.Sprintf("LLM_ERROR: %v", err)
			break
		}

		res.Usage.Calls++
		res.Usage.TokensIn += resp.Usage.PromptTokens
		res.Usage.TokensOut += resp.Usage.CompletionTokens

		// Check budget limit after turn tokens accumulated
		if res.Usage.TokensTotal() >= maxTokens {
			res.Status = schemas.ReportStatusNeedsReview
			res.Reason = "BUDGET"
			break
		}

		recordLog(TranscriptEntry{
			Turn:      turn,
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Append assistant response to dialogue
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// If model produced no tool calls, it has finished speaking
		if len(resp.ToolCalls) == 0 {
			res.Reason = "COMPLETED"
			break
		}

		// Execute all requested tool calls
		for _, tc := range resp.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

			// Live CLI logging so UI stream can observe actual worker activity
			if tc.Function.Name == "run_cmd" {
				fmt.Printf("[%s] turn %d: exec `%v`\n", task.ID, turn, args["cmd"])
			} else if tc.Function.Name == "write_file" {
				fmt.Printf("[%s] turn %d: write `%v`\n", task.ID, turn, args["path"])
			} else if tc.Function.Name == "read_file" {
				fmt.Printf("[%s] turn %d: read `%v`\n", task.ID, turn, args["path"])
			} else {
				fmt.Printf("[%s] turn %d: call %s\n", task.ID, turn, tc.Function.Name)
			}

			out := toolSet.Execute(ctx, tc.Function.Name, args)

			recordLog(TranscriptEntry{
				Turn:     turn,
				Role:     "tool",
				ToolResp: out,
			})

			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    out,
			})
		}
	}

	if res.Reason == "" {
		res.Reason = "MAX_TURNS"
		res.Status = schemas.ReportStatusNeedsReview
	}

	res.Duration = time.Since(start)

	// Attempt to read and parse output report written by worker
	reportPath := filepath.Join(worktree, "outputs", fmt.Sprintf("%s.md", task.ID))
	reportContent, err := os.ReadFile(reportPath)
	if err == nil {
		report, parseErr := schemas.ParseReportMarkdown(string(reportContent))
		if parseErr == nil {
			res.Report = report
			res.Status = report.Status
			return res, nil
		}
	}

	// Fallback report if worker did not write a valid output report
	fallbackStatus := res.Status
	if fallbackStatus == "" {
		fallbackStatus = schemas.ReportStatusNeedsReview
	}
	res.Status = fallbackStatus
	res.Report = &schemas.WorkerReport{
		Status:    fallbackStatus,
		Attempts:  attemptNum,
		Calls:     res.Usage.Calls,
		TokensIn:  res.Usage.TokensIn,
		TokensOut: res.Usage.TokensOut,
		Summary:   fmt.Sprintf("Worker attempt ended with reason: %s. Report was generated by runner fallback.", res.Reason),
	}

	return res, nil
}

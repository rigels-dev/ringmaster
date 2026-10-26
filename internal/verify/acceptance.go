package verify

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/worker"
)

const (
	maxTailBytes       = 4 * 1024 // 4 KB truncation
	maxAcceptanceWait  = 90 * time.Second
)

// CheckResult represents the outcome of running a single acceptance criterion command.
type CheckResult struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	Passed     bool   `json:"passed"`
	StdoutTail string `json:"stdout_tail,omitempty"`
	StderrTail string `json:"stderr_tail,omitempty"`
}

// RunAcceptance runs each command in task.Acceptance independently inside worktree.
// All commands must exit with code 0 to pass.
func RunAcceptance(ctx context.Context, task schemas.TaskSpec, worktree string, allowlist []string) ([]CheckResult, error) {
	var results []CheckResult

	for _, cmdStr := range task.Acceptance {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}

		// Ensure command is permitted by allowlist (or allow if allowlist is empty)
		if len(allowlist) > 0 && !worker.CheckCommand(cmdStr, allowlist) {
			results = append(results, CheckResult{
				Command:    cmdStr,
				ExitCode:   126,
				Passed:     false,
				StderrTail: "Command not in sandbox allowlist",
			})
			continue
		}

		fields := strings.Fields(cmdStr)
		c := fields[0]
		args := fields[1:]

		tCtx, cancel := context.WithTimeout(ctx, maxAcceptanceWait)
		cmd := exec.CommandContext(tCtx, c, args...)
		cmd.Dir = worktree
		cmd.Env = worker.ScrubbedEnv()

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		cancel()

		exitCode := 0
		passed := true
		if err != nil {
			passed = false
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		outTail := stdout.String()
		if len(outTail) > maxTailBytes {
			outTail = outTail[len(outTail)-maxTailBytes:]
		}

		errTail := stderr.String()
		if len(errTail) > maxTailBytes {
			errTail = errTail[len(errTail)-maxTailBytes:]
		}

		results = append(results, CheckResult{
			Command:    cmdStr,
			ExitCode:   exitCode,
			Passed:     passed,
			StdoutTail: strings.TrimSpace(outTail),
			StderrTail: strings.TrimSpace(errTail),
		})
	}

	return results, nil
}

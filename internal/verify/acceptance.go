package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/worker"
)

const maxTailBytes = 4 * 1024 // 4 KB truncation

// CheckResult represents the outcome of running a single acceptance criterion command.
type CheckResult struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	Passed     bool   `json:"passed"`
	StdoutTail string `json:"stdout_tail,omitempty"`
	StderrTail string `json:"stderr_tail,omitempty"`
	// Unrunnable marks a criterion that can never pass as written, so retrying the worker is pointless.
	Unrunnable bool `json:"unrunnable,omitempty"`
	// TimedOut marks a command stopped at the acceptance timeout; more attempts will not fix that.
	TimedOut bool `json:"timed_out,omitempty"`
}

// shellSyntax lists constructs that need a shell; acceptance commands are exec'd directly.
var shellSyntax = []string{"$(", "`", "|", "&", ";", ">", "<", "\"", "'"}

func hasShellSyntax(cmd string) bool {
	for _, s := range shellSyntax {
		if strings.Contains(cmd, s) {
			return true
		}
	}
	return false
}

// RunAcceptance runs each command in task.Acceptance independently inside worktree,
// each limited to timeout. All commands must exit with code 0 to pass.
// They get the same environment as worker commands (worker.CommandEnv).
func RunAcceptance(ctx context.Context, task schemas.TaskSpec, worktree string, allowlist, passEnv []string, timeout time.Duration) ([]CheckResult, error) {
	var results []CheckResult

	for _, cmdStr := range task.Acceptance {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}

		if hasShellSyntax(cmdStr) {
			results = append(results, CheckResult{
				Command:    cmdStr,
				ExitCode:   2,
				Passed:     false,
				Unrunnable: true,
				StderrTail: "Shell syntax ($(), backticks, pipes, &, ;, redirects, quotes) is not supported: acceptance commands run without a shell. Split it into plain commands in the plan.",
			})
			continue
		}

		// Ensure command is permitted by allowlist (or allow if allowlist is empty)
		if len(allowlist) > 0 && !worker.CheckCommand(cmdStr, allowlist) {
			results = append(results, CheckResult{
				Command:    cmdStr,
				ExitCode:   126,
				Passed:     false,
				Unrunnable: true,
				StderrTail: "Command not in sandbox allowlist",
			})
			continue
		}

		fields := strings.Fields(cmdStr)
		c := fields[0]
		args := fields[1:]

		tCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(tCtx, c, args...)
		cmd.Dir = worktree
		cmd.Env = worker.CommandEnv(passEnv)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		timedOut := errors.Is(tCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
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

		errTail = strings.TrimSpace(errTail)
		if timedOut {
			passed = false
			errTail = strings.TrimSpace(errTail + fmt.Sprintf("\n[ringmaster] stopped after %s (acceptance timeout)", timeout))
		}

		results = append(results, CheckResult{
			Command:    cmdStr,
			ExitCode:   exitCode,
			Passed:     passed,
			StdoutTail: strings.TrimSpace(outTail),
			StderrTail: errTail,
			TimedOut:   timedOut,
		})
	}

	return results, nil
}

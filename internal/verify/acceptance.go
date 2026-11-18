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

// UnrunnableReason says why Ringmaster can never run cmd as an acceptance
// command, or "" when it can. Acceptance commands are executed directly,
// without a shell: quotes keep an argument together, but pipes, &&, ;,
// redirects and substitutions cannot work, so such a command would fail
// however good the work is. The planner and the scheduler ask this before
// any work is done.
func UnrunnableReason(cmd string) string {
	args, err := worker.SplitCommand(cmd)
	if err != nil {
		return "Acceptance commands run without a shell: " + err.Error() + "."
	}
	if why := worker.UnquotedInlineCode(args); why != "" {
		return "Acceptance commands run without a shell: " + why + "."
	}
	return ""
}

// MissingProgram says, when the program an acceptance command starts is not
// installed here (not on PATH), what to do about it, or "" when it is found.
// No attempt can make such a check pass, so it is asked before any work.
func MissingProgram(cmd string) string {
	args, err := worker.SplitCommand(cmd)
	if err != nil || len(args) == 0 {
		return ""
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return notInstalledNote(args[0])
	}
	return ""
}

func notInstalledNote(program string) string {
	return fmt.Sprintf("[ringmaster] '%s' is not installed on this machine, or is not on PATH, so this check cannot run. "+
		"Install it, or change the task's acceptance commands.", program)
}

// notFoundOutput recognises a program that is on PATH but only reports it is
// not available, like Docker Desktop's stand-in for docker in a WSL distro
// without WSL integration, and a shell's "command not found".
func notFoundOutput(exitCode int, output string) bool {
	if exitCode == 127 {
		return true
	}
	lower := strings.ToLower(output)
	return strings.Contains(lower, "could not be found in this wsl") ||
		strings.Contains(lower, "command not found")
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

		if reason := UnrunnableReason(cmdStr); reason != "" {
			results = append(results, CheckResult{
				Command:    cmdStr,
				ExitCode:   2,
				Passed:     false,
				Unrunnable: true,
				StderrTail: reason,
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
				StderrTail: "The command is not on the command allowlist. Allow it with: ringmaster answer <task> --allow \"" + cmdStr + "\", or add it to [sandbox] allow in ringmaster.toml.",
			})
			continue
		}

		// UnrunnableReason above has already accepted the command
		fields, _ := worker.SplitCommand(cmdStr)
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
		missing := false
		if err != nil {
			passed = false
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
				missing = errors.Is(err, exec.ErrNotFound)
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
		// A program that is not installed fails the same way on every attempt
		if !passed && !timedOut && (missing || notFoundOutput(exitCode, stdout.String()+"\n"+stderr.String())) {
			results = append(results, CheckResult{
				Command:    cmdStr,
				ExitCode:   exitCode,
				Passed:     false,
				Unrunnable: true,
				StdoutTail: strings.TrimSpace(outTail),
				StderrTail: strings.TrimSpace(notInstalledNote(fields[0]) + "\n" + errTail),
			})
			continue
		}
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

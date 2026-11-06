package verify

import (
	"fmt"
	"strings"

	"ringmaster/internal/schemas"
)

type DecisionType string

const (
	DecisionDONE        DecisionType = "DONE"
	DecisionRETRY       DecisionType = "RETRY"
	DecisionBLOCKED     DecisionType = "BLOCKED"
	DecisionNEED_REVIEW DecisionType = "NEEDS_REVIEW"
	DecisionPAUSED      DecisionType = "PAUSED"
	// DecisionAWAITING_INPUT parks the task until the user answers its requests.
	DecisionAWAITING_INPUT DecisionType = "AWAITING_INPUT"
)

// Decision encapsulates the outcome of verifying a worker report and checks.
type Decision struct {
	Type        DecisionType           `json:"type"`
	Reason      string                 `json:"reason"`
	FailureLogs string                 `json:"failure_logs,omitempty"`
	Requests    []schemas.HumanRequest `json:"requests,omitempty"`
}

// Decide is a pure, side-effect-free function evaluating task attempt results.
func Decide(
	report *schemas.WorkerReport,
	checks []CheckResult,
	attempt int,
	maxRetries int,
	budgetExceeded bool,
) Decision {
	if budgetExceeded {
		return Decision{
			Type:   DecisionPAUSED,
			Reason: "Task budget exceeded configured token limit. Execution paused for human intervention.",
		}
	}

	if report == nil {
		if attempt <= maxRetries {
			return Decision{
				Type:        DecisionRETRY,
				Reason:      "Worker produced no report.",
				FailureLogs: "Missing output report file. Please generate .ringmaster/outputs/Txx.md before completing.",
			}
		}
		return Decision{
			Type:   DecisionNEED_REVIEW,
			Reason: "Worker produced no report and max retries exhausted.",
		}
	}

	if report.Status == schemas.ReportStatusBlocked {
		return Decision{
			Type:   DecisionBLOCKED,
			Reason: fmt.Sprintf("Worker reported BLOCKED: %s", report.Blockers),
		}
	}

	if report.Status == schemas.ReportStatusAwaitingInput {
		return Decision{
			Type:     DecisionAWAITING_INPUT,
			Reason:   fmt.Sprintf("Worker needs your input: %d request(s).", len(report.Requests)),
			Requests: report.Requests,
		}
	}

	if report.Status == schemas.ReportStatusNeedsReview {
		reason := "Worker requested human review."
		if summary := strings.TrimSpace(report.Summary); summary != "" {
			reason += " " + summary
		}
		return Decision{
			Type:   DecisionNEED_REVIEW,
			Reason: reason,
		}
	}

	// A criterion that can never pass as written needs a plan fix, not another attempt
	for _, chk := range checks {
		if chk.Unrunnable {
			return Decision{
				Type:        DecisionNEED_REVIEW,
				Reason:      fmt.Sprintf("Acceptance command cannot run as written: '%s'. Fix it in the plan.", chk.Command),
				FailureLogs: chk.StderrTail,
			}
		}
	}

	// A check stopped at the time limit fails the same way on every attempt
	for _, chk := range checks {
		if chk.TimedOut {
			return Decision{
				Type: DecisionNEED_REVIEW,
				Reason: fmt.Sprintf("Acceptance command '%s' ran past its time limit. Raise acceptance_timeout for this task "+
					"(in the plan) or [sandbox] acceptance_timeout in ringmaster.toml, then resume.", chk.Command),
				FailureLogs: chk.StderrTail,
			}
		}
	}

	// For COMPLETED, verify acceptance checks
	allChecksPassed := true
	var failedLogs []string

	for _, chk := range checks {
		if !chk.Passed {
			allChecksPassed = false
			logEntry := fmt.Sprintf("Command failed: '%s' (exit %d)", chk.Command, chk.ExitCode)
			if chk.StderrTail != "" {
				logEntry += fmt.Sprintf("\nStderr:\n%s", chk.StderrTail)
			}
			if chk.StdoutTail != "" {
				logEntry += fmt.Sprintf("\nStdout:\n%s", chk.StdoutTail)
			}
			failedLogs = append(failedLogs, logEntry)
		}
	}

	if allChecksPassed && len(checks) > 0 {
		return Decision{
			Type:   DecisionDONE,
			Reason: "Worker reported COMPLETED and all acceptance criteria passed.",
		}
	}

	// Some checks failed
	failuresCombined := strings.Join(failedLogs, "\n\n")

	if attempt <= maxRetries {
		return Decision{
			Type:        DecisionRETRY,
			Reason:      fmt.Sprintf("Acceptance checks failed on attempt %d. Retrying...", attempt),
			FailureLogs: failuresCombined,
		}
	}

	return Decision{
		Type:        DecisionNEED_REVIEW,
		Reason:      fmt.Sprintf("Acceptance checks failed and max retries (%d) exhausted.", maxRetries),
		FailureLogs: failuresCombined,
	}
}

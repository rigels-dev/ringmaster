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
)

// Decision encapsulates the outcome of verifying a worker report and checks.
type Decision struct {
	Type        DecisionType `json:"type"`
	Reason      string       `json:"reason"`
	FailureLogs string       `json:"failure_logs,omitempty"`
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
				FailureLogs: "Missing output report file. Please generate outputs/Txx.md before completing.",
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

	if report.Status == schemas.ReportStatusNeedsReview {
		return Decision{
			Type:   DecisionNEED_REVIEW,
			Reason: "Worker requested human review.",
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

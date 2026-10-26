package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
)

func TestDecidePure(t *testing.T) {
	// 1. Budget exceeded -> PAUSED
	d := Decide(nil, nil, 1, 2, true)
	if d.Type != DecisionPAUSED {
		t.Errorf("expected PAUSED, got %v", d.Type)
	}

	// 2. Report BLOCKED -> BLOCKED
	blockedReport := &schemas.WorkerReport{
		Status:   schemas.ReportStatusBlocked,
		Blockers: "Missing database credentials",
	}
	d = Decide(blockedReport, nil, 1, 2, false)
	if d.Type != DecisionBLOCKED {
		t.Errorf("expected BLOCKED, got %v", d.Type)
	}

	// 3. Report NEEDS_REVIEW -> NEEDS_REVIEW
	reviewReport := &schemas.WorkerReport{
		Status:  schemas.ReportStatusNeedsReview,
		Summary: "Requires architecture decision",
	}
	d = Decide(reviewReport, nil, 1, 2, false)
	if d.Type != DecisionNEED_REVIEW {
		t.Errorf("expected NEEDS_REVIEW, got %v", d.Type)
	}

	// 4. Completed & checks pass -> DONE
	completedReport := &schemas.WorkerReport{
		Status: schemas.ReportStatusCompleted,
	}
	passChecks := []CheckResult{
		{Command: "go test", ExitCode: 0, Passed: true},
	}
	d = Decide(completedReport, passChecks, 1, 2, false)
	if d.Type != DecisionDONE {
		t.Errorf("expected DONE, got %v", d.Type)
	}

	// 5. Checks fail & attempt <= maxRetries -> RETRY
	failChecks := []CheckResult{
		{Command: "go test", ExitCode: 1, Passed: false, StderrTail: "tests failed"},
	}
	d = Decide(completedReport, failChecks, 1, 2, false)
	if d.Type != DecisionRETRY {
		t.Errorf("expected RETRY on attempt 1/2, got %v", d.Type)
	}

	// 6. Checks fail & attempt > maxRetries -> NEEDS_REVIEW
	d = Decide(completedReport, failChecks, 3, 2, false)
	if d.Type != DecisionNEED_REVIEW {
		t.Errorf("expected NEEDS_REVIEW when retries exhausted, got %v", d.Type)
	}
}

func TestAcceptanceRun(t *testing.T) {
	tempDir := t.TempDir()
	task := schemas.TaskSpec{
		ID:         "T01",
		Acceptance: []string{"echo pass_test", "false"},
	}

	results, err := RunAcceptance(context.Background(), task, tempDir, []string{"echo", "false"})
	if err != nil {
		t.Fatalf("RunAcceptance error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !results[0].Passed || results[0].ExitCode != 0 {
		t.Errorf("expected echo to pass, got %+v", results[0])
	}

	if results[1].Passed || results[1].ExitCode == 0 {
		t.Errorf("expected false to fail, got %+v", results[1])
	}
}

func TestRunTaskLoopSuccessAttempt1(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("MOCK_API_KEY", "dummy")

	reportYAML := "---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 50\ntokens_out: 20\nfiles_changed: []\n---\n\n## Summary\nAll done"

	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "write_file",
									"arguments": fmt.Sprintf(`{"path":"outputs/T01.md","content":%q}`, reportYAML),
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     50,
				"completion_tokens": 20,
				"total_tokens":      70,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tsMock.Close()

	provider := llm.NewProvider("openai", tsMock.URL, "MOCK_API_KEY")
	task := schemas.TaskSpec{
		ID:         "T01",
		Title:      "Quick Task",
		Type:       schemas.TaskTypeBackend,
		Acceptance: []string{"echo check_ok"},
		MaxRetries: 2,
	}

	agent := config.AgentConfig{Model: "mock", Role: "worker"}
	budget := config.BudgetConfig{MaxTokensPerTask: 10000}
	sandboxCfg := config.SandboxConfig{Allow: []string{"echo"}}
	eventsPath := filepath.Join(tempDir, ".ringmaster", "logs", "events.jsonl")

	res, err := RunTaskLoop(context.Background(), task, tempDir, agent, provider, budget, sandboxCfg, "Demo Brief", eventsPath)
	if err != nil {
		t.Fatalf("RunTaskLoop error: %v", err)
	}

	if res.Decision.Type != DecisionDONE {
		t.Errorf("expected DONE, got %v", res.Decision.Type)
	}
	if res.TotalAttempts != 1 {
		t.Errorf("expected 1 attempt, got %d", res.TotalAttempts)
	}

	// Verify events log was recorded
	eventsData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("events log missing: %v", err)
	}
	if len(eventsData) == 0 {
		t.Errorf("events log was empty")
	}
}

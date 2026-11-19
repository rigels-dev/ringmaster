package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
	"ringmaster/internal/verify"
)

// The loop seen in the shorty scenario: an automaton barred from changing an
// outdated test asked to "approve" an instruction in words, the user approved
// it, and it asked again. Now that request is a question, and the user is
// offered what only Ringmaster can do: let the task change the test.
func TestStopOverAProtectedTestOffersToAllowTestEdits(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")
	engine, err := NewEngine(projectRoot, cfg, 1)
	if err != nil {
		t.Fatal(err)
	}

	sentence := "remove the {GET, /api/links} case from TestE2EWrongMethodIsNot2xx in internal/httpapi/e2e_test.go (outside this run), then re-run T07"
	res := &verify.TaskAttemptLoopResult{Decision: verify.Decision{
		Type:     verify.DecisionAWAITING_INPUT,
		Requests: []schemas.HumanRequest{{ID: "r1", Type: schemas.RequestApproval, Command: sentence}},
	}}
	backend := schemas.TaskSpec{ID: "T01", Type: schemas.TaskTypeBackend}

	got := engine.checkStopRequests("T01", backend, config.SandboxConfig{}, res)
	if len(got) != 2 || got[0].Type != schemas.RequestInput || got[1].ID != verify.TestEditsID ||
		!strings.Contains(got[1].Question, "internal/httpapi/e2e_test.go") {
		t.Fatalf("requests: %+v", got)
	}

	// A task that may change tests already is not asked
	for name, sandbox := range map[string]config.SandboxConfig{"allow_test_edits": {AllowTestEdits: true}} {
		if got := engine.checkStopRequests("T01", backend, sandbox, res); len(got) != 1 {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := engine.checkStopRequests("T01", schemas.TaskSpec{ID: "T01", Type: schemas.TaskTypeTest}, config.SandboxConfig{}, res); len(got) != 1 {
		t.Errorf("a test task: %+v", got)
	}
	// Nor one that stopped over something else
	other := &verify.TaskAttemptLoopResult{Decision: verify.Decision{Requests: []schemas.HumanRequest{{ID: "r1", Type: schemas.RequestInput, Question: "Which port?"}}}}
	if got := engine.checkStopRequests("T01", backend, config.SandboxConfig{}, other); len(got) != 1 {
		t.Errorf("an unrelated stop: %+v", got)
	}
	// Test files the automaton changed and Ringmaster restored count too
	other.RestoredTests = []string{"internal/store/memory_test.go"}
	if got := engine.checkStopRequests("T01", backend, config.SandboxConfig{}, other); len(got) != 2 || !strings.Contains(got[1].Question, "memory_test.go") {
		t.Errorf("restored tests: %+v", got)
	}

	// Choosing it lets the task change tests from now on, and tells the automaton
	_ = engine.stateMgr.UpdateTaskState("T01", func(ts *schemas.TaskState) {
		ts.Status = schemas.TaskStateAwaiting
		ts.Requests = got
	})
	ans := schemas.HumanAnswer{TaskID: "T01", Replies: map[string]string{
		"r1":               "Drop the GET /api/links case: it is outdated.",
		verify.TestEditsID: verify.OptionAllowTestEdits,
	}}
	if err := engine.applyAnswer(engine.stateMgr.GetState().TaskStates["T01"], ans); err != nil {
		t.Fatal(err)
	}
	ts := engine.stateMgr.GetState().TaskStates["T01"]
	if !ts.EditsTests || ts.Status != schemas.TaskStatePending {
		t.Fatalf("state after the answer: %+v", ts)
	}
	spec, err := os.ReadFile(filepath.Join(projectRoot, ".ringmaster", "tasks", "T01-task-one.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spec), "lets this task change existing test files") {
		t.Errorf("the automaton is not told:\n%s", spec)
	}
}

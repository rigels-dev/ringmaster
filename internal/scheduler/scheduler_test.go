package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ringmaster/internal/config"
	"ringmaster/internal/planner"
	"ringmaster/internal/schemas"
	"ringmaster/internal/workspace"
)

func setupTestProject(t *testing.T) (string, *config.Config) {
	t.Helper()
	projectRoot := t.TempDir()

	// 1. Setup git repo
	repoDir := projectRoot
	_ = os.MkdirAll(filepath.Join(projectRoot, ".ringmaster"), 0755)
	ctx := context.Background()
	_, _ = workspace.RunGit(ctx, repoDir, "init", "-b", "main")
	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test Repo\n"), 0644)
	_, _ = workspace.RunGit(ctx, repoDir, "add", "README.md")
	_, _ = workspace.RunGit(ctx, repoDir, "commit", "-m", "init")

	// 2. Setup brief
	_ = os.WriteFile(filepath.Join(projectRoot, ".ringmaster", "BRIEF.md"), []byte("# Test Project Brief\n"), 0644)

	// 3. Setup tasks
	tasksDir := filepath.Join(projectRoot, ".ringmaster", "tasks")
	_ = os.MkdirAll(tasksDir, 0755)

	t1Content := `---
id: T01
title: "Task One"
type: backend
depends_on: []
worker: "test_worker"
max_retries: 2
acceptance:
  - "echo task_one_passed"
---
Implement task one.
`
	_ = os.WriteFile(filepath.Join(tasksDir, "T01-task-one.md"), []byte(t1Content), 0644)

	t2Content := `---
id: T02
title: "Task Two"
type: backend
depends_on: ["T01"]
worker: "test_worker"
max_retries: 2
acceptance:
  - "echo task_two_passed"
---
Implement task two.
`
	_ = os.WriteFile(filepath.Join(tasksDir, "T02-task-two.md"), []byte(t2Content), 0644)

	// 4. Setup Plan & Approval
	plan := &schemas.Plan{
		Project: filepath.Base(projectRoot),
		Tasks: []schemas.PlanTask{
			{
				ID:         "T01",
				Title:      "Task One",
				Type:       schemas.TaskTypeBackend,
				DependsOn:  []string{},
				Worker:     "test_worker",
				MaxRetries: 2,
				Acceptance: []string{"echo task_one_passed"},
			},
			{
				ID:         "T02",
				Title:      "Task Two",
				Type:       schemas.TaskTypeBackend,
				DependsOn:  []string{"T01"},
				Worker:     "test_worker",
				MaxRetries: 2,
				Acceptance: []string{"echo task_two_passed"},
			},
		},
	}

	planPath := filepath.Join(projectRoot, ".ringmaster", "plan.json")
	planData, _ := json.Marshal(plan)
	_ = os.WriteFile(planPath, planData, 0644)

	_, _ = planner.ApprovePlan(projectRoot, plan)

	cfg := config.DefaultConfig()
	cfg.Sandbox.Allow = []string{"echo"}
	cfg.Agents["test_worker"] = config.AgentConfig{
		Provider: "mock_prov",
		Model:    "mock-model",
		Role:     "worker",
	}

	return projectRoot, cfg
}

func TestSchedulerDAGExecution(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	// Setup mock LLM server that successfully generates output reports
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Determine which task is asking
		messages, _ := body["messages"].([]any)
		tID := "T01"
		for _, m := range messages {
			mMap, _ := m.(map[string]any)
			content, _ := mMap["content"].(string)
			if strings.Contains(content, "T02") {
				tID = "T02"
				break
			}
		}

		reportYAML := fmt.Sprintf("---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 50\ntokens_out: 20\nfiles_changed: []\n---\n\n## Summary\nFinished %s", tID)

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_0",
								"type": "function",
								"function": map[string]any{
									"name":      "write_file",
									"arguments": fmt.Sprintf(`{"path":"%s.txt","content":"done"}`, strings.ToLower(tID)),
								},
							},
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "write_file",
									"arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, tID, reportYAML),
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

	cfg.Providers["mock_prov"] = config.ProviderConfig{
		Type:      "openai",
		BaseURL:   tsMock.URL,
		APIKeyEnv: "MOCK_KEY",
	}

	engine, err := NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err = engine.Run(ctx)
	if err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	// Verify both tasks completed and merged
	state := engine.stateMgr.GetState()
	if state.Status != "COMPLETED" {
		t.Errorf("expected project COMPLETED, got: %s", state.Status)
	}

	if state.TaskStates["T01"].Status != schemas.TaskStateCompleted {
		t.Errorf("expected T01 COMPLETED, got: %s (err: %s)", state.TaskStates["T01"].Status, state.TaskStates["T01"].Error)
	}
	if state.TaskStates["T02"].Status != schemas.TaskStateCompleted {
		t.Errorf("expected T02 COMPLETED, got: %s (err: %s)", state.TaskStates["T02"].Status, state.TaskStates["T02"].Error)
	}

	// Verify repo has merge commits on main
	out, _ := workspace.RunGit(ctx, projectRoot, "log", "--oneline")
	if !strings.Contains(out, "Merge task T01") || !strings.Contains(out, "Merge task T02") {
		t.Errorf("git repo missing merge commits on main:\n%s", out)
	}

	// Ringmaster state (reports, transcripts) must never be committed to main
	tracked, _ := workspace.RunGit(ctx, projectRoot, "ls-files")
	if strings.Contains(tracked, ".ringmaster") {
		t.Errorf("main branch tracks Ringmaster state files:\n%s", tracked)
	}

	// Each attempt is committed on the task branch, tagged with its automaton
	branchLog, _ := workspace.RunGit(ctx, projectRoot, "log", "main..rm/T01", "--format=%s|%(trailers:key=Ringmaster-Task,valueonly,separator=)|%(trailers:key=Ringmaster-Automaton,valueonly,separator=)|%(trailers:key=Ringmaster-Attempt,valueonly,separator=)")
	if strings.TrimSpace(branchLog) != "" {
		t.Errorf("rm/T01 should be fully merged, unmerged commits:\n%s", branchLog)
	}
	attemptLog, _ := workspace.RunGit(ctx, projectRoot, "log", "-1", "rm/T01", "--format=%s|%(trailers:key=Ringmaster-Task,valueonly,separator=)|%(trailers:key=Ringmaster-Automaton,valueonly,separator=)|%(trailers:key=Ringmaster-Attempt,valueonly,separator=)")
	if got := strings.TrimSpace(attemptLog); got != "T01: Finished T01|T01|test_worker|1" {
		t.Errorf("attempt commit = %q, want subject from the report and task/automaton/attempt trailers", got)
	}
	mergeLog, _ := workspace.RunGit(ctx, projectRoot, "log", "--merges", "--grep=Merge task T01", "--format=%(trailers:key=Ringmaster-Task,valueonly,separator=)")
	if strings.TrimSpace(mergeLog) != "T01" {
		t.Errorf("merge commit missing Ringmaster-Task trailer: %q", mergeLog)
	}

	// Merged tasks' worktrees are gone, but their report and transcripts are kept
	for _, id := range []string{"T01", "T02"} {
		if _, err := os.Stat(filepath.Join(workspace.WorktreesDir(projectRoot), id)); err == nil {
			t.Errorf("worktree for %s should be removed after merge", id)
		}
		for _, f := range []string{
			filepath.Join(projectRoot, ".ringmaster", "outputs", id+".md"),
			filepath.Join(projectRoot, ".ringmaster", "logs", id+"-attempt1.jsonl"),
		} {
			if _, err := os.Stat(f); err != nil {
				t.Errorf("expected archived %s", f)
			}
		}
	}
}

func TestSchedulerPausedTaskIsNotRedispatched(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	var calls int
	var mu sync.Mutex
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "working"}}},
			"usage":   map[string]any{"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70},
		})
	}))
	defer tsMock.Close()

	cfg.Providers["mock_prov"] = config.ProviderConfig{Type: "openai", BaseURL: tsMock.URL, APIKeyEnv: "MOCK_KEY"}
	// Every attempt blows the per-task budget on its first call
	cfg.Budget.MaxTokensPerTask = 10

	engine, err := NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := engine.Run(ctx); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	state := engine.stateMgr.GetState()
	if got := state.TaskStates["T01"].Status; got != schemas.TaskStatePaused {
		t.Errorf("expected T01 PAUSED, got %s", got)
	}
	if state.Status != "PAUSED" {
		t.Errorf("expected project PAUSED, got %s", state.Status)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 LLM call (no re-dispatch loop), got %d", calls)
	}
}

func TestSchedulerAwaitingInputAndAnswer(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	// T01 asks for approval until the approval shows up in its human answers
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		usage := map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10}
		if body.Messages[len(body.Messages)-1].Role == "tool" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "done"}}},
				"usage":   usage,
			})
			return
		}

		system := body.Messages[0].Content
		tID := "T01"
		if strings.Contains(system, "ID: T02") {
			tID = "T02"
		}
		report := fmt.Sprintf("---\nstatus: COMPLETED\nattempts: 1\nfiles_changed: []\n---\n\n## Summary\nFinished %s", tID)
		if tID == "T01" && !strings.Contains(system, "Command `make lint` → approve") {
			report = "---\nstatus: AWAITING_INPUT\nattempts: 1\nfiles_changed: []\nrequests:\n  - id: r1\n    type: approval\n    command: make lint\n    reason: run the linter\n---\n\n## Summary\nNeed lint."
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id": "call_1", "type": "function",
					"function": map[string]any{"name": "write_file", "arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, tID, report)},
				}},
			}}},
			"usage": usage,
		})
	}))
	defer tsMock.Close()
	cfg.Providers["mock_prov"] = config.ProviderConfig{Type: "openai", BaseURL: tsMock.URL, APIKeyEnv: "MOCK_KEY"}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	engine, err := NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := engine.Run(ctx); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}
	state := engine.stateMgr.GetState()
	if state.Status != "AWAITING_INPUT" {
		t.Fatalf("expected project AWAITING_INPUT, got %s", state.Status)
	}
	t1 := state.TaskStates["T01"]
	if t1.Status != schemas.TaskStateAwaiting || len(t1.Requests) != 1 || t1.Requests[0].Command != "make lint" {
		t.Fatalf("expected T01 awaiting approval of 'make lint', got %+v", t1)
	}

	// Answer, then resume with a fresh engine as `ringmaster resume` would
	err = QueueAnswer(projectRoot, schemas.HumanAnswer{
		TaskID:  "T01",
		Replies: map[string]string{"r1": "approve"},
		Allow:   []string{"make lint"},
		Scope:   "task",
	})
	if err != nil {
		t.Fatalf("QueueAnswer failed: %v", err)
	}
	engine, err = NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine (resume) failed: %v", err)
	}
	if err := engine.Run(ctx); err != nil {
		t.Fatalf("engine.Run (resume) failed: %v", err)
	}

	state = engine.stateMgr.GetState()
	if state.Status != "COMPLETED" {
		t.Errorf("expected project COMPLETED after answering, got %s (T01: %+v)", state.Status, state.TaskStates["T01"])
	}
	if got := LoadApprovals(projectRoot).For("T01"); len(got) != 1 || got[0] != "make lint" {
		t.Errorf("expected 'make lint' approved for T01, got %v", got)
	}
	if got := LoadApprovals(projectRoot).For("T02"); len(got) != 0 {
		t.Errorf("task-scoped approval leaked to T02: %v", got)
	}
	if entries, _ := os.ReadDir(answersDir(projectRoot)); len(entries) != 0 {
		t.Errorf("expected answer inbox to be drained, found %d file(s)", len(entries))
	}
}

// An acceptance command that needs a shell can never pass. The task must
// stop before the automaton is called, not after it has done the work.
func TestTaskWithUnrunnableAcceptanceIsNotStarted(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	spec := `---
id: T01
title: "Task One"
type: backend
depends_on: []
worker: "test_worker"
max_retries: 2
acceptance:
  - "grep -q json README.md && echo ok"
---
Implement task one.
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".ringmaster", "tasks", "T01-task-one.md"), []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	calls := 0
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		http.Error(w, "the automaton must not be called", http.StatusInternalServerError)
	}))
	defer tsMock.Close()
	cfg.Providers["mock_prov"] = config.ProviderConfig{Type: "openai", BaseURL: tsMock.URL, APIKeyEnv: "MOCK_KEY"}

	engine, err := NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = engine.Run(ctx)
	if ctx.Err() != nil {
		t.Fatal("the run did not end on its own")
	}

	ts := engine.stateMgr.GetState().TaskStates["T01"]
	if ts.Status != schemas.TaskStateReview {
		t.Fatalf("T01 status = %s, want NEEDS_REVIEW (err: %s)", ts.Status, ts.Error)
	}
	for _, want := range []string{"cannot run as written", ".ringmaster/tasks/", "without a shell"} {
		if !strings.Contains(ts.Error, want) {
			t.Errorf("reason does not mention %q:\n%s", want, ts.Error)
		}
	}
	if ts.Usage.TokensIn != 0 || ts.Attempt != 0 {
		t.Errorf("the task spent an attempt: attempt=%d tokens_in=%d", ts.Attempt, ts.Usage.TokensIn)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("the automaton was called %d time(s) for a task that can never be accepted", calls)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".ringmaster", "worktrees", "T01")); err == nil {
		t.Error("a worktree was created for a task that was not started")
	}
}

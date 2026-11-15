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
			// The automaton asks for another step, so it is not done when the budget runs out
			"choices": []map[string]any{{"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id": "call_0", "type": "function",
					"function": map[string]any{"name": "read_file", "arguments": `{"path":"README.md"}`},
				}},
			}}},
			"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70},
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
	reason := state.TaskStates["T01"].Error
	for _, want := range []string{"70 of 10 budget tokens", "max_tokens_per_task", "Resume"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the pause reason does not say %q:\n%s", want, reason)
		}
	}
}

// An agentic automaton does a whole task in one call and reads its context
// from the cache on every step. Its raw input count is far above the limit,
// but most of it is cache reads, and the work is finished: the task must be
// verified and merged, not paused.
func TestFinishedTaskIsNotPausedForCacheReads(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages, _ := body["messages"].([]any)
		tID := "T01"
		for _, m := range messages {
			if content, _ := m.(map[string]any)["content"].(string); strings.Contains(content, "T02") {
				tID = "T02"
				break
			}
		}
		// Second call of a task: the report is written, so the automaton stops
		last, _ := messages[len(messages)-1].(map[string]any)
		message := map[string]any{"role": "assistant", "content": "done"}
		if last["role"] != "tool" {
			report := fmt.Sprintf("---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 1\ntokens_out: 1\nfiles_changed: []\n---\n\n## Summary\nFinished %s", tID)
			message = map[string]any{"role": "assistant", "tool_calls": []map[string]any{{
				"id": "call_0", "type": "function",
				"function": map[string]any{"name": "write_file", "arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, tID, report)},
			}}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": message}},
			// 400k input per call, 396k of it from the cache: 4k + 39.6k + 1k budget tokens
			"usage": map[string]any{
				"prompt_tokens": 400000, "completion_tokens": 1000, "total_tokens": 401000,
				"prompt_tokens_details": map[string]any{"cached_tokens": 396000},
			},
		})
	}))
	defer tsMock.Close()
	cfg.Providers["mock_prov"] = config.ProviderConfig{Type: "openai", BaseURL: tsMock.URL, APIKeyEnv: "MOCK_KEY"}
	cfg.Budget.MaxTokensPerTask = 300000
	cfg.Budget.MaxTokensPerProject = 2000000

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
	for _, id := range []string{"T01", "T02"} {
		ts := state.TaskStates[id]
		if ts.Status != schemas.TaskStateCompleted {
			t.Errorf("%s status = %s, want COMPLETED (err: %s)", id, ts.Status, ts.Error)
		}
		if ts.Usage.TokensIn < 300000 {
			t.Errorf("%s recorded %d input tokens; the test needs a raw count above the limit", id, ts.Usage.TokensIn)
		}
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

// An automaton that finishes without a report Ringmaster can use is told
// what was wrong and writes the report again. The user is not asked.
func TestUnusableReportIsRetriedNotSentToTheUser(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	var mu sync.Mutex
	t01Calls := 0
	var feedback string
	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages, _ := body["messages"].([]any)
		tID := "T01"
		all := ""
		for _, m := range messages {
			content, _ := m.(map[string]any)["content"].(string)
			all += content + "\n"
			if strings.Contains(content, "T02") {
				tID = "T02"
			}
		}
		last, _ := messages[len(messages)-1].(map[string]any)

		message := map[string]any{"role": "assistant", "content": "All done, the report is written."}
		mu.Lock()
		if tID == "T01" {
			t01Calls++
		}
		// T01's first attempt says it is done but writes no report at all
		skipReport := tID == "T01" && t01Calls == 1
		if tID == "T01" && strings.Contains(all, "PREVIOUS ATTEMPT FAILED") {
			feedback = all
		}
		mu.Unlock()

		if !skipReport && last["role"] != "tool" {
			report := fmt.Sprintf("---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 1\ntokens_out: 1\nfiles_changed: []\n---\n\n## Summary\nFinished %s", tID)
			message = map[string]any{"role": "assistant", "tool_calls": []map[string]any{{
				"id": "call_0", "type": "function",
				"function": map[string]any{"name": "write_file", "arguments": fmt.Sprintf(`{"path":".ringmaster/outputs/%s.md","content":%q}`, tID, report)},
			}}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": message}},
			"usage":   map[string]any{"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70},
		})
	}))
	defer tsMock.Close()
	cfg.Providers["mock_prov"] = config.ProviderConfig{Type: "openai", BaseURL: tsMock.URL, APIKeyEnv: "MOCK_KEY"}

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
	t01 := state.TaskStates["T01"]
	if t01.Status != schemas.TaskStateCompleted {
		t.Fatalf("T01 status = %s, want COMPLETED (err: %s)", t01.Status, t01.Error)
	}
	if t01.Attempt != 2 {
		t.Errorf("T01 took %d attempt(s), want 2 (one without a report, one with)", t01.Attempt)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, want := range []string{"there is no report file", ".ringmaster/outputs/T01.md", "do NOT redo", "status: COMPLETED"} {
		if !strings.Contains(feedback, want) {
			t.Errorf("the second attempt was not told %q", want)
		}
	}
}

// When it never writes a usable report, the user is told exactly that.
func TestUnusableReportAfterAllRetriesIsExplained(t *testing.T) {
	projectRoot, cfg := setupTestProject(t)
	t.Setenv("MOCK_KEY", "dummy")

	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "T01 is done and the report is written."}}},
			"usage":   map[string]any{"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70},
		})
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

	t01 := engine.stateMgr.GetState().TaskStates["T01"]
	if t01.Status != schemas.TaskStateReview {
		t.Fatalf("T01 status = %s, want NEEDS_REVIEW", t01.Status)
	}
	if t01.Attempt != 3 {
		t.Errorf("T01 took %d attempt(s), want 3 (max_retries is 2)", t01.Attempt)
	}
	for _, want := range []string{"no report Ringmaster could use", "there is no report file", "acceptance checks were not run"} {
		if !strings.Contains(t01.Error, want) {
			t.Errorf("the reason does not say %q:\n%s", want, t01.Error)
		}
	}
	if strings.Contains(t01.Error, "requested human review") {
		t.Errorf("the reason still claims the automaton asked for review:\n%s", t01.Error)
	}
}

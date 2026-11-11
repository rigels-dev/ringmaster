package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ringmaster/internal/config"
	"ringmaster/internal/planner"
	"ringmaster/internal/scheduler"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

const (
	brokenCalc   = "package calc\n\nfunc Add(a, b int) int { return a - b }\n"
	fixedCalc    = "package calc\n\nfunc Add(a, b int) int { return a + b }\n"
	originalTest = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != 4 {\n\t\tt.Fatal(\"Add(2, 2) should be 4\")\n\t}\n}\n"
	gutTest      = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n"
	newTest      = "package calc\n\nimport \"testing\"\n\nfunc TestAddZero(t *testing.T) {\n\tif Add(0, 0) != 0 {\n\t\tt.Fatal(\"Add(0, 0) should be 0\")\n\t}\n}\n"
)

// cheatingWorker is a worker that "fixes" a failing test by gutting it. Only
// when told its test changes were undone does it fix the code instead.
func cheatingWorker(t *testing.T) *httptest.Server {
	t.Helper()
	writeFile := func(id, path, content string) map[string]any {
		args, _ := json.Marshal(map[string]string{"path": path, "content": content})
		return map[string]any{"id": id, "type": "function", "function": map[string]any{"name": "write_file", "arguments": string(args)}}
	}
	report := "---\nstatus: COMPLETED\nattempts: 1\ncalls: 1\ntokens_in: 10\ntokens_out: 10\nfiles_changed: []\n---\n\n## Summary\nMade the tests pass.\n"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		system, turns := "", 0
		for _, m := range req.Messages {
			if m.Role == "system" {
				system = m.Content
			}
			if m.Role == "assistant" {
				turns++
			}
		}

		message := map[string]any{"role": "assistant", "content": "done"}
		if turns == 0 {
			calls := []map[string]any{writeFile("c1", ".ringmaster/outputs/T01.md", report)}
			if strings.Contains(system, "Ringmaster restored these existing test files") {
				calls = append(calls, writeFile("c2", "calc.go", fixedCalc))
			} else {
				calls = append(calls,
					writeFile("c2", "calc_test.go", gutTest),
					writeFile("c3", "calc_extra_test.go", newTest),
				)
			}
			message = map[string]any{"role": "assistant", "tool_calls": calls}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": message}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
		})
	}))
}

// runCheatingTask runs one task ("make go test pass") against a repository
// whose code is wrong and whose existing test is right.
func runCheatingTask(t *testing.T, task schemas.PlanTask, tune func(*config.Config)) (projectRoot string, state schemas.ProjectState) {
	t.Helper()
	server := cheatingWorker(t)
	t.Cleanup(server.Close)

	projectRoot = t.TempDir()
	ctx := context.Background()
	_, _ = workspace.RunGit(ctx, projectRoot, "init", "-b", "main")
	_ = workspace.EnsureGitIdentity(ctx, projectRoot)
	for name, content := range map[string]string{
		"go.mod":       "module calc\n\ngo 1.21\n",
		"calc.go":      brokenCalc,
		"calc_test.go": originalTest,
	} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = workspace.RunGit(ctx, projectRoot, "add", "-A")
	if _, err := workspace.RunGit(ctx, projectRoot, "commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(projectRoot, ".ringmaster"), 0755)
	_ = os.WriteFile(filepath.Join(projectRoot, ".ringmaster", "BRIEF.md"), []byte("# Fix Add\n"), 0644)

	t.Setenv("E2E_MOCK_KEY", "k")
	t.Setenv("RINGMASTER_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Sandbox.Allow = []string{"go test"}
	cfg.Providers["mock"] = config.ProviderConfig{Type: "openai", BaseURL: server.URL, APIKeyEnv: "E2E_MOCK_KEY"}
	cfg.Agents["w"] = config.AgentConfig{Provider: "mock", Model: "m", Role: "worker", ToolCalling: true}
	cfg.Routing.DefaultWorker = "w"
	if tune != nil {
		tune(cfg)
	}

	task.ID, task.Title, task.Worker, task.MaxRetries = "T01", "Fix Add", "w", 2
	task.Acceptance = []string{"go test ./..."}
	plan := &schemas.Plan{Project: "calc", Tasks: []schemas.PlanTask{task}}
	if err := storage.WriteJSONAtomic(filepath.Join(projectRoot, ".ringmaster", "plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := planner.ApprovePlan(projectRoot, plan); err != nil {
		t.Fatal(err)
	}

	engine, err := scheduler.NewEngine(projectRoot, cfg, 1)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := engine.Run(runCtx); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}
	return projectRoot, engine.GetState()
}

func fileOnMain(t *testing.T, projectRoot, name string) string {
	t.Helper()
	out, err := workspace.RunGit(context.Background(), projectRoot, "show", "main:"+name)
	if err != nil {
		return ""
	}
	return out
}

// A worker cannot pass an acceptance check by weakening a test that already
// exists: its edit is undone before the check runs, the attempt fails, and the
// retry is told why. Tests it adds are kept.
func TestE2EExistingTestsAreRestoredBeforeAcceptance(t *testing.T) {
	projectRoot, state := runCheatingTask(t, schemas.PlanTask{Type: schemas.TaskTypeBackend}, nil)

	ts := state.TaskStates["T01"]
	if ts.Status != schemas.TaskStateCompleted {
		t.Fatalf("task did not complete: %+v", ts)
	}
	if ts.Attempt != 2 {
		t.Errorf("expected the gutted-test attempt to fail and a second to succeed, got %d attempt(s)", ts.Attempt)
	}
	if got := fileOnMain(t, projectRoot, "calc_test.go"); got != originalTest {
		t.Errorf("the existing test on main is not the original:\n%s", got)
	}
	if got := fileOnMain(t, projectRoot, "calc.go"); got != fixedCalc {
		t.Errorf("the code on main was not fixed:\n%s", got)
	}
	if got := fileOnMain(t, projectRoot, "calc_extra_test.go"); got != newTest {
		t.Errorf("the test file the worker added should be kept, got:\n%s", got)
	}

	events, _ := os.ReadFile(filepath.Join(projectRoot, ".ringmaster", "logs", "events.jsonl"))
	if !strings.Contains(string(events), `"TESTS_RESTORED"`) || !strings.Contains(string(events), "calc_test.go") {
		t.Errorf("audit trail lacks the restore:\n%s", events)
	}
	report, _ := os.ReadFile(filepath.Join(projectRoot, ".ringmaster", "outputs", "T01.md"))
	if strings.Contains(string(report), "were restored") {
		t.Errorf("the final attempt changed no tests, yet its report mentions a restore:\n%s", report)
	}
}

// The protection has three off-switches, each explicit: the approved plan
// (edits_tests on the task), the task type "test", and [sandbox] allow_test_edits.
func TestE2EExistingTestsMayChangeWhenAllowed(t *testing.T) {
	cases := []struct {
		name string
		task schemas.PlanTask
		tune func(*config.Config)
	}{
		{"plan sets edits_tests", schemas.PlanTask{Type: schemas.TaskTypeBackend, EditsTests: true}, nil},
		{"task of type test", schemas.PlanTask{Type: schemas.TaskTypeTest}, nil},
		{"sandbox allow_test_edits", schemas.PlanTask{Type: schemas.TaskTypeBackend}, func(c *config.Config) { c.Sandbox.AllowTestEdits = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			projectRoot, state := runCheatingTask(t, c.task, c.tune)
			ts := state.TaskStates["T01"]
			if ts.Status != schemas.TaskStateCompleted || ts.Attempt != 1 {
				t.Fatalf("expected completion on the first attempt: %+v", ts)
			}
			if got := fileOnMain(t, projectRoot, "calc_test.go"); got != gutTest {
				t.Errorf("the worker's change to the test should have been kept:\n%s", got)
			}
			events, _ := os.ReadFile(filepath.Join(projectRoot, ".ringmaster", "logs", "events.jsonl"))
			if strings.Contains(string(events), "TESTS_RESTORED") {
				t.Errorf("nothing should have been restored:\n%s", events)
			}
		})
	}
}

// With custom patterns only the files they name are protected.
func TestE2ECustomTestFilePatterns(t *testing.T) {
	projectRoot, state := runCheatingTask(t, schemas.PlanTask{Type: schemas.TaskTypeBackend}, func(c *config.Config) {
		c.Sandbox.TestFiles = []string{"*.feature"}
	})
	if ts := state.TaskStates["T01"]; ts.Status != schemas.TaskStateCompleted || ts.Attempt != 1 {
		t.Fatalf("calc_test.go is outside the custom patterns, so the edit should stand: %+v", ts)
	}
	if got := fileOnMain(t, projectRoot, "calc_test.go"); got != gutTest {
		t.Errorf("unexpected test content on main:\n%s", got)
	}
}

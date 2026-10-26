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
	repoDir := filepath.Join(projectRoot, "repo")
	_ = os.MkdirAll(repoDir, 0755)
	ctx := context.Background()
	_, _ = workspace.RunGit(ctx, repoDir, "init", "-b", "main")
	_ = workspace.EnsureGitIdentity(ctx, repoDir)
	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test Repo\n"), 0644)
	_, _ = workspace.RunGit(ctx, repoDir, "add", "README.md")
	_, _ = workspace.RunGit(ctx, repoDir, "commit", "-m", "init")

	// 2. Setup brief
	_ = os.WriteFile(filepath.Join(projectRoot, "BRIEF.md"), []byte("# Test Project Brief\n"), 0644)

	// 3. Setup tasks
	tasksDir := filepath.Join(projectRoot, "tasks")
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

	planPath := filepath.Join(projectRoot, "plan.json")
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
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "write_file",
									"arguments": fmt.Sprintf(`{"path":"outputs/%s.md","content":%q}`, tID, reportYAML),
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
	out, _ := workspace.RunGit(ctx, filepath.Join(projectRoot, "repo"), "log", "--oneline")
	if !strings.Contains(out, "Merge task T01") || !strings.Contains(out, "Merge task T02") {
		t.Errorf("git repo missing merge commits on main:\n%s", out)
	}
}

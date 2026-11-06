package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ringmaster/internal/config"
	"ringmaster/internal/planner"
	"ringmaster/internal/registry"
	"ringmaster/internal/scheduler"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
	"ringmaster/tests/mockserver"
)

func setupE2EProject(t *testing.T, ms *mockserver.MockServer) (string, *config.Config) {
	t.Helper()
	projectRoot := t.TempDir()

	// 1. Git repository setup
	repoDir := projectRoot
	_ = os.MkdirAll(filepath.Join(projectRoot, ".ringmaster"), 0755)
	ctx := context.Background()
	_, _ = workspace.RunGit(ctx, repoDir, "init", "-b", "main")
	_ = workspace.EnsureGitIdentity(ctx, repoDir)
	_ = os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Ringmaster E2E\n"), 0644)
	_, _ = workspace.RunGit(ctx, repoDir, "add", "README.md")
	_, _ = workspace.RunGit(ctx, repoDir, "commit", "-m", "init commit")

	// 2. Brief setup
	_ = os.WriteFile(filepath.Join(projectRoot, ".ringmaster", "BRIEF.md"), []byte("# E2E Project Brief\nBuild Auth and User modules.\n"), 0644)

	// 3. Config setup
	t.Setenv("E2E_MOCK_KEY", "super-secret-key-12345")
	cfg := config.DefaultConfig()
	cfg.Sandbox.Allow = []string{"echo"}
	cfg.Providers["mock_e2e"] = config.ProviderConfig{
		Type:      "openai",
		BaseURL:   ms.URL(),
		APIKeyEnv: "E2E_MOCK_KEY",
	}
	cfg.Agents["mock-master"] = config.AgentConfig{
		Provider: "mock_e2e",
		Model:    "mock-gpt",
		Role:     "master",
	}
	cfg.Agents["mock-worker"] = config.AgentConfig{
		Provider:    "mock_e2e",
		Model:       "mock-gpt",
		Role:        "worker",
		ToolCalling: true,
	}
	cfg.Routing.DefaultMaster = "mock-master"
	cfg.Routing.DefaultWorker = "mock-worker"

	return projectRoot, cfg
}

func TestE2ELifecycleHappyPath(t *testing.T) {
	ms := mockserver.NewMockServer(mockserver.ScenarioHappy)
	defer ms.Close()

	projectRoot, cfg := setupE2EProject(t, ms)

	// 1. PLAN: Generate DAG from master agent
	reg := registry.NewRegistry(cfg)
	prov, agent, err := reg.GetProviderForAgent("mock-master")
	if err != nil {
		t.Fatalf("GetProviderForAgent failed: %v", err)
	}

	plan, usage, err := planner.GeneratePlan(context.Background(), projectRoot, cfg, *agent, prov)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks in plan, got: %d", len(plan.Tasks))
	}
	if usage.TokensTotal() <= 0 {
		t.Errorf("expected tokens consumed during planning")
	}

	// 2. APPROVE: Gate approval
	hash, err := planner.ApprovePlan(projectRoot, plan)
	if err != nil || len(hash) != 64 {
		t.Fatalf("ApprovePlan failed: %v (hash: %s)", err, hash)
	}

	// 3. RUN: Execute DAG via Scheduler Engine
	engine, err := scheduler.NewEngine(projectRoot, cfg, 2)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := engine.Run(ctx); err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	// 4. VERIFY STATE & OUTCOMES
	state := engine.GetState()
	if state.Status != "COMPLETED" {
		t.Errorf("expected project state COMPLETED, got: %s", state.Status)
	}

	for _, tID := range []string{"T01", "T02"} {
		ts, ok := state.TaskStates[tID]
		if !ok || ts.Status != schemas.TaskStateCompleted {
			t.Errorf("task %s was not completed: %+v", tID, ts)
		}
	}

	// 5. MERGE VERIFICATION: check git repo on main has both committed files
	repoDir := projectRoot
	logOut, _ := workspace.RunGit(ctx, repoDir, "log", "--oneline")
	if !strings.Contains(logOut, "Merge task T01") || !strings.Contains(logOut, "Merge task T02") {
		t.Errorf("main repo log missing merge commits:\n%s", logOut)
	}

	for _, file := range []string{"t01.txt", "t02.txt"} {
		if _, err := os.Stat(filepath.Join(repoDir, file)); err != nil {
			t.Errorf("expected %s on main branch, but not found", file)
		}
	}

	// 6. SECURITY AUDIT: Zero API Key Leakage
	secret := "super-secret-key-12345"
	_ = filepath.Walk(projectRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr == nil && strings.Contains(string(data), secret) {
			t.Errorf("CRITICAL SECURITY LEAK: API key found in file: %s", p)
		}
		return nil
	})
}

func TestE2EBlockedAndHumanAnswer(t *testing.T) {
	ms := mockserver.NewMockServer(mockserver.ScenarioBlocked)
	defer ms.Close()

	projectRoot, cfg := setupE2EProject(t, ms)

	// Setup plan directly & approve
	plan := &schemas.Plan{
		Project: filepath.Base(projectRoot),
		Tasks: []schemas.PlanTask{
			{
				ID:         "T01",
				Title:      "Needs Database",
				Type:       schemas.TaskTypeBackend,
				Worker:     "mock-worker",
				MaxRetries: 2,
				Acceptance: []string{"echo db_ok"},
			},
		},
	}
	_ = storage.WriteJSONAtomic(filepath.Join(projectRoot, ".ringmaster", "plan.json"), plan)
	_, _ = planner.ApprovePlan(projectRoot, plan)

	// Create task file
	tasksDir := filepath.Join(projectRoot, ".ringmaster", "tasks")
	_ = os.MkdirAll(tasksDir, 0755)
	_ = os.WriteFile(filepath.Join(tasksDir, "T01-needs-database.md"), []byte(`---
id: T01
title: "Needs Database"
type: backend
depends_on: []
worker: "mock-worker"
max_retries: 2
acceptance:
  - "echo db_ok"
---
Configure database.
`), 0644)

	engine, err := scheduler.NewEngine(projectRoot, cfg, 1)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// First Run -> Hits BLOCKED
	_ = engine.Run(context.Background())
	state := engine.GetState()
	if state.TaskStates["T01"].Status != schemas.TaskStateBlocked {
		t.Fatalf("expected T01 to be BLOCKED, got: %s", state.TaskStates["T01"].Status)
	}

	// Human answers blocker
	taskFile := filepath.Join(tasksDir, "T01-needs-database.md")
	content, _ := os.ReadFile(taskFile)
	contentWithAnswer := string(content) + "\n\n## Human Answers\nDatabase host is localhost:5432\n"
	_ = os.WriteFile(taskFile, []byte(contentWithAnswer), 0644)

	// Reset state to PENDING (simulate ringmaster answer)
	_ = engine.UpdateTaskState("T01", func(ts *schemas.TaskState) {
		ts.Status = schemas.TaskStatePending
		ts.Error = ""
	})

	// Second Run -> Resumes and Completes
	_ = engine.Run(context.Background())
	state = engine.GetState()
	if state.TaskStates["T01"].Status != schemas.TaskStateCompleted {
		t.Errorf("expected T01 to complete after human answer, got: %s (err: %s)", state.TaskStates["T01"].Status, state.TaskStates["T01"].Error)
	}
}

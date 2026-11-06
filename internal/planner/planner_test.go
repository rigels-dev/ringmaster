package planner

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

	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

func TestEstimatesCalculation(t *testing.T) {
	plan := &schemas.Plan{
		Project: "demo",
		Tasks: []schemas.PlanTask{
			{ID: "T01", Type: schemas.TaskTypeBackend},
			{ID: "T02", Type: schemas.TaskTypeFrontend},
			{ID: "T03", Type: schemas.TaskTypeTest},
		},
	}

	est := CalculateEstimates(plan, nil)
	if est.TotalMin != 90000 || est.TotalMax != 270000 {
		t.Errorf("expected 90k-270k, got min=%d max=%d", est.TotalMin, est.TotalMax)
	}

	preview := RenderPreview(plan, est, config.BudgetConfig{MaxTokensPerProject: 200000})
	if !strings.Contains(preview, "90k – 270k") {
		t.Errorf("preview missing cumulative token range: %s", preview)
	}
	if !strings.Contains(preview, "WARNING: Maximum estimated tokens") {
		t.Errorf("preview missing budget warning when max exceeds project budget")
	}
}

func TestApproveAndVerifyPlan(t *testing.T) {
	tempDir := t.TempDir()

	plan := &schemas.Plan{
		Project: "approval_test",
		Tasks: []schemas.PlanTask{
			{ID: "T01", Title: "Task 1", Type: schemas.TaskTypeBackend, Acceptance: []string{"check"}},
		},
	}

	// 1. Approve
	hash, err := ApprovePlan(tempDir, plan)
	if err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64 char sha256 hash, got: %s", hash)
	}

	// 2. Verify matches
	valid, err := VerifyPlanApproval(tempDir, plan)
	if err != nil || !valid {
		t.Fatalf("expected valid approval: %v", err)
	}

	// 3. Modify plan and verify rejection
	modifiedPlan := &schemas.Plan{
		Project: "approval_test",
		Tasks: []schemas.PlanTask{
			{ID: "T01", Title: "Modified Title", Type: schemas.TaskTypeBackend, Acceptance: []string{"check"}},
		},
	}
	valid, err = VerifyPlanApproval(tempDir, modifiedPlan)
	if err == nil || valid {
		t.Errorf("expected rejection when plan is modified without re-approval")
	}

	// 4. Revoke approval
	_ = RevokeApproval(tempDir)
	valid, err = VerifyPlanApproval(tempDir, plan)
	if err == nil || valid {
		t.Errorf("expected failure after approval revocation")
	}
}

func TestGeneratePlan(t *testing.T) {
	tempDir := t.TempDir()
	briefFile := filepath.Join(tempDir, ".ringmaster", "BRIEF.md")
	_ = os.MkdirAll(filepath.Dir(briefFile), 0755)
	_ = os.WriteFile(briefFile, []byte("# Brief\nBuild a REST API in Go\n"), 0644)

	rawPlan := map[string]any{
		"project": "demo_project",
		"tasks": []map[string]any{
			{
				"id":          "T01",
				"title":       "Setup DB",
				"type":        "backend",
				"depends_on":  []string{},
				"worker":      "coder",
				"max_retries": 2,
				"acceptance":  []string{"go test ./..."},
				"description": "Setup SQLite connection",
			},
			{
				"id":          "T02",
				"title":       "Auth API",
				"type":        "backend",
				"depends_on":  []string{"T01"},
				"worker":      "coder",
				"max_retries": 2,
				"acceptance":  []string{"go test -v ./..."},
				"description": "Implement JWT endpoints",
			},
		},
	}
	planBytes, _ := json.Marshal(rawPlan)

	tsMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": fmt.Sprintf("```json\n%s\n```", string(planBytes)),
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     200,
				"completion_tokens": 100,
				"total_tokens":      300,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tsMock.Close()

	t.Setenv("MOCK_KEY", "dummy")
	provider := llm.NewProvider("openai", tsMock.URL, "MOCK_KEY")

	cfg := config.DefaultConfig()
	cfg.Agents["master"] = config.AgentConfig{Role: "master", Model: "gpt-4o"}
	cfg.Agents["coder"] = config.AgentConfig{Role: "worker", Model: "gpt-4o-mini"}

	plan, usage, err := GeneratePlan(context.Background(), tempDir, cfg, cfg.Agents["master"], provider)
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(plan.Tasks))
	}
	if usage.TokensTotal() != 300 {
		t.Errorf("expected 300 tokens, got %d", usage.TokensTotal())
	}

	// Verify plan.json exists
	savedPlan, err := storage.ReadJSON[schemas.Plan](filepath.Join(tempDir, ".ringmaster", "plan.json"))
	if err != nil || len(savedPlan.Tasks) != 2 {
		t.Errorf("failed reading saved plan.json: %v", err)
	}

	// Verify task files created
	tasksDir := filepath.Join(tempDir, ".ringmaster", "tasks")
	entries, _ := os.ReadDir(tasksDir)
	if len(entries) != 2 {
		t.Errorf("expected 2 task files in tasks/, got %d", len(entries))
	}
}

func TestApproveRequiresAnsweredQuestions(t *testing.T) {
	dir := t.TempDir()
	plan := &schemas.Plan{
		Project:       "q",
		Tasks:         []schemas.PlanTask{{ID: "T01", Title: "x", Type: schemas.TaskTypeBackend, Acceptance: []string{"go test ./..."}}},
		OpenQuestions: []schemas.PlanQuestion{{ID: "Q1", Question: "Which DB?", Options: []string{"sqlite", "postgres"}}},
	}
	if _, err := ApprovePlan(dir, plan); err == nil {
		t.Fatal("expected approval to be refused while Q1 is unanswered")
	}

	plan.OpenQuestions[0].Answer = "sqlite"
	before, _ := plan.PlanHash()
	if _, err := ApprovePlan(dir, plan); err != nil {
		t.Fatalf("expected approval once answered, got %v", err)
	}

	// Changing an answer after approval invalidates it
	plan.OpenQuestions[0].Answer = "postgres"
	if after, _ := plan.PlanHash(); after == before {
		t.Error("expected plan hash to cover question answers")
	}
	if ok, _ := VerifyPlanApproval(dir, plan); ok {
		t.Error("expected approval to be invalid after changing an answer")
	}
}

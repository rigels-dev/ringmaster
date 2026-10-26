package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

func setupStatusTestProject(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()

	cfg = config.DefaultConfig()

	// 1. Create plan.json
	plan := &schemas.Plan{
		Project: "status_test",
		Tasks: []schemas.PlanTask{
			{ID: "T01", Title: "Task One", Type: schemas.TaskTypeBackend, Worker: "coder", MaxRetries: 2},
			{ID: "T02", Title: "Task Two", Type: schemas.TaskTypeFrontend, Worker: "coder", MaxRetries: 2},
			{ID: "T03", Title: "Task Three", Type: schemas.TaskTypeTest, Worker: "coder", MaxRetries: 2},
		},
	}
	_ = storage.WriteJSONAtomic(filepath.Join(projectDir, "plan.json"), plan)

	// 2. Create state.json with diverse statuses
	state := schemas.NewProjectState("status_test", "test-hash")
	state.Status = "ACTIVE"
	state.TaskStates["T01"] = schemas.TaskState{
		ID:      "T01",
		Status:  schemas.TaskStateCompleted,
		Worker:  "coder",
		Attempt: 1,
		Usage:   schemas.Usage{Calls: 2, TokensIn: 200, TokensOut: 80},
	}
	state.TaskStates["T02"] = schemas.TaskState{
		ID:      "T02",
		Status:  schemas.TaskStateBlocked,
		Worker:  "coder",
		Attempt: 1,
		Error:   "Need API key for third-party service",
	}
	state.TaskStates["T03"] = schemas.TaskState{
		ID:      "T03",
		Status:  schemas.TaskStateReview,
		Worker:  "coder",
		Attempt: 2,
		Error:   "Acceptance criteria failed",
	}
	_ = os.MkdirAll(filepath.Join(projectDir, ".ringmaster"), 0755)
	_ = storage.WriteJSONAtomic(filepath.Join(projectDir, ".ringmaster", "state.json"), state)

	return projectDir
}

func TestStatusViewRendering(t *testing.T) {
	projectDir := setupStatusTestProject(t)

	// Render text status
	view, isHalted, err := buildStatusView(projectDir, "status_test", false)
	if err != nil {
		t.Fatalf("buildStatusView error: %v", err)
	}

	if isHalted {
		t.Errorf("expected isHalted=false for active project")
	}

	if !strings.Contains(view, "DONE") || !strings.Contains(view, "BLOCKED") || !strings.Contains(view, "REVIEW") {
		t.Errorf("status view missing task status badges: %s", view)
	}

	if !strings.Contains(view, "ACTIONABLE BLOCKERS") || !strings.Contains(view, "ringmaster answer T02") {
		t.Errorf("status view missing actionable blockers instructions: %s", view)
	}

	if !strings.Contains(view, "TASKS REQUIRING REVIEW") {
		t.Errorf("status view missing tasks requiring review: %s", view)
	}

	// Render JSON status
	jsonView, _, err := buildStatusView(projectDir, "status_test", true)
	if err != nil {
		t.Fatalf("buildStatusView json error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonView), &parsed); err != nil {
		t.Fatalf("failed parsing json view: %v", err)
	}
	if parsed["project"] != "status_test" {
		t.Errorf("expected project=status_test, got %v", parsed["project"])
	}
}

func TestUsageViewRendering(t *testing.T) {
	projectDir := setupStatusTestProject(t)

	// Render text usage
	view, err := buildUsageView(projectDir, "status_test", false)
	if err != nil {
		t.Fatalf("buildUsageView error: %v", err)
	}

	if !strings.Contains(view, "CONTROL PLANE & ROLE BREAKDOWN") {
		t.Errorf("usage view missing role breakdown: %s", view)
	}
	if !strings.Contains(view, "PER-TASK ACCURACY") {
		t.Errorf("usage view missing per-task table: %s", view)
	}

	// Render JSON usage
	jsonView, err := buildUsageView(projectDir, "status_test", true)
	if err != nil {
		t.Fatalf("buildUsageView json error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonView), &parsed); err != nil {
		t.Fatalf("failed parsing json usage view: %v", err)
	}
	if parsed["project"] != "status_test" {
		t.Errorf("expected project=status_test, got %v", parsed["project"])
	}
}

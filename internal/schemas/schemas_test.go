package schemas

import (
	"strings"
	"testing"
)

func TestTaskMarkdownParsing(t *testing.T) {
	validMD := `---
id: T01
title: Scaffold Database
type: backend
depends_on: []
worker: coder-cheap
max_retries: 3
acceptance:
  - "go test ./db/... passes"
---

This is the main task description.

## Human Answers
Use SQLite with pure driver.
`
	task, err := ParseTaskMarkdown(validMD)
	if err != nil {
		t.Fatalf("unexpected error parsing valid task: %v", err)
	}

	if task.ID != "T01" {
		t.Errorf("expected ID T01, got %s", task.ID)
	}
	if task.Type != TaskTypeBackend {
		t.Errorf("expected type backend, got %s", task.Type)
	}
	if task.MaxRetries != 3 {
		t.Errorf("expected max_retries 3, got %d", task.MaxRetries)
	}
	if task.HumanAnswers != "Use SQLite with pure driver." {
		t.Errorf("unexpected human answers: %s", task.HumanAnswers)
	}

	// Invalid ID
	badID := strings.Replace(validMD, "id: T01", "id: INVALID", 1)
	if _, err := ParseTaskMarkdown(badID); err == nil {
		t.Errorf("expected error for invalid ID, got nil")
	}

	// Empty acceptance
	noAcceptance := strings.Replace(validMD, "acceptance:\n  - \"go test ./db/... passes\"", "acceptance: []", 1)
	if _, err := ParseTaskMarkdown(noAcceptance); err == nil {
		t.Errorf("expected error for empty acceptance, got nil")
	}
}

func TestReportMarkdown(t *testing.T) {
	validReport := `---
status: BLOCKED
attempts: 1
calls: 4
tokens_in: 5000
tokens_out: 1200
files_changed:
  - "internal/db.go"
---

Summary of work done so far.

## Blockers
Need secret token for third-party auth service.
`
	rep, err := ParseReportMarkdown(validReport)
	if err != nil {
		t.Fatalf("unexpected error parsing report: %v", err)
	}
	if rep.Status != ReportStatusBlocked {
		t.Errorf("expected status BLOCKED, got %s", rep.Status)
	}
	if rep.Blockers != "Need secret token for third-party auth service." {
		t.Errorf("unexpected blocker content: %s", rep.Blockers)
	}

	// Blocked without blocker section should fail
	emptyBlocker := strings.Replace(validReport, "Need secret token for third-party auth service.", "", 1)
	if _, err := ParseReportMarkdown(emptyBlocker); err == nil {
		t.Errorf("expected validation error when BLOCKED has no blockers, got nil")
	}
}

func TestPlanDAGAndCycleDetection(t *testing.T) {
	// Diamond DAG:
	//      T01
	//     /   \
	//   T02   T03
	//     \   /
	//      T04
	diamond := &Plan{
		Project: "test-diamond",
		Tasks: []PlanTask{
			{ID: "T01", Type: TaskTypeBackend, DependsOn: []string{}, Acceptance: []string{"ok"}},
			{ID: "T02", Type: TaskTypeBackend, DependsOn: []string{"T01"}, Acceptance: []string{"ok"}},
			{ID: "T03", Type: TaskTypeBackend, DependsOn: []string{"T01"}, Acceptance: []string{"ok"}},
			{ID: "T04", Type: TaskTypeBackend, DependsOn: []string{"T02", "T03"}, Acceptance: []string{"ok"}},
		},
	}

	levels, err := diamond.TopologicalLevels()
	if err != nil {
		t.Fatalf("diamond DAG levels failed: %v", err)
	}

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	if len(levels[0]) != 1 || levels[0][0] != "T01" {
		t.Errorf("level 0 should be [T01], got %v", levels[0])
	}
	if len(levels[1]) != 2 || levels[1][0] != "T02" || levels[1][1] != "T03" {
		t.Errorf("level 1 should be [T02, T03], got %v", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0] != "T04" {
		t.Errorf("level 2 should be [T04], got %v", levels[2])
	}

	// Cycle: T01 -> T02 -> T01
	cyclePlan := &Plan{
		Project: "cycle",
		Tasks: []PlanTask{
			{ID: "T01", Type: TaskTypeBackend, DependsOn: []string{"T02"}, Acceptance: []string{"ok"}},
			{ID: "T02", Type: TaskTypeBackend, DependsOn: []string{"T01"}, Acceptance: []string{"ok"}},
		},
	}
	if err := ValidatePlan(cyclePlan); err == nil {
		t.Errorf("expected cycle error, got nil")
	}

	// Self cycle: T01 -> T01
	selfCycle := &Plan{
		Project: "self-cycle",
		Tasks: []PlanTask{
			{ID: "T01", Type: TaskTypeBackend, DependsOn: []string{"T01"}, Acceptance: []string{"ok"}},
		},
	}
	if err := ValidatePlan(selfCycle); err == nil {
		t.Errorf("expected self cycle error, got nil")
	}
}

func TestPlanHashStability(t *testing.T) {
	plan1 := &Plan{
		Project: "test-hash",
		Tasks: []PlanTask{
			{ID: "T01", Type: TaskTypeBackend, DependsOn: []string{"T00"}, Acceptance: []string{"acc1", "acc2"}},
			{ID: "T02", Type: TaskTypeFrontend, DependsOn: []string{"T01"}, Acceptance: []string{"acc"}},
		},
	}

	// Reversed order
	plan2 := &Plan{
		Project: "test-hash",
		Tasks: []PlanTask{
			{ID: "T02", Type: TaskTypeFrontend, DependsOn: []string{"T01"}, Acceptance: []string{"acc"}},
			{ID: "T01", Type: TaskTypeBackend, DependsOn: []string{"T00"}, Acceptance: []string{"acc2", "acc1"}},
		},
	}

	h1, err := plan1.PlanHash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := plan2.PlanHash()
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Errorf("hashes should be identical regardless of insertion order, got %s != %s", h1, h2)
	}
}

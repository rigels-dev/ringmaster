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

// Letting a task change existing tests is part of what the user approves: the
// flag changes the plan hash, the preview points it out, the planner is told
// how to ask for it, and a plan without the flag hashes as it always did.
func TestEditsTestsIsPartOfTheApprovedPlan(t *testing.T) {
	task := schemas.PlanTask{ID: "T01", Title: "Rename the API", Type: schemas.TaskTypeBackend, Worker: "w", MaxRetries: 2, Acceptance: []string{"go test ./..."}, DependsOn: []string{}}
	plain := &schemas.Plan{Project: "p", Tasks: []schemas.PlanTask{task}}
	task.EditsTests = true
	flagged := &schemas.Plan{Project: "p", Tasks: []schemas.PlanTask{task}}

	plainHash, err := plain.PlanHash()
	if err != nil {
		t.Fatal(err)
	}
	flaggedHash, err := flagged.PlanHash()
	if err != nil {
		t.Fatal(err)
	}
	if plainHash == flaggedHash {
		t.Error("edits_tests does not change the plan hash, so it could be added after approval")
	}

	data, _ := json.Marshal(plain)
	if strings.Contains(string(data), "edits_tests") {
		t.Errorf("an unset flag must stay out of plan.json, or existing approvals break: %s", data)
	}
	var back schemas.Plan
	data, _ = json.Marshal(flagged)
	if err := json.Unmarshal(data, &back); err != nil || !back.Tasks[0].EditsTests {
		t.Errorf("edits_tests lost in plan.json: %s", data)
	}

	estimates := CalculateEstimates(flagged, nil)
	if preview := RenderPreview(flagged, estimates, config.BudgetConfig{}); !strings.Contains(preview, "May change existing tests: T01") {
		t.Errorf("preview does not point out the task:\n%s", preview)
	}
	if preview := RenderPreview(plain, CalculateEstimates(plain, nil), config.BudgetConfig{}); strings.Contains(preview, "existing tests") {
		t.Errorf("preview mentions test edits for a plan without any:\n%s", preview)
	}

	if prompt := MasterSystemPrompt(config.DefaultConfig(), "w", nil); !strings.Contains(prompt, `"edits_tests": true`) {
		t.Error("the planner is not told about edits_tests")
	}
}

// The planner is told which skills the project's workers will have, so it can
// point a task at one; disabled skills, whether off for the fleet or only for
// this project, are left out.
func TestPlannerSeesEnabledSkills(t *testing.T) {
	t.Setenv("MOCK_KEY", "k")
	configDir := t.TempDir()
	t.Setenv("RINGMASTER_CONFIG_DIR", configDir)
	projectRoot := t.TempDir()

	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	skill := func(root, name string) {
		write(filepath.Join(root, name, "SKILL.md"), "---\ndescription: "+name+" rules\n---\n\nBODY-OF-"+name+"\n")
	}
	skill(filepath.Join(configDir, "skills"), "go-style")
	skill(filepath.Join(configDir, "skills"), "fleet-off")
	skill(filepath.Join(projectRoot, ".ringmaster", "skills"), "deploy")
	skill(filepath.Join(projectRoot, ".ringmaster", "skills"), "project-off")
	write(filepath.Join(projectRoot, ".ringmaster", "BRIEF.md"), "# Brief\nBuild it.\n")
	write(filepath.Join(projectRoot, "ringmaster.toml"), "name = \"p\"\n\n[skills]\ndisabled = [\"project-off\"]\n")

	var system string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		for _, m := range req.Messages {
			if m.Role == "system" {
				system = m.Content
			}
		}
		plan := `{"project":"p","tasks":[{"id":"T01","title":"Do it","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"description":"d"}],"open_questions":[],"assumptions":[]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": plan}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	cfg.Providers["mock"] = config.ProviderConfig{Type: "openai", BaseURL: ts.URL, APIKeyEnv: "MOCK_KEY"}
	cfg.Agents["m"] = config.AgentConfig{Provider: "mock", Model: "x", Role: "master"}
	cfg.Agents["w"] = config.AgentConfig{Provider: "mock", Model: "x", Role: "worker", ToolCalling: true}
	cfg.Skills.Disabled = []string{"fleet-off"}

	if _, _, err := GeneratePlan(context.Background(), projectRoot, cfg, cfg.Agents["m"], llm.NewProvider("openai", ts.URL, "MOCK_KEY")); err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	for _, want := range []string{"INSTALLED SKILLS", "- deploy: deploy rules", "- go-style: go-style rules"} {
		if !strings.Contains(system, want) {
			t.Errorf("planner prompt lacks %q", want)
		}
	}
	for _, unwanted := range []string{"fleet-off", "project-off", "BODY-OF-"} {
		if strings.Contains(system, unwanted) {
			t.Errorf("planner prompt should not contain %q", unwanted)
		}
	}
	if strings.Contains(MasterSystemPrompt(cfg, "w", nil), "SKILLS") {
		t.Error("with no skills the planner prompt should not mention them")
	}
}

// planServer answers each planning call with the next plan in replies (the
// last one repeats) and records what the master was told.
func planServer(t *testing.T, replies ...string) (url string, calls *int, lastUser *string) {
	t.Helper()
	calls, lastUser = new(int), new(string)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if n := len(req.Messages); n > 0 {
			*lastUser = req.Messages[n-1].Content
		}
		reply := replies[len(replies)-1]
		if *calls < len(replies) {
			reply = replies[*calls]
		}
		*calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": reply}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	t.Cleanup(ts.Close)
	return ts.URL, calls, lastUser
}

func planWithAcceptance(command string) string {
	data, _ := json.Marshal(map[string]any{
		"project": "p",
		"tasks": []map[string]any{{
			"id": "T01", "title": "Docs", "type": "docs", "depends_on": []string{}, "worker": "w",
			"max_retries": 2, "acceptance": []string{command}, "description": "d",
		}},
		"open_questions": []any{}, "assumptions": []any{},
	})
	return string(data)
}

func planningProject(t *testing.T, url string) (string, *config.Config) {
	t.Helper()
	t.Setenv("MOCK_KEY", "k")
	t.Setenv("RINGMASTER_CONFIG_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ringmaster"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ringmaster", "BRIEF.md"), []byte("# Brief\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Providers["mock"] = config.ProviderConfig{Type: "openai", BaseURL: url, APIKeyEnv: "MOCK_KEY"}
	cfg.Agents["m"] = config.AgentConfig{Provider: "mock", Model: "x", Role: "master"}
	cfg.Agents["w"] = config.AgentConfig{Provider: "mock", Model: "x", Role: "worker", ToolCalling: true}
	return root, cfg
}

const shellAcceptance = `python -c "import pathlib; t=pathlib.Path('README.md').read_text(); assert '--json' in t"`

// The master wrote an acceptance command that needs a shell. It is told which
// one and why, and its corrected plan is the one that is saved.
func TestPlannerHasUnrunnableAcceptanceCorrected(t *testing.T) {
	url, calls, lastUser := planServer(t, planWithAcceptance(shellAcceptance), planWithAcceptance("pytest -q"))
	root, cfg := planningProject(t, url)

	plan, _, err := GeneratePlan(context.Background(), root, cfg, cfg.Agents["m"], llm.NewProvider("openai", url, "MOCK_KEY"))
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if *calls != 2 {
		t.Fatalf("master was called %d time(s), want 2 (plan, then correction)", *calls)
	}
	for _, want := range []string{"T01", "python -c", "NOT through a shell", "no quotes"} {
		if !strings.Contains(*lastUser, want) {
			t.Errorf("the correction request does not mention %q:\n%s", want, *lastUser)
		}
	}
	if got := plan.Tasks[0].Acceptance; len(got) != 1 || got[0] != "pytest -q" {
		t.Fatalf("saved acceptance = %v, want the corrected command", got)
	}
	saved, err := os.ReadFile(filepath.Join(root, ".ringmaster", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "python -c") {
		t.Fatalf("plan.json still holds the unrunnable command:\n%s", saved)
	}
	specs, _ := filepath.Glob(filepath.Join(root, ".ringmaster", "tasks", "T01*.md"))
	if len(specs) != 1 {
		t.Fatalf("task spec files = %v", specs)
	}
	if spec, _ := os.ReadFile(specs[0]); !strings.Contains(string(spec), "pytest -q") || strings.Contains(string(spec), "python -c") {
		t.Fatalf("task spec holds the wrong acceptance command:\n%s", spec)
	}
}

// A master that never corrects itself must not leave a plan the user could
// approve and run.
func TestPlannerRefusesPlanWhoseAcceptanceCannotRun(t *testing.T) {
	url, calls, _ := planServer(t, planWithAcceptance(shellAcceptance))
	root, cfg := planningProject(t, url)

	_, _, err := GeneratePlan(context.Background(), root, cfg, cfg.Agents["m"], llm.NewProvider("openai", url, "MOCK_KEY"))
	if err == nil {
		t.Fatal("a plan with an unrunnable acceptance command was accepted")
	}
	if !strings.Contains(err.Error(), "T01") || !strings.Contains(err.Error(), "python -c") {
		t.Errorf("error does not name the task and command: %v", err)
	}
	if *calls != 3 {
		t.Errorf("master was called %d time(s), want 3", *calls)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".ringmaster", "plan.json")); statErr == nil {
		t.Error("plan.json was written for a plan that was refused")
	}
}

func TestPlainAcceptanceCommandsAreLeftAlone(t *testing.T) {
	url, calls, _ := planServer(t, planWithAcceptance("go test ./..."))
	root, cfg := planningProject(t, url)

	if _, _, err := GeneratePlan(context.Background(), root, cfg, cfg.Agents["m"], llm.NewProvider("openai", url, "MOCK_KEY")); err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	if *calls != 1 {
		t.Errorf("master was called %d time(s) for a plan with nothing to correct", *calls)
	}
}

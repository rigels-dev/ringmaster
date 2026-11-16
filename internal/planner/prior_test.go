package planner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ringmaster/internal/llm"
)

func TestFitIDs(t *testing.T) {
	prior := priorWork{
		used: map[string]bool{"T01": true, "T02": true, "T03": true},
		done: []doneTask{{ID: "T01", Title: "Store"}, {ID: "T02", Title: "API"}},
	}

	// The master started from T01 again: everything moves past what is in use
	ids, deps := prior.fitIDs(
		[]string{"T01", "T02", "T03"},
		[][]string{{}, {"T01"}, {"T01", "T02"}},
	)
	if want := []string{"T04", "T05", "T06"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if want := [][]string{{}, {"T04"}, {"T04", "T05"}}; !reflect.DeepEqual(deps, want) {
		t.Errorf("deps = %v, want %v", deps, want)
	}

	// The master numbered from the first free ID: nothing moves, and a
	// dependency on finished work is dropped because that work is in main
	ids, deps = prior.fitIDs(
		[]string{"T04", "T05"},
		[][]string{{"T02"}, {"T04", "T01"}},
	)
	if want := []string{"T04", "T05"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if want := [][]string{{}, {"T04"}}; !reflect.DeepEqual(deps, want) {
		t.Errorf("deps = %v, want %v", deps, want)
	}

	// A dependency on a task that is neither in the plan nor done is kept, so validation reports it
	_, deps = prior.fitIDs([]string{"T04"}, [][]string{{"T03"}})
	if want := [][]string{{"T03"}}; !reflect.DeepEqual(deps, want) {
		t.Errorf("deps = %v, want %v", deps, want)
	}

	// A new project is left exactly as the master wrote it
	ids, deps = priorWork{used: map[string]bool{}}.fitIDs([]string{"T01", "T02"}, [][]string{{}, {"T01"}})
	if !reflect.DeepEqual(ids, []string{"T01", "T02"}) || !reflect.DeepEqual(deps, [][]string{{}, {"T01"}}) {
		t.Errorf("a first plan was changed: %v %v", ids, deps)
	}
}

func TestPriorWorkPromptSection(t *testing.T) {
	if got := (priorWork{used: map[string]bool{}}).promptSection(); got != "" {
		t.Errorf("a new project should add nothing to the prompt, got %q", got)
	}

	prior := priorWork{
		used: map[string]bool{"T01": true, "T02": true, "T05": true},
		done: []doneTask{{ID: "T01", Title: "Link store"}, {ID: "T02"}},
	}
	got := prior.promptSection()
	for _, want := range []string{"ALREADY DONE", "- T01: Link store", "- T02: (no title recorded)", "from T06"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt section lacks %q:\n%s", want, got)
		}
	}
}

// A second plan on a finished project: the master is told what is done, and
// its tasks get IDs and branches that do not clash with the first round.
func TestSecondPlanContinuesAfterTheFirst(t *testing.T) {
	second := `{"project":"p","tasks":[
	 {"id":"T01","title":"Add due dates","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"branch":"feat/due-dates","description":"d"},
	 {"id":"T02","title":"Document due dates","type":"docs","depends_on":["T01"],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"description":"d"}],
	 "open_questions":[],"assumptions":[]}`
	url, _, lastUser := planServer(t, second)
	root, cfg := planningProject(t, url)

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, ".ringmaster", name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// The first round: two tasks done; the first ran before titles were recorded
	write("plan.json", `{"project":"p","tasks":[
	 {"id":"T01","title":"Link store","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."]},
	 {"id":"T02","title":"HTTP API","type":"backend","depends_on":["T01"],"worker":"w","max_retries":2,"acceptance":["go test ./..."]}]}`)
	write("state.json", `{"task_states":{
	 "T01":{"id":"T01","status":"COMPLETED"},
	 "T02":{"id":"T02","status":"COMPLETED","title":"HTTP API handlers","branch":"feat/http-api"}}}`)

	plan, _, err := GeneratePlan(context.Background(), root, cfg, cfg.Agents["m"], llm.NewProvider("openai", url, "MOCK_KEY"))
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	for _, want := range []string{"ALREADY DONE", "- T01: Link store", "- T02: HTTP API handlers", "from T03"} {
		if !strings.Contains(*lastUser, want) {
			t.Errorf("the master was not told %q", want)
		}
	}

	if len(plan.Tasks) != 2 || plan.Tasks[0].ID != "T03" || plan.Tasks[1].ID != "T04" {
		t.Fatalf("task IDs = %s, %s; want T03, T04", plan.Tasks[0].ID, plan.Tasks[1].ID)
	}
	if !reflect.DeepEqual(plan.Tasks[1].DependsOn, []string{"T03"}) {
		t.Errorf("T04 depends on %v, want [T03]", plan.Tasks[1].DependsOn)
	}
	if plan.Tasks[0].Branch != "feat/due-dates" || plan.Tasks[1].Branch != "docs/document-due-dates" {
		t.Errorf("branches = %q, %q", plan.Tasks[0].Branch, plan.Tasks[1].Branch)
	}

	specs, _ := filepath.Glob(filepath.Join(root, ".ringmaster", "tasks", "T0[34]-*.md"))
	if len(specs) != 2 {
		t.Errorf("task files for the new IDs = %v", specs)
	}
	if spec, _ := os.ReadFile(specs[len(specs)-1]); !strings.Contains(string(spec), `depends_on: ["T03"]`) {
		t.Errorf("the task file still names the old dependency:\n%s", spec)
	}
}

// Two tasks the master gave the same branch still end up on different ones.
func TestPlanBranchesAreUnique(t *testing.T) {
	plan := `{"project":"p","tasks":[
	 {"id":"T01","title":"A","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"branch":"feat/api","description":"d"},
	 {"id":"T02","title":"B","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"branch":"feat/api","description":"d"},
	 {"id":"T03","title":"C","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."],"branch":"Feat: API","description":"d"}],
	 "open_questions":[],"assumptions":[]}`
	url, _, _ := planServer(t, plan)
	root, cfg := planningProject(t, url)

	got, _, err := GeneratePlan(context.Background(), root, cfg, cfg.Agents["m"], llm.NewProvider("openai", url, "MOCK_KEY"))
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}
	branches := []string{got.Tasks[0].Branch, got.Tasks[1].Branch, got.Tasks[2].Branch}
	if want := []string{"feat/api", "feat/api-2", "feat/api-3"}; !reflect.DeepEqual(branches, want) {
		t.Errorf("branches = %v, want %v", branches, want)
	}
}

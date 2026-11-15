package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/release"
)

func writeProjectFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".ringmaster", name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const twoTaskPlan = `{"project":"p","tasks":[
 {"id":"T01","title":"A","type":"backend","depends_on":[],"worker":"w","max_retries":2,"acceptance":["go test ./..."]},
 {"id":"T02","title":"B","type":"docs","depends_on":["T01"],"worker":"w","max_retries":2,"acceptance":["go test ./..."]}]}`

func TestUnfinishedTasks(t *testing.T) {
	dir := t.TempDir()
	if got := unfinishedTasks(dir); len(got) != 0 {
		t.Fatalf("a project without a plan has unfinished tasks: %v", got)
	}

	writeProjectFile(t, dir, "plan.json", twoTaskPlan)
	if got := strings.Join(unfinishedTasks(dir), ","); got != "T01,T02" {
		t.Fatalf("with a plan and no state: %s", got)
	}

	writeProjectFile(t, dir, "state.json", `{"task_states":{"T01":{"status":"COMPLETED"},"T02":{"status":"NEEDS_REVIEW"}}}`)
	if got := strings.Join(unfinishedTasks(dir), ","); got != "T02" {
		t.Fatalf("with one task done: %s", got)
	}

	writeProjectFile(t, dir, "state.json", `{"task_states":{"T01":{"status":"COMPLETED"},"T02":{"status":"COMPLETED"}}}`)
	if got := unfinishedTasks(dir); len(got) != 0 {
		t.Fatalf("with every task done: %v", got)
	}
}

func TestRecommendedKindFollowsWhatWasBuilt(t *testing.T) {
	dir := t.TempDir()
	next := release.Suggest("v0.1.0")

	if got := recommendedKind(dir, release.Suggest("")); got != release.KindFirst {
		t.Errorf("without any tag: %q", got)
	}

	writeProjectFile(t, dir, "plan.json", twoTaskPlan)
	if got := recommendedKind(dir, next); got != release.KindMinor {
		t.Errorf("a plan with a backend task: %q, want minor", got)
	}

	writeProjectFile(t, dir, "plan.json", strings.ReplaceAll(twoTaskPlan, `"backend"`, `"test"`))
	if got := recommendedKind(dir, next); got != release.KindPatch {
		t.Errorf("a plan of only docs and tests: %q, want patch", got)
	}

	if got := recommendedKind(dir, release.Suggest("release-november")); got != "" {
		t.Errorf("a scheme Ringmaster cannot read: %q, want no recommendation", got)
	}
}

func TestReleaseView(t *testing.T) {
	view := buildReleaseView(releaseReport{
		Info:       release.Info{Latest: "v0.1.0", Suggestions: release.Suggest("v0.1.0"), Recommended: release.KindMinor},
		Unfinished: []string{"T03"},
	})
	for _, want := range []string{"Last release: v0.1.0", "★ v0.2.0", "v0.1.1", "not done yet: T03", "yours to choose"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}

	view = buildReleaseView(releaseReport{Info: release.Info{Latest: "v0.2.0", Current: "v0.2.0"}})
	if !strings.Contains(view, "already released as v0.2.0") || strings.Contains(view, "Suggestions") {
		t.Errorf("a released project should have nothing to suggest:\n%s", view)
	}

	view = buildReleaseView(releaseReport{Info: release.Info{Latest: "stable", Suggestions: []release.Suggestion{}}})
	if !strings.Contains(view, "not a version number") {
		t.Errorf("an unreadable scheme should be said plainly:\n%s", view)
	}
}

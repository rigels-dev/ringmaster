package steps

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ringmaster/internal/release"
	"ringmaster/internal/schemas"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func newProject(t *testing.T) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Ada\n\temail = ada@example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "x")
		os.Unsetenv(name)
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "initial")
	if err := os.MkdirAll(filepath.Join(dir, ".ringmaster"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// mergeTask does what the merge queue does: a task branch with one commit,
// merged into main with a Ringmaster-Task trailer.
func mergeTask(t *testing.T, dir, id string) string {
	t.Helper()
	branch := "feat/" + strings.ToLower(id)
	git(t, dir, "checkout", "-q", "-b", branch)
	if err := os.WriteFile(filepath.Join(dir, id+".txt"), []byte(id), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", id+": work", "-m", "Ringmaster-Task: "+id)
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--no-ff", branch, "-m", "Merge task "+id+" into main [Ringmaster]", "-m", "Ringmaster-Task: "+id+"\nRingmaster-Branch: "+branch)
	return git(t, dir, "rev-parse", "HEAD")
}

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	data, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, ".ringmaster", name), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func setPlan(t *testing.T, dir string, ids ...string) {
	t.Helper()
	plan := schemas.Plan{Project: "p"}
	for _, id := range ids {
		plan.Tasks = append(plan.Tasks, schemas.PlanTask{ID: id, Title: id, Type: schemas.TaskTypeBackend, DependsOn: []string{}, Worker: "w", MaxRetries: 2, Acceptance: []string{"go test ./..."}})
	}
	writeJSON(t, dir, "plan.json", plan)
}

func setStates(t *testing.T, dir string, states map[string]schemas.TaskExecutionStatus) {
	t.Helper()
	finish := time.Date(2026, 11, 17, 7, 17, 54, 0, time.UTC)
	ts := map[string]schemas.TaskState{}
	for id, st := range states {
		ts[id] = schemas.TaskState{ID: id, Status: st, FinishAt: &finish}
	}
	writeJSON(t, dir, "state.json", schemas.ProjectState{Project: "p", TaskStates: ts})
}

func TestAFinishedPlanBecomesAStepOnce(t *testing.T) {
	ctx := context.Background()
	dir := newProject(t)

	if list, err := List(ctx, dir); err != nil || len(list) != 0 {
		t.Fatalf("a project without a plan: %v %v", list, err)
	}

	mergeTask(t, dir, "T01")
	setPlan(t, dir, "T01", "T02")
	setStates(t, dir, map[string]schemas.TaskExecutionStatus{"T01": schemas.TaskStateCompleted, "T02": schemas.TaskStateRunning})
	if list, _ := List(ctx, dir); len(list) != 0 {
		t.Fatalf("an unfinished plan became a step: %+v", list)
	}

	end := mergeTask(t, dir, "T02")
	setStates(t, dir, map[string]schemas.TaskExecutionStatus{"T01": schemas.TaskStateCompleted, "T02": schemas.TaskStateCompleted})
	list, err := List(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("steps = %+v, want one", list)
	}
	s := list[0]
	if s.Number != 1 || s.Title() != "Step 1" || !reflect.DeepEqual(s.Tasks, []string{"T01", "T02"}) || s.Commit != end || s.Closed || len(s.Releases) != 0 {
		t.Fatalf("step = %+v", s)
	}
	if s.FinishedAt.Hour() != 7 {
		t.Errorf("finished_at = %v, want the time the last task finished", s.FinishedAt)
	}

	// Listing again, or after the plan changed without new work, adds nothing
	setPlan(t, dir, "T02", "T01") // same tasks, other order, so another hash
	if list, _ := List(ctx, dir); len(list) != 1 {
		t.Fatalf("the same work was recorded twice: %+v", list)
	}
}

func TestStepsFollowEachOtherAndReadTheirReleasesFromGit(t *testing.T) {
	ctx := context.Background()
	dir := newProject(t)

	firstEnd := mergeTask(t, dir, "T01")
	setPlan(t, dir, "T01")
	setStates(t, dir, map[string]schemas.TaskExecutionStatus{"T01": schemas.TaskStateCompleted})
	if _, err := List(ctx, dir); err != nil {
		t.Fatal(err)
	}

	secondEnd := mergeTask(t, dir, "T02")
	mergeTask(t, dir, "T03")
	secondEnd = git(t, dir, "rev-parse", "HEAD")
	setPlan(t, dir, "T02", "T03")
	setStates(t, dir, map[string]schemas.TaskExecutionStatus{
		"T01": schemas.TaskStateCompleted, "T02": schemas.TaskStateCompleted, "T03": schemas.TaskStateCompleted,
	})
	list, err := List(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[1].Number != 2 || list[1].Commit != secondEnd || list[0].Commit != firstEnd {
		t.Fatalf("steps = %+v", list)
	}

	// Step 1 is released after main has moved on: the tag lands where it ended
	if err := release.TagAt(ctx, dir, "v0.1.0", list[0].Commit); err != nil {
		t.Fatal(err)
	}
	if git(t, dir, "rev-parse", "v0.1.0^{commit}") != firstEnd {
		t.Fatal("the release tag is not on the step's last commit")
	}
	list, _ = List(ctx, dir)
	if !reflect.DeepEqual(list[0].Releases, []string{"v0.1.0"}) || len(list[1].Releases) != 0 {
		t.Fatalf("releases = %v / %v", list[0].Releases, list[1].Releases)
	}

	// Releases are never stored: they come from git each time
	data, _ := os.ReadFile(filepath.Join(dir, ".ringmaster", "steps.json"))
	if strings.Contains(string(data), "v0.1.0") {
		t.Errorf("a release was written to steps.json:\n%s", data)
	}
}

func TestNameAndCloseAStep(t *testing.T) {
	ctx := context.Background()
	dir := newProject(t)
	mergeTask(t, dir, "T01")
	setPlan(t, dir, "T01")
	setStates(t, dir, map[string]schemas.TaskExecutionStatus{"T01": schemas.TaskStateCompleted})

	if err := Rename(ctx, dir, 1, "  First   version "); err != nil {
		t.Fatal(err)
	}
	if err := SetClosed(ctx, dir, 1, true); err != nil {
		t.Fatal(err)
	}
	s, err := Get(ctx, dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "First version" || s.Title() != "First version" || !s.Closed {
		t.Fatalf("step = %+v", s)
	}

	if err := SetClosed(ctx, dir, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := Rename(ctx, dir, 1, ""); err != nil {
		t.Fatal(err)
	}
	if s, _ = Get(ctx, dir, 1); s.Closed || s.Title() != "Step 1" {
		t.Fatalf("after undoing: %+v", s)
	}

	for _, n := range []int{0, 2, -1} {
		if err := SetClosed(ctx, dir, n, true); err == nil {
			t.Errorf("SetClosed(%d) succeeded", n)
		}
	}
	if err := Rename(ctx, dir, 1, strings.Repeat("x", 81)); err == nil {
		t.Error("an over-long name was accepted")
	}
}

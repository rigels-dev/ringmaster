// Package steps records each finished plan as a step of the project: Step 1
// built the first version, Step 2 the next change, and so on. A step ends at
// the commit on main where its last task was merged. It may be released, by
// tagging that commit, or finished without a release; either way it stays in
// the project's history.
//
// Steps live in .ringmaster/steps.json, next to the rest of Ringmaster's
// state, so they are never committed or pushed. Releases are git tags and
// are read from git, so a tag made by hand counts too.
package steps

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

const mainBranch = "main"

// Step is one finished plan.
type Step struct {
	Number int `json:"number"`
	// Name is the user's; "" shows as "Step <Number>".
	Name     string   `json:"name,omitempty"`
	PlanHash string   `json:"plan_hash"`
	Tasks    []string `json:"tasks"`
	// Commit is where the step ends on main: the merge of its last task.
	Commit     string    `json:"commit"`
	FinishedAt time.Time `json:"finished_at"`
	// Closed is set when the user finished the step without a release, so
	// Ringmaster stops offering one.
	Closed bool `json:"closed,omitempty"`

	// Releases are the tags on Commit; filled in when steps are listed.
	Releases []string `json:"releases"`
}

// Title is how the step is shown: its name, or "Step <n>".
func (s Step) Title() string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("Step %d", s.Number)
}

type file struct {
	Steps []Step `json:"steps"`
}

func path(projectRoot string) string {
	return filepath.Join(projectRoot, ".ringmaster", "steps.json")
}

func load(projectRoot string) []Step {
	f, err := storage.ReadJSON[file](path(projectRoot))
	if err != nil || f == nil {
		return nil
	}
	return f.Steps
}

func save(projectRoot string, list []Step) error {
	for i := range list {
		list[i].Releases = nil // read from git, never stored
	}
	return storage.WriteJSONAtomic(path(projectRoot), file{Steps: list})
}

// List records the current plan as a new step when all its tasks are done
// and it is not recorded yet, and returns every step, oldest first, with
// their releases read from git.
func List(ctx context.Context, projectRoot string) ([]Step, error) {
	list := load(projectRoot)
	if step, ok := finishedPlan(ctx, projectRoot, list); ok {
		step.Number = len(list) + 1
		list = append(list, step)
		if err := save(projectRoot, list); err != nil {
			return nil, err
		}
	}
	for i := range list {
		list[i].Releases = tagsAt(ctx, projectRoot, list[i].Commit)
	}
	return list, nil
}

// finishedPlan returns the current plan as a step, if every task of it is
// done and merged and no step holds those tasks yet.
func finishedPlan(ctx context.Context, projectRoot string, recorded []Step) (Step, bool) {
	plan, err := storage.ReadJSON[schemas.Plan](filepath.Join(projectRoot, ".ringmaster", "plan.json"))
	if err != nil || plan == nil || len(plan.Tasks) == 0 {
		return Step{}, false
	}
	state, err := storage.ReadJSON[schemas.ProjectState](filepath.Join(projectRoot, ".ringmaster", "state.json"))
	if err != nil || state == nil {
		return Step{}, false
	}

	var ids []string
	var finished time.Time
	for _, t := range plan.Tasks {
		ts := state.TaskStates[t.ID]
		if ts.Status != schemas.TaskStateCompleted {
			return Step{}, false
		}
		ids = append(ids, t.ID)
		if ts.FinishAt != nil && ts.FinishAt.After(finished) {
			finished = *ts.FinishAt
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// Already recorded: the plan changed after it finished (an automaton was
	// reassigned, say), but its tasks are the same work
	for _, s := range recorded {
		if sameTasks(s.Tasks, ids) {
			return Step{}, false
		}
	}

	commit := lastMergeOf(ctx, projectRoot, ids)
	if commit == "" {
		return Step{}, false
	}
	hash, _ := plan.PlanHash()
	if finished.IsZero() {
		finished = time.Now().UTC()
	}
	return Step{PlanHash: hash, Tasks: ids, Commit: commit, FinishedAt: finished}, true
}

func sameTasks(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			return false
		}
	}
	return true
}

// lastMergeOf finds the newest commit on main's own line that merged one of
// the tasks, by its Ringmaster-Task trailer: the step's end.
func lastMergeOf(ctx context.Context, projectRoot string, tasks []string) string {
	want := map[string]bool{}
	for _, id := range tasks {
		want[id] = true
	}
	out, err := workspace.RunGit(ctx, projectRoot, "log", "--first-parent", "--format=%H%x1f%(trailers:key=Ringmaster-Task,valueonly,separator=%x2C)", mainBranch)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		sha, trailer, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue
		}
		for _, id := range strings.Split(trailer, ",") {
			if want[strings.TrimSpace(id)] {
				return strings.TrimSpace(sha)
			}
		}
	}
	return ""
}

func tagsAt(ctx context.Context, projectRoot, commit string) []string {
	tags := []string{}
	if commit == "" {
		return tags
	}
	out, err := workspace.RunGit(ctx, projectRoot, "tag", "--points-at", commit, "--sort=-creatordate")
	if err != nil {
		return tags
	}
	return append(tags, strings.Fields(out)...)
}

// Get returns step n.
func Get(ctx context.Context, projectRoot string, n int) (Step, error) {
	list, err := List(ctx, projectRoot)
	if err != nil {
		return Step{}, err
	}
	if n < 1 || n > len(list) {
		return Step{}, fmt.Errorf("there is no step %d (the project has %d)", n, len(list))
	}
	return list[n-1], nil
}

// Rename gives step n a name; "" goes back to "Step n".
func Rename(ctx context.Context, projectRoot string, n int, name string) error {
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) > 80 {
		return errors.New("the name is too long (80 characters at most)")
	}
	return update(ctx, projectRoot, n, func(s *Step) { s.Name = name })
}

// SetClosed marks step n as finished without a release, or undoes that.
func SetClosed(ctx context.Context, projectRoot string, n int, closed bool) error {
	return update(ctx, projectRoot, n, func(s *Step) { s.Closed = closed })
}

func update(ctx context.Context, projectRoot string, n int, fn func(*Step)) error {
	list, err := List(ctx, projectRoot)
	if err != nil {
		return err
	}
	if n < 1 || n > len(list) {
		return fmt.Errorf("there is no step %d (the project has %d)", n, len(list))
	}
	fn(&list[n-1])
	return save(projectRoot, list)
}

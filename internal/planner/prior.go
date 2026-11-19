package planner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

// priorWork is what earlier plans left in a project: tasks that were done
// and merged, and every task ID that has been used.
type priorWork struct {
	done []doneTask
	used map[string]bool // every ID a task state or an rm/ branch already has
	// files are the project's manifests on main, as "--- name ---" sections
	files string
}

// manifestFiles state a project's language version, dependencies and build.
// Must match MANIFEST_FILES in web/server/utils/chat.ts.
var manifestFiles = []string{"go.mod", "package.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "Gemfile", "composer.json", "Dockerfile"}

const maxManifestChars = 2000

// mainManifests reads the manifests on main: what the project really uses,
// which may differ from what an early request said.
func mainManifests(ctx context.Context, projectRoot string) string {
	var parts []string
	for _, file := range manifestFiles {
		out, err := workspace.RunGit(ctx, projectRoot, "show", "main:"+file)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(out)
		if content == "" {
			continue
		}
		if len(content) > maxManifestChars {
			content = content[:maxManifestChars] + "\n…"
		}
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", file, content))
	}
	return strings.Join(parts, "\n\n")
}

type doneTask struct {
	ID, Title string
}

var taskIDPattern = regexp.MustCompile(`^T(\d{2,3})$`)

// loadPriorWork reads the project's state. A new project has none.
func loadPriorWork(ctx context.Context, projectRoot string) priorWork {
	prior := priorWork{used: map[string]bool{}}

	// Titles of tasks that ran before titles were recorded come from the plan they belonged to
	titles := map[string]string{}
	if old, err := ReadPlanFile(filepath.Join(projectRoot, ".ringmaster", "plan.json")); err == nil && old != nil {
		for _, t := range old.Tasks {
			titles[t.ID] = t.Title
		}
	}

	state, err := storage.ReadJSON[schemas.ProjectState](filepath.Join(projectRoot, ".ringmaster", "state.json"))
	if err == nil && state != nil {
		for id, ts := range state.TaskStates {
			prior.used[id] = true
			if ts.Status == schemas.TaskStateCompleted {
				title := ts.Title
				if title == "" {
					title = titles[id]
				}
				prior.done = append(prior.done, doneTask{ID: id, Title: title})
			}
		}
	}

	// Branches from before task states carried their branch: rm/T01 means T01 is taken
	if out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+strings.TrimSuffix(schemas.LegacyBranchPrefix, "/")); err == nil {
		for _, name := range strings.Fields(out) {
			if id := strings.TrimPrefix(name, schemas.LegacyBranchPrefix); taskIDPattern.MatchString(id) {
				prior.used[id] = true
			}
		}
	}

	sort.Slice(prior.done, func(i, j int) bool { return taskNumber(prior.done[i].ID) < taskNumber(prior.done[j].ID) })
	if len(prior.done) > 0 {
		prior.files = mainManifests(ctx, projectRoot)
	}
	return prior
}

func taskNumber(id string) int {
	if m := taskIDPattern.FindStringSubmatch(id); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func formatTaskID(n int) string {
	return fmt.Sprintf("T%02d", n)
}

// firstFreeID is the ID after the highest one in use.
func (p priorWork) firstFreeID() string {
	highest := 0
	for id := range p.used {
		if n := taskNumber(id); n > highest {
			highest = n
		}
	}
	return formatTaskID(highest + 1)
}

// promptSection tells the master what is already built. It is empty for a
// project with no earlier work, so a first plan reads as it always did.
func (p priorWork) promptSection() string {
	if len(p.used) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n# ALREADY DONE\n")
	if len(p.done) == 0 {
		sb.WriteString("No task has been completed yet, but some task IDs are taken.\n")
	} else {
		sb.WriteString("These tasks are finished and merged into main. Their code is in the repository; do not plan them again:\n")
		for _, t := range p.done {
			title := t.Title
			if title == "" {
				title = "(no title recorded)"
			}
			fmt.Fprintf(&sb, "- %s: %s\n", t.ID, title)
		}
	}
	fmt.Fprintf(&sb, "Number the tasks of this plan from %s.\n", p.firstFreeID())
	if p.files != "" {
		sb.WriteString("\n# PROJECT FILES ON MAIN\n")
		sb.WriteString("These files are in the repository now. They are the facts about language version, dependencies and build: " +
			"plan and write task specs to match them (the go directive in go.mod, say), even where an earlier request in the brief says otherwise, " +
			"unless the brief asks to change them.\n\n")
		sb.WriteString(p.files + "\n")
	}
	return sb.String()
}

// fitIDs makes a new plan's task IDs and dependencies fit the project. When
// an ID is already in use, every task is renumbered from the first free ID,
// in order, and dependencies between the plan's tasks follow. A dependency
// on a task that is already done and merged is dropped: its work is in main,
// which every task starts from. It returns the slices it was given when
// nothing needs changing.
func (p priorWork) fitIDs(ids []string, deps [][]string) ([]string, [][]string) {
	inPlan := map[string]bool{}
	collides := false
	for _, id := range ids {
		inPlan[id] = true
		if p.used[id] {
			collides = true
		}
	}

	rename := map[string]string{}
	if collides {
		next := taskNumber(p.firstFreeID())
		for _, id := range ids {
			if _, seen := rename[id]; !seen {
				rename[id] = formatTaskID(next)
				next++
			}
		}
	}

	done := map[string]bool{}
	for _, t := range p.done {
		done[t.ID] = true
	}

	newIDs := make([]string, len(ids))
	newDeps := make([][]string, len(deps))
	for i, id := range ids {
		newIDs[i] = id
		if to, ok := rename[id]; ok {
			newIDs[i] = to
		}
		kept := []string{}
		for _, dep := range deps[i] {
			switch {
			case inPlan[dep]:
				if to, ok := rename[dep]; ok {
					dep = to
				}
				kept = append(kept, dep)
			case done[dep]:
				// already merged into main
			default:
				kept = append(kept, dep) // unknown: plan validation reports it
			}
		}
		newDeps[i] = kept
	}
	return newIDs, newDeps
}

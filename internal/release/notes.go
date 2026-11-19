package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

// SummaryPlaceholder marks where the release's summary goes in a draft. It is
// an HTML comment, so a draft published as it is shows nothing there; it is
// removed before the notes are used anyway.
const SummaryPlaceholder = "<!-- Summary: a sentence or two on what this release is for. Write it here, or let AI write it. -->"

// MaxNotesBytes bounds release notes; a tag message or a release page has no
// use for more.
const MaxNotesBytes = 20000

// NoteTask is one finished task, as release notes name it.
type NoteTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Branch string `json:"branch,omitempty"`
}

// Sections of a draft, by the kind a task's branch names, in the order shown.
var sections = []struct{ prefix, heading string }{
	{"feat/", "Features"},
	{"fix/", "Fixes"},
	{"docs/", "Documentation"},
	{"test/", "Tests"},
}

const otherHeading = "Other changes"

// DraftNotes writes release notes from the tasks a release holds: a
// placeholder for the summary, then one line per task under the kind of
// change it is. Nothing is invented: every line comes from a task.
func DraftNotes(tasks []NoteTask) string {
	groups := map[string][]NoteTask{}
	for _, t := range tasks {
		heading := otherHeading
		for _, s := range sections {
			if strings.HasPrefix(t.Branch, s.prefix) {
				heading = s.heading
				break
			}
		}
		groups[heading] = append(groups[heading], t)
	}

	var sb strings.Builder
	sb.WriteString(SummaryPlaceholder + "\n")
	order := []string{}
	for _, s := range sections {
		order = append(order, s.heading)
	}
	for _, heading := range append(order, otherHeading) {
		if len(groups[heading]) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n### %s\n\n", heading)
		for _, t := range groups[heading] {
			title := strings.TrimSpace(t.Title)
			if title == "" {
				title = "(untitled task)"
			}
			fmt.Fprintf(&sb, "- %s (%s)\n", title, t.ID)
		}
	}
	return sb.String()
}

// InsertSummary puts summary where the draft's placeholder is. When the user
// has removed the placeholder, the summary goes first, above what they wrote.
func InsertSummary(notes, summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return notes
	}
	if strings.Contains(notes, SummaryPlaceholder) {
		return strings.Replace(notes, SummaryPlaceholder, summary, 1)
	}
	if strings.TrimSpace(notes) == "" {
		return summary + "\n"
	}
	return summary + "\n\n" + strings.TrimLeft(notes, "\n")
}

// CleanNotes is what is published: the notes without the unfilled
// placeholder, with line endings and surrounding blank lines tidied.
func CleanNotes(notes string) string {
	notes = strings.ReplaceAll(notes, "\r\n", "\n")
	notes = strings.Replace(notes, SummaryPlaceholder, "", 1)
	return strings.TrimSpace(notes)
}

// CheckNotes refuses notes that are too long to be a tag message.
func CheckNotes(notes string) error {
	if len(notes) > MaxNotesBytes {
		return fmt.Errorf("the release notes are too long (%d characters at most)", MaxNotesBytes)
	}
	return nil
}

// TagMessage is the annotated tag's message: a subject line, then the notes.
func TagMessage(version, notes string) string {
	msg := "Release " + version
	if notes = CleanNotes(notes); notes != "" {
		msg += "\n\n" + notes
	}
	return msg + "\n"
}

// Title is what a release is called on its page: the version, and the step's
// name when it has one.
func Title(version, name string) string {
	if name = strings.TrimSpace(name); name != "" {
		return version + " · " + name
	}
	return version
}

// TasksOf names the given tasks from the project's state, where each task's
// title and branch were recorded when it ran, or from the plan for tasks
// that ran before that.
func TasksOf(projectRoot string, ids []string) []NoteTask {
	plan, _ := storage.ReadJSON[schemas.Plan](filepath.Join(projectRoot, ".ringmaster", "plan.json"))
	planned := map[string]schemas.PlanTask{}
	if plan != nil {
		for _, t := range plan.Tasks {
			planned[t.ID] = t
		}
	}
	state, _ := storage.ReadJSON[schemas.ProjectState](filepath.Join(projectRoot, ".ringmaster", "state.json"))

	tasks := []NoteTask{}
	for _, id := range ids {
		t := NoteTask{ID: id, Title: planned[id].Title, Branch: planned[id].Branch}
		if state != nil {
			if ts, ok := state.TaskStates[id]; ok {
				if ts.Title != "" {
					t.Title = ts.Title
				}
				if ts.Branch != "" {
					t.Branch = ts.Branch
				}
			}
		}
		tasks = append(tasks, t)
	}
	sort.SliceStable(tasks, func(i, j int) bool { return taskOrder(tasks[i].ID) < taskOrder(tasks[j].ID) })
	return tasks
}

var taskIDNumber = regexp.MustCompile(`^T(\d+)$`)

func taskOrder(id string) int {
	if m := taskIDNumber.FindStringSubmatch(id); m != nil {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		return n
	}
	return 1 << 30
}

// TasksSince lists the tasks merged into main after the release before
// commit and up to it, by their Ringmaster-Task trailers: what a release at
// commit holds, whether commit is tagged already or not.
func TasksSince(ctx context.Context, projectRoot, commit string) []string {
	if strings.HasPrefix(commit, "-") {
		return []string{}
	}
	rangeArg := commit
	if out, err := workspace.RunGit(ctx, projectRoot, "describe", "--tags", "--abbrev=0", commit+"^"); err == nil {
		if tag := strings.TrimSpace(out); tag != "" {
			rangeArg = tag + ".." + commit
		}
	}
	out, err := workspace.RunGit(ctx, projectRoot, "log", "--first-parent", "--reverse",
		"--format=%(trailers:key=Ringmaster-Task,valueonly,separator=%x2C)", rangeArg)
	if err != nil {
		return []string{}
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, line := range strings.Split(out, "\n") {
		for _, id := range strings.Split(line, ",") {
			if id = strings.TrimSpace(id); id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// Record is what Ringmaster keeps of a release it made: its notes, and the
// page it has on the git host, if one was created.
type Record struct {
	Version   string    `json:"version"`
	Title     string    `json:"title"`
	Notes     string    `json:"notes"`
	PageURL   string    `json:"page_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

var unsafeFile = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func recordPath(projectRoot, version string) string {
	return filepath.Join(projectRoot, ".ringmaster", "releases", unsafeFile.ReplaceAllString(version, "_")+".json")
}

// SaveRecord keeps r in .ringmaster/releases, which is never committed.
func SaveRecord(projectRoot string, r Record) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	path := recordPath(projectRoot, r.Version)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return storage.WriteJSONAtomic(path, r)
}

// LoadRecord returns the record of version, or nil when Ringmaster has none
// (a tag made by hand, or before release notes were kept).
func LoadRecord(projectRoot, version string) *Record {
	r, err := storage.ReadJSON[Record](recordPath(projectRoot, version))
	if err != nil || r == nil || r.Version != version {
		return nil
	}
	return r
}

// Records returns every release record, keyed by version.
func Records(projectRoot string) map[string]Record {
	out := map[string]Record{}
	entries, err := os.ReadDir(filepath.Join(projectRoot, ".ringmaster", "releases"))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := storage.ReadJSON[Record](filepath.Join(projectRoot, ".ringmaster", "releases", e.Name()))
		if err == nil && r != nil && r.Version != "" {
			out[r.Version] = *r
		}
	}
	return out
}

// TagNotes reads the notes back from an existing annotated tag, without its
// subject line: the notes of a release made before records were kept.
func TagNotes(ctx context.Context, projectRoot, version string) (string, error) {
	if strings.HasPrefix(version, "-") {
		return "", errors.New("a version cannot start with '-'")
	}
	out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref", "--format=%(contents:body)", "refs/tags/"+version)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

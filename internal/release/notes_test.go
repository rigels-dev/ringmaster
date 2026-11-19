package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/llm"
)

func TestDraftNotesGroupsTasksByKind(t *testing.T) {
	got := DraftNotes([]NoteTask{
		{ID: "T01", Title: "Link store", Branch: "feat/link-store"},
		{ID: "T02", Title: "README", Branch: "docs/readme-usage"},
		{ID: "T03", Title: "Empty URL crash", Branch: "fix/empty-url"},
		{ID: "T04", Title: "Old task", Branch: "rm/T04"},
		{ID: "T05", Title: "HTTP API", Branch: "feat/http-api"},
	})
	want := SummaryPlaceholder + `

### Features

- Link store (T01)
- HTTP API (T05)

### Fixes

- Empty URL crash (T03)

### Documentation

- README (T02)

### Other changes

- Old task (T04)
`
	if got != want {
		t.Fatalf("draft:\n%s\nwant:\n%s", got, want)
	}
}

func TestInsertSummary(t *testing.T) {
	draft := DraftNotes([]NoteTask{{ID: "T01", Title: "A", Branch: "feat/a"}})
	got := InsertSummary(draft, "  shorty shortens links.  ")
	if strings.Contains(got, SummaryPlaceholder) || !strings.HasPrefix(got, "shorty shortens links.\n\n### Features") {
		t.Fatalf("the summary goes where the placeholder was:\n%s", got)
	}

	// The user removed the placeholder: the summary goes on top, and what they wrote stays
	edited := "### Features\n\n- A (T01)\n"
	if got := InsertSummary(edited, "Summary."); got != "Summary.\n\n"+edited {
		t.Fatalf("without the placeholder:\n%q", got)
	}
	if got := InsertSummary(edited, "   "); got != edited {
		t.Fatalf("an empty summary changes nothing: %q", got)
	}
}

func TestCleanNotesAndTagMessage(t *testing.T) {
	draft := DraftNotes([]NoteTask{{ID: "T01", Title: "A", Branch: "feat/a"}})
	if got := CleanNotes(draft); got != "### Features\n\n- A (T01)" {
		t.Fatalf("clean: %q", got)
	}
	if got := TagMessage("v0.1.0", draft); got != "Release v0.1.0\n\n### Features\n\n- A (T01)\n" {
		t.Fatalf("tag message: %q", got)
	}
	if got := TagMessage("v0.1.0", SummaryPlaceholder); got != "Release v0.1.0\n" {
		t.Fatalf("notes left empty: %q", got)
	}
	if err := CheckNotes(strings.Repeat("x", MaxNotesBytes+1)); err == nil {
		t.Fatal("notes that are too long are accepted")
	}
}

func TestTagCarriesTheNotesVerbatim(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	notes := "shorty shortens links.\n\n### Features\n\n- Link store (T01)\n"
	if err := Tag(ctx, dir, "v0.1.0", notes); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "for-each-ref", "--format=%(contents:subject)", "refs/tags/v0.1.0"); got != "Release v0.1.0" {
		t.Fatalf("subject: %q", got)
	}
	got, err := TagNotes(ctx, dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	// "### Features" would be dropped as a comment without --cleanup=verbatim
	if got != strings.TrimSpace(notes) {
		t.Fatalf("notes in the tag:\n%q\nwant:\n%q", got, notes)
	}
}

func writeState(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, ".ringmaster", name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestTasksOfPrefersTheRecordedTitleAndBranch(t *testing.T) {
	dir := t.TempDir()
	writeState(t, dir, "plan.json", `{"project":"p","tasks":[
	 {"id":"T02","title":"Planned title","type":"docs","branch":"docs/planned","depends_on":[],"worker":"w","max_retries":2,"acceptance":["true"]},
	 {"id":"T10","title":"Ten","type":"backend","branch":"feat/ten","depends_on":[],"worker":"w","max_retries":2,"acceptance":["true"]}]}`)
	writeState(t, dir, "state.json", `{"task_states":{
	 "T02":{"status":"COMPLETED","title":"Recorded title","branch":"docs/recorded"},
	 "T01":{"status":"COMPLETED","title":"From an earlier plan","branch":"feat/earlier"}}}`)

	got := TasksOf(dir, []string{"T10", "T02", "T01"})
	want := []NoteTask{
		{ID: "T01", Title: "From an earlier plan", Branch: "feat/earlier"},
		{ID: "T02", Title: "Recorded title", Branch: "docs/recorded"},
		{ID: "T10", Title: "Ten", Branch: "feat/ten"},
	}
	if len(got) != len(want) {
		t.Fatalf("tasks: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("task %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

func mergeTask(t *testing.T, dir, id string) {
	t.Helper()
	git(t, dir, "checkout", "-q", "-b", "task-"+id)
	commit(t, dir, id+".txt", id)
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--no-ff", "task-"+id, "-m", "Merge task "+id+"\n\nRingmaster-Task: "+id)
}

func TestTasksSinceTheLastRelease(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	mergeTask(t, dir, "T01")
	mergeTask(t, dir, "T02")
	if got := strings.Join(TasksSince(ctx, dir, "main"), ","); got != "T01,T02" {
		t.Fatalf("before any release: %s", got)
	}
	if err := Tag(ctx, dir, "v0.1.0", ""); err != nil {
		t.Fatal(err)
	}
	// The tagged release itself still holds its tasks
	if got := strings.Join(TasksSince(ctx, dir, "v0.1.0"), ","); got != "T01,T02" {
		t.Fatalf("v0.1.0 holds: %s", got)
	}
	mergeTask(t, dir, "T03")
	if got := strings.Join(TasksSince(ctx, dir, "main"), ","); got != "T03" {
		t.Fatalf("after v0.1.0: %s", got)
	}
}

func TestRecords(t *testing.T) {
	dir := t.TempDir()
	if LoadRecord(dir, "v0.1.0") != nil || len(Records(dir)) != 0 {
		t.Fatal("a project without releases has records")
	}
	if err := SaveRecord(dir, Record{Version: "v0.1.0/x", Title: "v0.1.0", Notes: "n", PageURL: "https://github.com/o/r/releases/tag/v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	r := LoadRecord(dir, "v0.1.0/x")
	if r == nil || r.PageURL == "" || r.CreatedAt.IsZero() {
		t.Fatalf("record: %+v", r)
	}
	if _, ok := Records(dir)["v0.1.0/x"]; !ok {
		t.Fatalf("records: %+v", Records(dir))
	}
}

func TestGitHubRepo(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/rigels-dev/shorty.git":       "rigels-dev/shorty",
		"https://github.com/rigels-dev/shorty":           "rigels-dev/shorty",
		"https://token@github.com/rigels-dev/shorty.git": "rigels-dev/shorty",
		"git@github.com:rigels-dev/shorty.git":           "rigels-dev/shorty",
		"ssh://git@github.com/rigels-dev/my.repo.git":    "rigels-dev/my.repo",
		"https://gitlab.com/rigels-dev/shorty.git":       "",
		"https://github.com/rigels-dev":                  "",
		"/srv/git/shorty.git":                            "",
	} {
		if got := GitHubRepo(url); got != want {
			t.Errorf("%s: %q, want %q", url, got, want)
		}
	}
}

func fakeGH(t *testing.T, fn func(args []string) (string, error)) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old, oldHas := runGH, hasGH
	runGH = func(ctx context.Context, dir string, args ...string) (string, error) {
		*calls = append(*calls, args)
		return fn(args)
	}
	hasGH = func() bool { return true }
	t.Cleanup(func() { runGH, hasGH = old, oldHas })
	return calls
}

func TestCreatePage(t *testing.T) {
	ctx := context.Background()
	var notesSeen string
	calls := fakeGH(t, func(args []string) (string, error) {
		for i, a := range args {
			if a == "--notes-file" {
				data, _ := os.ReadFile(args[i+1])
				notesSeen = string(data)
			}
		}
		return "https://github.com/rigels-dev/shorty/releases/tag/v0.1.0", nil
	})

	url, err := CreatePage(ctx, t.TempDir(), "git@github.com:rigels-dev/shorty.git", "v0.1.0", "v0.1.0 · First version",
		InsertSummary(DraftNotes([]NoteTask{{ID: "T01", Title: "A", Branch: "feat/a"}}), ""))
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/rigels-dev/shorty/releases/tag/v0.1.0" {
		t.Fatalf("url: %s", url)
	}
	args := strings.Join((*calls)[0], " ")
	for _, want := range []string{"release create v0.1.0", "--repo rigels-dev/shorty", "--title v0.1.0 · First version", "--verify-tag"} {
		if !strings.Contains(args, want) {
			t.Errorf("gh %s: missing %q", args, want)
		}
	}
	if notesSeen != "### Features\n\n- A (T01)\n" {
		t.Fatalf("the page's notes keep the placeholder: %q", notesSeen)
	}

	if _, err := CreatePage(ctx, t.TempDir(), "https://gitlab.com/o/r.git", "v0.1.0", "", ""); err == nil {
		t.Fatal("a page is made for a remote that is not GitHub")
	}
}

func TestCreatePageExplainsGHFailures(t *testing.T) {
	ctx := context.Background()
	for reason, want := range map[string]string{
		"a release with the same tag name already exists: v0.1.0":          "already has a release page",
		"To get started with GitHub CLI, please run:  gh auth login":       "not logged in",
		"tag v0.1.0 doesn't exist in the repo rigels-dev/shorty, aborting": "gh could not create the release page: tag v0.1.0",
	} {
		fakeGH(t, func(args []string) (string, error) { return "", errors.New(reason) })
		_, err := CreatePage(ctx, t.TempDir(), "https://github.com/o/r", "v0.1.0", "", "")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", reason, err, want)
		}
	}
}

type fakeProvider struct {
	reply string
	got   []llm.Message
}

func (f *fakeProvider) Chat(ctx context.Context, model string, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.got = req.Messages
	return &llm.ChatResponse{Content: f.reply, Usage: llm.TokenUsage{PromptTokens: 100, CompletionTokens: 20}}, nil
}

func (f *fakeProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) { return nil, nil }

func TestSummarizeUsesTheTasksAndCleansTheReply(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	writeState(t, dir, "tasks/T01-link-store.md", "---\nid: T01\ntitle: Link store\ntype: backend\nacceptance: [\"true\"]\n---\nAn in-memory store of short links.\n\n## Human Answers\nsecret answer\n")
	mergeTask(t, dir, "T01")

	p := &fakeProvider{reply: "```\nSummary: \"shorty shortens long URLs.\"\n```"}
	summary, usage, err := Summarize(ctx, dir, p, "m", SummaryInput{
		Project: "shorty", Version: "v0.1.0", Title: "First version",
		Tasks: []NoteTask{{ID: "T01", Title: "Link store"}}, Commit: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary != "shorty shortens long URLs." {
		t.Fatalf("summary: %q", summary)
	}
	if usage.Calls != 1 || usage.TokensIn != 100 || usage.TokensOut != 20 {
		t.Fatalf("usage: %+v", usage)
	}
	prompt := p.got[1].Content
	for _, want := range []string{"v0.1.0 · First version", "T01: Link store", "An in-memory store of short links.", "T01.txt"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "secret answer") {
		t.Error("the answers given to the automaton are sent to the model")
	}

	if _, _, err := Summarize(ctx, dir, p, "m", SummaryInput{}); err == nil {
		t.Error("a release without tasks is summarized")
	}
	p.reply = "  "
	if _, _, err := Summarize(ctx, dir, p, "m", SummaryInput{Tasks: []NoteTask{{ID: "T01"}}}); err == nil {
		t.Error("an empty reply is accepted")
	}
}

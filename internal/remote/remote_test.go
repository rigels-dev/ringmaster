package remote

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, dir, file, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(msg+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", msg)
}

// project makes a repository with main and two task branches, one merged.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, "a.txt", "first")
	git(t, dir, "branch", "rm/T01")
	git(t, dir, "checkout", "-q", "-b", "rm/T02")
	commit(t, dir, "b.txt", "task two, not merged")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "branch", "scratch") // not Ringmaster's; must never be pushed
	return dir
}

func bareRemote(t *testing.T) (path, fileURL string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "remote.git")
	git(t, filepath.Dir(path), "init", "-q", "--bare", "-b", "main", path)
	return path, "file://" + path
}

func states(st Status) map[string]string {
	m := map[string]string{}
	for _, b := range st.Branches {
		m[b.Name] = b.State
	}
	return m
}

func results(rs []PushResult) map[string]string {
	m := map[string]string{}
	for _, r := range rs {
		m[r.Branch] = r.Result
	}
	return m
}

func TestValidateURL(t *testing.T) {
	good := []string{
		"https://github.com/owner/repo.git",
		"https://github.com/owner/repo",
		"git@github.com:owner/repo.git",
		"ssh://git@github.com/owner/repo.git",
		"file:///srv/git/repo.git",
	}
	for _, u := range good {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}

	bad := []string{
		"",
		"--upload-pack=touch /tmp/x",
		"-oProxyCommand=x",
		"ext::sh -c 'touch /tmp/x'",
		"ext::sh",
		"https://github.com/owner/repo with space",
		"https://ghp_secret@github.com/owner/repo.git",
		"https://user:pass@github.com/owner/repo.git",
		"https:///owner/repo",
		"ftp://example.com/repo.git",
		"git://example.com/repo.git",
		"owner/repo",
		"/srv/git/repo.git",
		"git@github.com:owner/repo.git\n--force",
	}
	for _, u := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) = nil, want an error", u)
		}
	}
}

func TestSetURLAddsThenReplaces(t *testing.T) {
	ctx := context.Background()
	dir := project(t)

	if got := URL(ctx, dir); got != "" {
		t.Fatalf("a fresh project has origin %q, want none", got)
	}
	if err := SetURL(ctx, dir, "https://github.com/owner/one.git"); err != nil {
		t.Fatal(err)
	}
	if got := URL(ctx, dir); got != "https://github.com/owner/one.git" {
		t.Fatalf("origin = %q after add", got)
	}
	if err := SetURL(ctx, dir, "git@github.com:owner/two.git"); err != nil {
		t.Fatal(err)
	}
	if got := URL(ctx, dir); got != "git@github.com:owner/two.git" {
		t.Fatalf("origin = %q after replace", got)
	}

	if err := SetURL(ctx, dir, "--upload-pack=x"); err == nil {
		t.Fatal("an option-like URL was accepted")
	}
	if got := URL(ctx, dir); got != "git@github.com:owner/two.git" {
		t.Fatalf("a refused URL changed origin to %q", got)
	}
}

func TestPushSendsMainAndTaskBranchesOnly(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	remotePath, remoteURL := bareRemote(t)

	if _, err := Push(ctx, dir, false); err == nil {
		t.Fatal("push without a remote should fail")
	}

	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}
	st, err := GetStatus(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.URL != remoteURL {
		t.Fatalf("status URL = %q", st.URL)
	}
	if len(st.Branches) != 3 || st.Branches[0].Name != "main" {
		t.Fatalf("status branches = %+v, want main first then the two task branches", st.Branches)
	}
	// rm/T01 is merged: its work goes with main, so it is not waiting on a push
	if st.Pending() != 2 || st.Merged() != 1 || !st.Branches[1].Merged || st.Branches[2].Merged {
		t.Fatalf("pending = %d, merged = %d before the first push, want 2 and 1 (%+v)", st.Pending(), st.Merged(), st.Branches)
	}

	rs, err := Push(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	got := results(rs)
	for _, b := range []string{"main", "rm/T01", "rm/T02"} {
		if got[b] != ResultNew {
			t.Errorf("first push of %s = %q, want %q", b, got[b], ResultNew)
		}
	}

	onRemote := git(t, remotePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if onRemote != "main\nrm/T01\nrm/T02" {
		t.Fatalf("remote branches = %q; a branch that is not Ringmaster's must not be pushed", onRemote)
	}
	if git(t, remotePath, "rev-parse", "rm/T02") != git(t, dir, "rev-parse", "rm/T02") {
		t.Fatal("the unmerged task branch on the remote differs from the local one")
	}

	st, _ = GetStatus(ctx, dir)
	if st.Pending() != 0 {
		t.Fatalf("pending = %d after pushing, want 0 (%+v)", st.Pending(), st.Branches)
	}

	// New work on main: only main is ahead, and only main is sent
	commit(t, dir, "c.txt", "more on main")
	st, _ = GetStatus(ctx, dir)
	if s := states(st); s["main"] != StateAhead || s["rm/T01"] != StateSynced {
		t.Fatalf("states after a new commit = %v", s)
	}
	rs, err = Push(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	got = results(rs)
	if got["main"] != ResultUpdated || got["rm/T01"] != ResultSame {
		t.Fatalf("second push = %v, want main updated and rm/T01 up-to-date", got)
	}
}

func TestPushMainOnly(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	remotePath, remoteURL := bareRemote(t)
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}

	if _, err := Push(ctx, dir, true); err != nil {
		t.Fatal(err)
	}
	if onRemote := git(t, remotePath, "for-each-ref", "--format=%(refname:short)", "refs/heads"); onRemote != "main" {
		t.Fatalf("remote branches = %q, want only main", onRemote)
	}
	st, _ := GetStatus(ctx, dir)
	if s := states(st); s["main"] != StateSynced || s["rm/T01"] != StateUnpushed {
		t.Fatalf("states = %v", s)
	}
}

func TestPushWithoutTaskBranches(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, "a.txt", "first")
	_, remoteURL := bareRemote(t)
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}

	rs, err := Push(ctx, dir, false)
	if err != nil {
		t.Fatalf("a project with no task branches yet should still push main: %v", err)
	}
	if got := results(rs); len(got) != 1 || got["main"] != ResultNew {
		t.Fatalf("results = %v", got)
	}
}

// The remote moved on (someone else pushed to main). Push must not force:
// main is rejected and reported, and the remote keeps its commit.
func TestPushNeverOverwritesTheRemote(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	remotePath, remoteURL := bareRemote(t)
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(ctx, dir, false); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", remoteURL, other)
	commit(t, other, "theirs.txt", "someone else's commit")
	git(t, other, "push", "-q", "origin", "main")
	theirs := git(t, remotePath, "rev-parse", "main")

	commit(t, dir, "ours.txt", "our commit")
	rs, err := Push(ctx, dir, false)
	if err == nil {
		t.Fatal("a non-fast-forward push should fail")
	}
	if !strings.Contains(err.Error(), "rejected main") {
		t.Fatalf("error does not name the rejected branch: %v", err)
	}
	if got := results(rs); got["main"] != ResultRejected {
		t.Fatalf("results = %v, want main rejected", got)
	}
	if now := git(t, remotePath, "rev-parse", "main"); now != theirs {
		t.Fatal("the remote's main was overwritten")
	}
}

func TestPushReportsAnUnreachableRemote(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	if err := SetURL(ctx, dir, "file://"+filepath.Join(t.TempDir(), "missing.git")); err != nil {
		t.Fatal(err)
	}
	_, err := Push(ctx, dir, false)
	if err == nil {
		t.Fatal("pushing to a remote that does not exist should fail")
	}
	if !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("error does not carry git's reason: %v", err)
	}
}

// Task branches with readable names are pushed because Ringmaster recorded
// them for its tasks; a branch of the user's own with a similar name is not.
func TestPushSendsRecordedTaskBranchesNotTheUsers(t *testing.T) {
	ctx := context.Background()
	dir := project(t) // main, rm/T01, rm/T02, and the user's "scratch"
	remotePath, remoteURL := bareRemote(t)
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}

	git(t, dir, "branch", "feat/link-store")  // a task's
	git(t, dir, "branch", "docs/usage")       // a task's
	git(t, dir, "branch", "feat/my-own-idea") // the user's
	if err := os.MkdirAll(filepath.Join(dir, ".ringmaster"), 0755); err != nil {
		t.Fatal(err)
	}
	state := `{"task_states":{
	 "T03":{"id":"T03","status":"COMPLETED","branch":"feat/link-store"},
	 "T04":{"id":"T04","status":"RUNNING","branch":"docs/usage"},
	 "T05":{"id":"T05","status":"PENDING","branch":"feat/not-created-yet"},
	 "T06":{"id":"T06","status":"PENDING","branch":"--force"}}}`
	if err := os.WriteFile(filepath.Join(dir, ".ringmaster", "state.json"), []byte(state), 0644); err != nil {
		t.Fatal(err)
	}

	if got, want := strings.Join(TaskBranches(ctx, dir), " "), "docs/usage feat/link-store rm/T01 rm/T02"; got != want {
		t.Fatalf("task branches = %q, want %q", got, want)
	}

	st, err := GetStatus(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, b := range st.Branches {
		names = append(names, b.Name)
	}
	if got, want := strings.Join(names, " "), "main docs/usage feat/link-store rm/T01 rm/T02"; got != want {
		t.Fatalf("status lists %q, want %q", got, want)
	}

	if _, err := Push(ctx, dir, false); err != nil {
		t.Fatal(err)
	}
	onRemote := strings.Join(strings.Fields(git(t, remotePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")), " ")
	if want := "docs/usage feat/link-store main rm/T01 rm/T02"; onRemote != want {
		t.Fatalf("remote branches = %q, want %q", onRemote, want)
	}
}

func pruned(rs []PruneResult) map[string][]string {
	m := map[string][]string{}
	for _, r := range rs {
		m[r.Branch] = append(m[r.Branch], r.Result)
	}
	return m
}

// Only merged task branches go; main, an unmerged task branch, a branch
// checked out in a worktree and the user's own branch stay.
func TestPruneDeletesMergedTaskBranchesOnly(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	git(t, dir, "branch", "rm/T03")
	git(t, dir, "worktree", "add", "-q", filepath.Join(t.TempDir(), "T03"), "rm/T03")

	st, _ := GetStatus(ctx, dir)
	for _, b := range st.Branches {
		if b.Name == "rm/T03" && b.Merged {
			t.Error("a branch checked out in a worktree counts as merged")
		}
	}

	rs, err := Prune(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := pruned(rs); len(got) != 1 || got["rm/T01"][0] != PruneDeleted {
		t.Fatalf("prune = %+v, want only rm/T01 deleted", rs)
	}
	left := git(t, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if left != "main\nrm/T02\nrm/T03\nscratch" {
		t.Fatalf("branches left = %q", left)
	}

	// Nothing left to prune
	if rs, err := Prune(ctx, dir, false); err != nil || len(rs) != 0 {
		t.Fatalf("second prune = %+v, %v", rs, err)
	}
}

// With onRemote, merged task branches are deleted on the remote too,
// including ones already gone here, but not one the remote has moved on.
func TestPruneOnRemote(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	git(t, dir, "branch", "rm/T04")
	remotePath, remoteURL := bareRemote(t)

	if _, err := Prune(ctx, dir, true); err == nil {
		t.Fatal("pruning on a remote that is not set should fail")
	}
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(ctx, dir, false); err != nil {
		t.Fatal(err)
	}

	// rm/T04 was deleted here earlier, and is still on the remote
	git(t, dir, "branch", "-d", "rm/T04")

	// Someone moves rm/T01 on the remote after this project last saw it
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", remoteURL, other)
	git(t, other, "checkout", "-q", "rm/T01")
	commit(t, other, "d.txt", "someone else's work")
	git(t, other, "push", "-q", "origin", "rm/T01")

	rs, err := Prune(ctx, dir, true)
	got := pruned(rs)
	if err == nil {
		t.Error("a refused remote deletion should be reported as an error")
	}
	if strings.Join(got["rm/T01"], ",") != PruneDeleted+","+PruneKept {
		t.Errorf("rm/T01 = %v, want deleted here and kept on the remote (%+v)", got["rm/T01"], rs)
	}

	onRemote := git(t, remotePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if strings.Contains(onRemote, "rm/T04") {
		t.Errorf("rm/T04 was not deleted on the remote: %q", onRemote)
	}
	for _, keep := range []string{"main", "rm/T01", "rm/T02"} {
		if !strings.Contains(onRemote, keep) {
			t.Errorf("%s was deleted on the remote: %q", keep, onRemote)
		}
	}
	if strings.Join(got["rm/T04"], ",") != PruneRemoteDeleted {
		t.Errorf("rm/T04 = %v (%+v)", got["rm/T04"], rs)
	}
}

// A release tagged without pushing goes with the next push, and the status
// says whether it is on the remote. Tags that are not a step's release are
// never pushed.
func TestPushSendsReleaseTags(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	remotePath, remoteURL := bareRemote(t)
	if err := SetURL(ctx, dir, remoteURL); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(ctx, dir, true); err != nil {
		t.Fatal(err)
	}

	// Step 1 ends at main and is released as v0.1.0, locally only
	mainSha := git(t, dir, "rev-parse", "main")
	steps := `{"steps":[{"number":1,"plan_hash":"x","tasks":["T01"],"commit":"` + mainSha + `","finished_at":"2026-11-19T08:08:00Z"}]}`
	if err := os.MkdirAll(filepath.Join(dir, ".ringmaster"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ringmaster", "steps.json"), []byte(steps), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "tag", "-a", "-m", "release", "v0.1.0")
	git(t, dir, "tag", "mine", "rm/T02") // the user's own tag, on another commit

	st, _ := GetStatus(ctx, dir)
	if len(st.Tags) != 1 || st.Tags[0].Name != "v0.1.0" || st.Tags[0].State != StateUnpushed {
		t.Fatalf("tags = %+v, want v0.1.0 not pushed", st.Tags)
	}
	if st.Pending() == 0 {
		t.Fatal("an unpushed release tag does not count as waiting on a push")
	}

	rs, err := Push(ctx, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if results(rs)["tag v0.1.0"] != ResultNew {
		t.Fatalf("push = %+v, want the tag sent", rs)
	}
	onRemote := git(t, remotePath, "tag")
	if onRemote != "v0.1.0" {
		t.Fatalf("remote tags = %q, want only the release tag", onRemote)
	}

	st, _ = GetStatus(ctx, dir)
	if st.Tags[0].State != StateSynced || st.Pending() != 1 {
		// rm/T02 is unmerged and was not pushed with --main-only
		t.Fatalf("after the push: tags = %+v, pending = %d", st.Tags, st.Pending())
	}

	// Another remote has not had it
	other, otherURL := bareRemote(t)
	_ = other
	if err := SetURL(ctx, dir, otherURL); err != nil {
		t.Fatal(err)
	}
	if st, _ = GetStatus(ctx, dir); st.Tags[0].State != StateUnpushed {
		t.Fatalf("after changing the remote: tags = %+v", st.Tags)
	}
}

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
	if st.Pending() != 3 {
		t.Fatalf("pending = %d before the first push, want 3", st.Pending())
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

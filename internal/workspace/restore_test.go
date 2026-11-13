package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := RunGit(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func put(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		return "<missing>"
	}
	return string(data)
}

func isTest(file string) bool { return strings.HasSuffix(file, "_test.go") }

// Whatever a task did to a protected file that existed at its base — edited
// in the working tree, staged, committed in an earlier attempt, or deleted —
// is undone. Its own new files and its unprotected changes stay.
func TestRestoreFromBase(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-b", "main")
	for _, name := range []string{"a_test.go", "b_test.go", "c_test.go", "d_test.go", "sub dir/é_test.go", "code.go"} {
		put(t, repo, name, "original "+name)
	}
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-m", "base")

	worktree, err := CreateWorktree(ctx, repo, "T01", "main")
	if err != nil {
		t.Fatal(err)
	}

	// main moves on after the task branched; the task's base does not
	put(t, repo, "later_test.go", "added on main later")
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-m", "later")

	put(t, worktree, "a_test.go", "weakened and committed")
	put(t, worktree, "own_test.go", "the task's own test, committed")
	put(t, worktree, "code.go", "implementation, committed")
	mustGit(t, worktree, "add", "-A")
	mustGit(t, worktree, "commit", "-m", "attempt 1")

	put(t, worktree, "b_test.go", "weakened, unstaged")
	put(t, worktree, "c_test.go", "weakened, staged")
	mustGit(t, worktree, "add", "c_test.go")
	if err := os.Remove(filepath.Join(worktree, "d_test.go")); err != nil {
		t.Fatal(err)
	}
	put(t, worktree, "sub dir/é_test.go", "weakened, odd name")
	put(t, worktree, "new_test.go", "the task's own test, untracked")
	put(t, worktree, "own_test.go", "the task's own test, edited again")

	restored, err := RestoreFromBase(ctx, worktree, "main", isTest)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(restored, "|"), "a_test.go|b_test.go|c_test.go|d_test.go|sub dir/é_test.go"; got != want {
		t.Errorf("restored %s, want %s", got, want)
	}
	for _, name := range []string{"a_test.go", "b_test.go", "c_test.go", "d_test.go", "sub dir/é_test.go"} {
		if got := read(t, worktree, name); got != "original "+name {
			t.Errorf("%s not restored: %q", name, got)
		}
	}
	for name, want := range map[string]string{
		"code.go":       "implementation, committed",
		"new_test.go":   "the task's own test, untracked",
		"own_test.go":   "the task's own test, edited again",
		"later_test.go": "<missing>",
	} {
		if got := read(t, worktree, name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}

	// Nothing left to restore the second time
	if again, err := RestoreFromBase(ctx, worktree, "main", isTest); err != nil || len(again) != 0 {
		t.Errorf("second run restored %v (err %v)", again, err)
	}

	// The commit that follows records the tests as they were at the base
	if _, err := CommitAll(ctx, worktree, "attempt 2", nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(mustGit(t, worktree, "show", "HEAD:a_test.go")); got != "original a_test.go" {
		t.Errorf("committed a_test.go: %q", got)
	}
}

// A task that builds on another task's branch treats that branch as its base:
// tests the upstream task added are protected too.
func TestRestoreFromBaseUsesUpstreamBranch(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-b", "main")
	put(t, repo, "code.go", "v0")
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-m", "base")

	upstream, err := CreateWorktree(ctx, repo, "T01", "main")
	if err != nil {
		t.Fatal(err)
	}
	put(t, upstream, "up_test.go", "written by T01")
	mustGit(t, upstream, "add", "-A")
	mustGit(t, upstream, "commit", "-m", "T01")

	downstream, err := CreateWorktree(ctx, repo, "T02", "rm/T01")
	if err != nil {
		t.Fatal(err)
	}
	put(t, downstream, "up_test.go", "weakened by T02")

	restored, err := RestoreFromBase(ctx, downstream, "rm/T01", isTest)
	if err != nil || len(restored) != 1 || read(t, downstream, "up_test.go") != "written by T01" {
		t.Errorf("restored %v (err %v), content %q", restored, err, read(t, downstream, "up_test.go"))
	}

	if _, err := RestoreFromBase(ctx, downstream, "no-such-ref", isTest); err == nil {
		t.Error("an unknown base should be an error, not silently no protection")
	}
}

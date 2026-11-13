package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func excludeFile(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func untracked(t *testing.T, repo string) string {
	t.Helper()
	out, err := RunGit(context.Background(), repo, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStateAndManifestAreExcludedFromGit(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	ctx := context.Background()
	touch(t, filepath.Join(repo, "ringmaster.toml"))
	touch(t, filepath.Join(repo, ".ringmaster", "state.json"))
	touch(t, filepath.Join(repo, "main.go"))
	// Same name, deeper in the project: this one is the user's own file
	touch(t, filepath.Join(repo, "examples", "ringmaster.toml"))

	if err := EnsureStateExcluded(ctx, repo); err != nil {
		t.Fatal(err)
	}
	status := untracked(t, repo)
	if strings.Contains(status, ".ringmaster/") || strings.Contains(status, "?? ringmaster.toml") {
		t.Errorf("Ringmaster's files still show up in git status:\n%s", status)
	}
	for _, want := range []string{"main.go", "examples/ringmaster.toml"} {
		if !strings.Contains(status, want) {
			t.Errorf("%s is hidden, but only the root manifest and .ringmaster/ should be:\n%s", want, status)
		}
	}

	// Calling it again changes nothing
	before := excludeFile(t, repo)
	if err := EnsureStateExcluded(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if after := excludeFile(t, repo); after != before {
		t.Errorf("exclude file changed on the second call:\n%s", after)
	}
}

// A project set up by an older version has only .ringmaster/ excluded. The
// manifest is added next to it, and what was there is kept.
func TestManifestIsAddedToAnOlderExcludeFile(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	path := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("*.swp\n# Ringmaster state\n.ringmaster/"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureStateExcluded(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	got := excludeFile(t, repo)
	if !strings.Contains(got, "*.swp\n") || strings.Count(got, ".ringmaster/") != 1 {
		t.Errorf("existing entries were lost or duplicated:\n%s", got)
	}
	if !strings.Contains(got, "\n/ringmaster.toml\n") {
		t.Errorf("the manifest was not added on its own line:\n%s", got)
	}
}

// A user who commits the manifest on purpose keeps it tracked.
func TestCommittedManifestStaysTracked(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	ctx := context.Background()
	if err := EnsureStateExcluded(ctx, repo); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(repo, "ringmaster.toml"))
	if _, err := RunGit(ctx, repo, "add", "-f", "ringmaster.toml"); err != nil {
		t.Fatal(err)
	}
	if _, err := RunGit(ctx, repo, "commit", "-q", "-m", "share the manifest"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo, "ringmaster.toml"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if status := untracked(t, repo); !strings.Contains(status, "M ringmaster.toml") {
		t.Errorf("a change to the committed manifest is not seen by git:\n%s", status)
	}
}

package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"ringmaster/internal/workspace"
)

func TestInitProject(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	projectName := "demo_test"

	// 1. Initial init should succeed
	if err := RunInit(projectName, false); err != nil {
		t.Fatalf("first RunInit failed: %v", err)
	}

	// Verify directories and files
	expectedBrief := filepath.Join(projectName, ".ringmaster", "BRIEF.md")
	if _, err := os.Stat(expectedBrief); os.IsNotExist(err) {
		t.Errorf("expected BRIEF.md at %s", expectedBrief)
	}

	expectedGit := filepath.Join(projectName, ".git")
	if _, err := os.Stat(expectedGit); os.IsNotExist(err) {
		t.Errorf("expected .git directory at %s", expectedGit)
	}
	if _, err := os.Stat(filepath.Join(projectName, "repo")); err == nil {
		t.Errorf("init must not create a nested repo/ directory")
	}

	// 2. Second init without force should fail
	if err := RunInit(projectName, false); err == nil {
		t.Fatalf("expected error on duplicate init, got nil")
	}

	// 3. Init with force should succeed
	if err := RunInit(projectName, true); err != nil {
		t.Fatalf("expected force init to succeed, got: %v", err)
	}
}

// Initializing inside an existing repository must only add ringmaster.toml and
// .ringmaster/, keep .ringmaster/ out of git, and never commit the user's files.
func TestInitExistingRepo(t *testing.T) {
	repoDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	ctx := context.Background()
	if _, err := workspace.RunGit(ctx, repoDir, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0644)
	if _, err := workspace.RunGit(ctx, repoDir, "add", "main.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.RunGit(ctx, repoDir, "commit", "-m", "user commit"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repoDir, "wip.txt"), []byte("uncommitted\n"), 0644)

	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	if err := RunInit("", false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	entries, _ := os.ReadDir(repoDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	want := []string{".git", ".ringmaster", "main.go", "ringmaster.toml", "wip.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("unexpected project contents: got %v, want %v", names, want)
	}

	log, _ := workspace.RunGit(ctx, repoDir, "log", "--oneline")
	if n := len(strings.Split(strings.TrimSpace(log), "\n")); n != 1 {
		t.Errorf("init must not add commits to an existing repo, log:\n%s", log)
	}

	status, _ := workspace.RunGit(ctx, repoDir, "status", "--porcelain")
	if strings.Contains(status, ".ringmaster") {
		t.Errorf(".ringmaster/ should be git-excluded, status:\n%s", status)
	}
	if strings.Contains(status, "ringmaster.toml") {
		t.Errorf("ringmaster.toml should be git-excluded, status:\n%s", status)
	}
	if ignore, err := os.ReadFile(filepath.Join(repoDir, ".gitignore")); err == nil {
		t.Errorf("init must not create or edit the project's .gitignore:\n%s", ignore)
	}
}

// A new project created inside another checkout (e.g. examples/ of the
// Ringmaster repo) must get its own repository, never reuse the outer one.
func TestInitNestedNewProjectGetsOwnRepo(t *testing.T) {
	outer := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	ctx := context.Background()
	if _, err := workspace.RunGit(ctx, outer, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	examples := filepath.Join(outer, "examples")
	_ = os.MkdirAll(examples, 0755)
	if err := os.Chdir(examples); err != nil {
		t.Fatal(err)
	}

	if err := RunInit("url-shortener", false); err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}
	projectDir := filepath.Join(examples, "url-shortener")
	if !workspace.IsRepoRoot(ctx, projectDir) {
		t.Fatalf("expected %s to be its own git repository", projectDir)
	}
	if _, err := workspace.CreateWorktree(ctx, projectDir, "T01", "main"); err != nil {
		t.Fatalf("CreateWorktree in nested project failed: %v", err)
	}
	if out, _ := workspace.RunGit(ctx, outer, "branch", "--list", "rm/T01"); strings.TrimSpace(out) != "" {
		t.Errorf("task branch leaked into the outer repository")
	}
}

// Running `init` in a subdirectory of an existing repo is refused before any
// file is written.
func TestInitCurrentDirInsideRepoRefused(t *testing.T) {
	outer := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if _, err := workspace.RunGit(context.Background(), outer, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(outer, "pkg")
	_ = os.MkdirAll(sub, 0755)
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	if err := RunInit("", false); err == nil {
		t.Fatalf("expected init inside a repo subdirectory to fail")
	}
	if entries, _ := os.ReadDir(sub); len(entries) != 0 {
		t.Errorf("refused init must not write files, found %d entries", len(entries))
	}
}

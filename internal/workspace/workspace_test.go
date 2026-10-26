package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo initializes a git repo inside tempDir/repo with an initial commit on main
func setupTestRepo(t *testing.T) string {
	t.Helper()
	projectRoot := t.TempDir()
	repoPath := filepath.Join(projectRoot, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	ctx := context.Background()
	if _, err := RunGit(ctx, repoPath, "init", "-b", "main"); err != nil {
		t.Fatalf("failed git init: %v", err)
	}
	_ = EnsureGitIdentity(ctx, repoPath)

	readme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readme, []byte("# Ringmaster Test\n"), 0644); err != nil {
		t.Fatalf("failed writing initial README: %v", err)
	}

	if _, err := RunGit(ctx, repoPath, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := RunGit(ctx, repoPath, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return projectRoot
}

func TestCreateAndRemoveWorktreeIdempotent(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()

	// 1. Create worktree T01
	wtPath, err := CreateWorktree(ctx, projectRoot, "T01", "main")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(wtPath, "README.md")); err != nil {
		t.Errorf("README.md missing in worktree: %v", err)
	}

	// 2. Idempotency check: calling CreateWorktree again should not fail
	wtPath2, err := CreateWorktree(ctx, projectRoot, "T01", "main")
	if err != nil {
		t.Fatalf("idempotent CreateWorktree failed: %v", err)
	}
	if wtPath != wtPath2 {
		t.Errorf("expected same path on reuse: %s vs %s", wtPath, wtPath2)
	}

	// 3. Remove worktree
	if err := RemoveWorktree(ctx, projectRoot, "T01"); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after remove")
	}
}

func TestMergeQueueSuccess(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()
	repoPath := filepath.Join(projectRoot, "repo")

	// Branch 1: Task T01 writes file_a.txt
	wtA, err := CreateWorktree(ctx, projectRoot, "T01", "main")
	if err != nil {
		t.Fatalf("CreateWorktree T01: %v", err)
	}
	_ = os.WriteFile(filepath.Join(wtA, "file_a.txt"), []byte("Feature A\n"), 0644)
	_, _ = RunGit(ctx, wtA, "add", "file_a.txt")
	_, _ = RunGit(ctx, wtA, "commit", "-m", "implement feature a")

	// Branch 2: Task T02 writes file_b.txt
	wtB, err := CreateWorktree(ctx, projectRoot, "T02", "main")
	if err != nil {
		t.Fatalf("CreateWorktree T02: %v", err)
	}
	_ = os.WriteFile(filepath.Join(wtB, "file_b.txt"), []byte("Feature B\n"), 0644)
	_, _ = RunGit(ctx, wtB, "add", "file_b.txt")
	_, _ = RunGit(ctx, wtB, "commit", "-m", "implement feature b")

	mq := NewMergeQueue(projectRoot)

	// Merge T01
	resA, err := mq.Enqueue(ctx, "T01")
	if err != nil {
		t.Fatalf("merge T01 failed: %v", err)
	}
	if !resA.Merged || resA.Conflict {
		t.Errorf("expected T01 to merge successfully: %+v", resA)
	}

	// Merge T02
	resB, err := mq.Enqueue(ctx, "T02")
	if err != nil {
		t.Fatalf("merge T02 failed: %v", err)
	}
	if !resB.Merged || resB.Conflict {
		t.Errorf("expected T02 to merge successfully: %+v", resB)
	}

	// Verify main has both files
	if _, err := os.Stat(filepath.Join(repoPath, "file_a.txt")); err != nil {
		t.Errorf("main missing file_a.txt")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "file_b.txt")); err != nil {
		t.Errorf("main missing file_b.txt")
	}
}

func TestMergeQueueConflictAbortsCleanly(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()
	repoPath := filepath.Join(projectRoot, "repo")

	// Both branches modify the SAME file with conflicting content
	wt1, _ := CreateWorktree(ctx, projectRoot, "T01", "main")
	_ = os.WriteFile(filepath.Join(wt1, "conflict.txt"), []byte("Alpha version\n"), 0644)
	_, _ = RunGit(ctx, wt1, "add", "conflict.txt")
	_, _ = RunGit(ctx, wt1, "commit", "-m", "conflict alpha")

	wt2, _ := CreateWorktree(ctx, projectRoot, "T02", "main")
	_ = os.WriteFile(filepath.Join(wt2, "conflict.txt"), []byte("Beta version\n"), 0644)
	_, _ = RunGit(ctx, wt2, "add", "conflict.txt")
	_, _ = RunGit(ctx, wt2, "commit", "-m", "conflict beta")

	mq := NewMergeQueue(projectRoot)

	// Merge T01 first (succeeds)
	res1, err := mq.Enqueue(ctx, "T01")
	if err != nil || !res1.Merged {
		t.Fatalf("expected T01 to merge cleanly: %v", err)
	}

	// Merge T02 (must conflict and abort cleanly)
	res2, err := mq.Enqueue(ctx, "T02")
	if err != nil {
		t.Fatalf("unexpected error on conflict: %v", err)
	}

	if !res2.Conflict || res2.Merged {
		t.Errorf("expected conflict flag=true, merged=false: %+v", res2)
	}

	if len(res2.ConflictedFiles) == 0 || !strings.Contains(res2.ConflictedFiles[0], "conflict.txt") {
		t.Errorf("expected conflict.txt in ConflictedFiles, got: %v", res2.ConflictedFiles)
	}

	// Check that main is clean and not in MERGING state
	status, err := RunGit(ctx, repoPath, "status")
	if err != nil {
		t.Fatalf("git status error: %v", err)
	}
	if strings.Contains(status, "You have unmerged paths") || strings.Contains(status, "All conflicts fixed") {
		t.Errorf("main repository was left in dirty merging state: %s", status)
	}
}

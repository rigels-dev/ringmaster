package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo initializes a git repo in tempDir with an initial commit on main
func setupTestRepo(t *testing.T) string {
	t.Helper()
	projectRoot := t.TempDir()
	repoPath := projectRoot
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
	repoPath := projectRoot

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
	repoPath := projectRoot

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

func TestCreateWorktreeRefusesNestedProject(t *testing.T) {
	outer := setupTestRepo(t)
	nested := filepath.Join(outer, "examples", "app")
	_ = os.MkdirAll(nested, 0755)

	if _, err := CreateWorktree(context.Background(), nested, "T01", "main"); err == nil {
		t.Fatalf("expected CreateWorktree to refuse a project nested in another repo")
	}
}

// taskCommit makes a worktree for taskID that writes file with content.
func taskCommit(t *testing.T, ctx context.Context, projectRoot, taskID, file, content string) {
	t.Helper()
	wt, err := CreateWorktree(ctx, projectRoot, taskID, "main")
	if err != nil {
		t.Fatalf("CreateWorktree %s: %v", taskID, err)
	}
	_ = os.WriteFile(filepath.Join(wt, file), []byte(content), 0644)
	if _, err := CommitAll(ctx, wt, taskID+": work", nil); err != nil {
		t.Fatalf("commit %s: %v", taskID, err)
	}
}

// The project is the user's working tree: merging must not switch their
// branch or touch their uncommitted files.
func TestMergeQueueLeavesUserCheckoutAlone(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()
	taskCommit(t, ctx, projectRoot, "T01", "feature.txt", "from task\n")

	// The user works on their own branch with an uncommitted change
	if _, err := RunGit(ctx, projectRoot, "checkout", "-b", "my-feature"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("user edit\n"), 0644)

	res, err := NewMergeQueue(projectRoot).Enqueue(ctx, "T01")
	if err != nil || !res.Merged {
		t.Fatalf("expected merge into main, got %+v, %v", res, err)
	}

	head, _ := RunGit(ctx, projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if strings.TrimSpace(head) != "my-feature" {
		t.Errorf("user's branch was switched to %q", strings.TrimSpace(head))
	}
	if data, _ := os.ReadFile(filepath.Join(projectRoot, "README.md")); string(data) != "user edit\n" {
		t.Errorf("user's uncommitted change was lost: %q", data)
	}
	if _, err := RunGit(ctx, projectRoot, "cat-file", "-e", "main:feature.txt"); err != nil {
		t.Errorf("main does not contain the task's file")
	}
	parents, _ := RunGit(ctx, projectRoot, "log", "-1", "--format=%P", "main")
	if len(strings.Fields(parents)) != 2 {
		t.Errorf("expected a two-parent merge commit on main, parents: %q", parents)
	}
}

func TestMergeQueueOnCheckedOutMainKeepsLocalChanges(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()
	taskCommit(t, ctx, projectRoot, "T01", "feature.txt", "from task\n")

	// On main, with an unrelated uncommitted edit
	_ = os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("user edit\n"), 0644)

	res, err := NewMergeQueue(projectRoot).Enqueue(ctx, "T01")
	if err != nil || !res.Merged {
		t.Fatalf("expected merge, got %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "feature.txt")); err != nil {
		t.Errorf("checked-out main was not updated with the task's file")
	}
	if data, _ := os.ReadFile(filepath.Join(projectRoot, "README.md")); string(data) != "user edit\n" {
		t.Errorf("user's uncommitted change was lost: %q", data)
	}
}

func TestMergeQueueRefusesToOverwriteUserChanges(t *testing.T) {
	projectRoot := setupTestRepo(t)
	ctx := context.Background()
	taskCommit(t, ctx, projectRoot, "T01", "README.md", "from task\n")
	before, _ := RunGit(ctx, projectRoot, "rev-parse", "main")

	// On main, editing the same file the task changed
	_ = os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("user edit\n"), 0644)

	res, err := NewMergeQueue(projectRoot).Enqueue(ctx, "T01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Merged || !strings.Contains(res.Message, "uncommitted changes") {
		t.Errorf("expected the merge to be refused with an explanation, got %+v", res)
	}
	after, _ := RunGit(ctx, projectRoot, "rev-parse", "main")
	if before != after {
		t.Errorf("main moved although the merge was refused")
	}
	if data, _ := os.ReadFile(filepath.Join(projectRoot, "README.md")); string(data) != "user edit\n" {
		t.Errorf("user's uncommitted change was overwritten: %q", data)
	}
}

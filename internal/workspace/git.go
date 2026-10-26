package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gitTimeout = 60 * time.Second

// RunGit runs a git command with -C <repoPath> explicitly.
func RunGit(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	tCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(tCtx, "git", cmdArgs...)
	// Ensure clean git environment
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Ringmaster",
		"GIT_AUTHOR_EMAIL=ringmaster@local",
		"GIT_COMMITTER_NAME=Ringmaster",
		"GIT_COMMITTER_EMAIL=ringmaster@local",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := stdout.String()
	errStr := stderr.String()

	if err != nil {
		combined := strings.TrimSpace(outStr + "\n" + errStr)
		return outStr, fmt.Errorf("git %s failed: %w (output: %s)", strings.Join(args, " "), err, combined)
	}

	return outStr, nil
}

// EnsureGitIdentity sets user.name and user.email locally in the repo if not set.
func EnsureGitIdentity(ctx context.Context, repoPath string) error {
	_, _ = RunGit(ctx, repoPath, "config", "user.name", "Ringmaster")
	_, _ = RunGit(ctx, repoPath, "config", "user.email", "ringmaster@local")
	return nil
}

// CreateWorktree creates an isolated git worktree for taskID under .worktrees/Txx.
// It branches from baseRef (default "main", or "rm/<upstream>").
// It is fully idempotent: if worktree already exists, it is safely reused.
func CreateWorktree(ctx context.Context, projectRoot, taskID, baseRef string) (string, error) {
	repoPath := filepath.Join(projectRoot, "repo")
	worktreesDir := filepath.Join(projectRoot, ".worktrees")
	targetDir := filepath.Join(worktreesDir, taskID)
	branchName := fmt.Sprintf("rm/%s", taskID)

	if baseRef == "" {
		baseRef = "main"
	}

	if err := EnsureGitIdentity(ctx, repoPath); err != nil {
		return "", err
	}

	// Check if target worktree directory already exists
	if fi, err := os.Stat(targetDir); err == nil && fi.IsDir() {
		// Verify if it is recognized as a worktree by git
		out, err := RunGit(ctx, repoPath, "worktree", "list", "--porcelain")
		if err == nil && strings.Contains(out, targetDir) {
			return targetDir, nil
		}
	}

	_ = os.MkdirAll(worktreesDir, 0755)

	// Check if branch already exists
	branchExists := false
	out, err := RunGit(ctx, repoPath, "branch", "--list", branchName)
	if err == nil && strings.TrimSpace(out) != "" {
		branchExists = true
	}

	if branchExists {
		// Worktree using existing branch
		_, err = RunGit(ctx, repoPath, "worktree", "add", targetDir, branchName)
	} else {
		// Worktree with new branch from baseRef
		_, err = RunGit(ctx, repoPath, "worktree", "add", "-b", branchName, targetDir, baseRef)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create worktree for %s: %w", taskID, err)
	}

	return targetDir, nil
}

// RemoveWorktree safely removes and prunes the worktree directory for taskID.
func RemoveWorktree(ctx context.Context, projectRoot, taskID string) error {
	repoPath := filepath.Join(projectRoot, "repo")
	targetDir := filepath.Join(projectRoot, ".worktrees", taskID)

	// Remove worktree from git
	_, _ = RunGit(ctx, repoPath, "worktree", "remove", "--force", targetDir)
	_, _ = RunGit(ctx, repoPath, "worktree", "prune")

	// Ensure directory cleanup
	_ = os.RemoveAll(targetDir)
	return nil
}

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

// StateDirName is the only directory Ringmaster adds to a project (besides
// ringmaster.toml). The project directory itself is the git repository.
const StateDirName = ".ringmaster"

// WorktreesDir is where per-task git worktrees live.
func WorktreesDir(projectRoot string) string {
	return filepath.Join(projectRoot, StateDirName, "worktrees")
}

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
// An identity the user already has (local or global) is left untouched.
func EnsureGitIdentity(ctx context.Context, repoPath string) error {
	if out, err := RunGit(ctx, repoPath, "config", "user.name"); err != nil || strings.TrimSpace(out) == "" {
		_, _ = RunGit(ctx, repoPath, "config", "user.name", "Ringmaster")
	}
	if out, err := RunGit(ctx, repoPath, "config", "user.email"); err != nil || strings.TrimSpace(out) == "" {
		_, _ = RunGit(ctx, repoPath, "config", "user.email", "ringmaster@local")
	}
	return nil
}

// EnsureStateExcluded lists .ringmaster/ in the repo's info/exclude so that
// Ringmaster state never shows up in git status or gets committed, without
// touching the project's own .gitignore. info/exclude lives in the common git
// dir, so it also covers every task worktree.
func EnsureStateExcluded(ctx context.Context, repoPath string) error {
	out, err := RunGit(ctx, repoPath, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	commonDir := strings.TrimSpace(out)
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoPath, commonDir)
	}
	excludePath := filepath.Join(commonDir, "info", "exclude")

	existing, _ := os.ReadFile(excludePath)
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == StateDirName+"/" {
			return nil
		}
	}

	_ = os.MkdirAll(filepath.Dir(excludePath), 0755)
	f, err := os.OpenFile(excludePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry := "# Ringmaster state\n" + StateDirName + "/\n"
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		entry = "\n" + entry
	}
	_, err = f.WriteString(entry)
	return err
}

// Trailer is one "Key: value" line in a commit message's trailer block.
type Trailer struct {
	Key, Value string
}

// CommitAll stages every change in dir and commits it with subject and
// trailers (e.g. Ringmaster-Task / Ringmaster-Automaton), so the history
// records which automaton produced each commit. It returns false when there
// was nothing to commit.
func CommitAll(ctx context.Context, dir, subject string, trailers []Trailer) (bool, error) {
	if _, err := RunGit(ctx, dir, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := RunGit(ctx, dir, "diff", "--cached", "--quiet"); err == nil {
		return false, nil
	}
	args := []string{"commit", "-m", subject}
	if len(trailers) > 0 {
		var lines []string
		for _, t := range trailers {
			lines = append(lines, t.Key+": "+t.Value)
		}
		args = append(args, "-m", strings.Join(lines, "\n"))
	}
	if _, err := RunGit(ctx, dir, args...); err != nil {
		return false, err
	}
	return true, nil
}

// IsRepoRoot reports whether dir is the top level of its own git repository.
// A directory merely nested inside another repository (e.g. a project under
// examples/ of some other checkout) is not.
func IsRepoRoot(ctx context.Context, dir string) bool {
	out, err := RunGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	return samePath(strings.TrimSpace(out), dir)
}

// EnsureRepoRoot refuses to operate on a project that is not its own git
// repository, so task branches and merges never land in an enclosing repo.
func EnsureRepoRoot(ctx context.Context, projectRoot string) error {
	if IsRepoRoot(ctx, projectRoot) {
		return nil
	}
	if out, err := RunGit(ctx, projectRoot, "rev-parse", "--show-toplevel"); err == nil {
		return fmt.Errorf("project %s is nested inside git repository %s, not a repository of its own; run 'git init' in the project directory", projectRoot, strings.TrimSpace(out))
	}
	return fmt.Errorf("project %s is not a git repository; run 'ringmaster init' or 'git init' there", projectRoot)
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// CreateWorktree creates an isolated git worktree for taskID under .ringmaster/worktrees/Txx.
// It branches from baseRef (default "main", or "rm/<upstream>").
// It is fully idempotent: if worktree already exists, it is safely reused.
func CreateWorktree(ctx context.Context, projectRoot, taskID, baseRef string) (string, error) {
	repoPath := projectRoot
	worktreesDir := WorktreesDir(projectRoot)
	targetDir := filepath.Join(worktreesDir, taskID)
	branchName := fmt.Sprintf("rm/%s", taskID)

	if baseRef == "" {
		baseRef = "main"
	}

	if err := EnsureRepoRoot(ctx, repoPath); err != nil {
		return "", err
	}
	if err := EnsureGitIdentity(ctx, repoPath); err != nil {
		return "", err
	}
	if err := EnsureStateExcluded(ctx, repoPath); err != nil {
		return "", err
	}

	// Check if target worktree directory already exists
	if fi, err := os.Stat(targetDir); err == nil && fi.IsDir() {
		// Verify if it is recognized as a worktree by git. Git lists resolved
		// paths, so compare as paths: the project may sit under a symlink
		// (on macOS, anything in /tmp or /var).
		out, err := RunGit(ctx, repoPath, "worktree", "list", "--porcelain")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if listed, ok := strings.CutPrefix(line, "worktree "); ok && samePath(listed, targetDir) {
					return targetDir, nil
				}
			}
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
	repoPath := projectRoot
	targetDir := filepath.Join(WorktreesDir(projectRoot), taskID)

	// Remove worktree from git
	_, _ = RunGit(ctx, repoPath, "worktree", "remove", "--force", targetDir)
	_, _ = RunGit(ctx, repoPath, "worktree", "prune")

	// Ensure directory cleanup
	_ = os.RemoveAll(targetDir)
	return nil
}

// RestoreFromBase undoes a task's changes to files that already existed where
// its branch started (the merge base of the worktree's HEAD and baseRef) and
// that protect selects: modified ones get their old content back, deleted ones
// reappear. Files the task added are left alone. It returns the restored
// paths, slash-separated and sorted by git.
func RestoreFromBase(ctx context.Context, worktree, baseRef string, protect func(path string) bool) ([]string, error) {
	if baseRef == "" {
		baseRef = "main"
	}
	out, err := RunGit(ctx, worktree, "merge-base", "HEAD", baseRef)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(out)

	// Compares the base commit with the working tree, so it sees committed,
	// staged and unstaged changes alike. core.quotePath off keeps names literal.
	out, err = RunGit(ctx, worktree, "-c", "core.quotePath=false", "diff", "--name-status", "--no-renames", base, "--")
	if err != nil {
		return nil, err
	}
	var restore []string
	for _, line := range strings.Split(out, "\n") {
		status, file, ok := strings.Cut(line, "\t")
		if !ok || status == "A" {
			continue
		}
		if protect(file) {
			restore = append(restore, file)
		}
	}

	const batch = 100 // keep each command line short
	for i := 0; i < len(restore); i += batch {
		end := min(i+batch, len(restore))
		args := append([]string{"checkout", base, "--"}, restore[i:end]...)
		if _, err := RunGit(ctx, worktree, args...); err != nil {
			return nil, err
		}
	}
	return restore, nil
}

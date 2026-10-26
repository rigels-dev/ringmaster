package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// MergeResult represents the outcome of a merge operation into main.
type MergeResult struct {
	TaskID          string   `json:"task_id"`
	Branch          string   `json:"branch"`
	Merged          bool     `json:"merged"`
	Conflict        bool     `json:"conflict"`
	ConflictedFiles []string `json:"conflicted_files,omitempty"`
	CommitHash      string   `json:"commit_hash,omitempty"`
	Message         string   `json:"message"`
}

// MergeQueue serializes git merge operations into the main branch.
type MergeQueue struct {
	projectRoot string
	repoPath    string
	mu          sync.Mutex
}

// NewMergeQueue creates a new MergeQueue for a project repository.
func NewMergeQueue(projectRoot string) *MergeQueue {
	return &MergeQueue{
		projectRoot: projectRoot,
		repoPath:    filepath.Join(projectRoot, "repo"),
	}
}

// Enqueue acquires the lock and attempts to merge branch rm/<taskID> into main.
// If a conflict occurs, it aborts the merge cleanly without corrupting main.
func (mq *MergeQueue) Enqueue(ctx context.Context, taskID string) (*MergeResult, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	branchName := fmt.Sprintf("rm/%s", taskID)
	result := &MergeResult{
		TaskID: taskID,
		Branch: branchName,
	}

	if err := EnsureGitIdentity(ctx, mq.repoPath); err != nil {
		return nil, err
	}

	// 1. Ensure we are on main
	if _, err := RunGit(ctx, mq.repoPath, "checkout", "main"); err != nil {
		return nil, fmt.Errorf("failed to checkout main: %w", err)
	}

	// 2. Perform merge with --no-ff
	mergeMsg := fmt.Sprintf("Merge task %s into main [Ringmaster]", taskID)
	_, mergeErr := RunGit(ctx, mq.repoPath, "merge", "--no-ff", "-m", mergeMsg, branchName)

	if mergeErr != nil {
		// Detect conflicted files
		statusOut, _ := RunGit(ctx, mq.repoPath, "status", "--porcelain")
		var conflicts []string
		for _, line := range strings.Split(statusOut, "\n") {
			line = strings.TrimSpace(line)
			// Git porcelain status UU, AA, UD, DU indicates unmerged conflict
			if strings.HasPrefix(line, "UU ") || strings.HasPrefix(line, "AA ") ||
				strings.HasPrefix(line, "UD ") || strings.HasPrefix(line, "DU ") {
				conflicts = append(conflicts, strings.TrimSpace(line[3:]))
			}
		}

		// Abort merge cleanly
		_, _ = RunGit(ctx, mq.repoPath, "merge", "--abort")

		result.Conflict = true
		result.ConflictedFiles = conflicts
		result.Message = fmt.Sprintf("Merge conflict encountered on %s", strings.Join(conflicts, ", "))
		return result, nil
	}

	// 3. Merge succeeded, get commit hash
	hashOut, err := RunGit(ctx, mq.repoPath, "rev-parse", "HEAD")
	if err == nil {
		result.CommitHash = strings.TrimSpace(hashOut)
	}
	result.Merged = true
	result.Message = fmt.Sprintf("Successfully merged %s into main (%s)", branchName, result.CommitHash[:min(7, len(result.CommitHash))])

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

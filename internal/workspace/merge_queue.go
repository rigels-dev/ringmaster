package workspace

import (
	"context"
	"fmt"
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
		repoPath:    projectRoot,
	}
}

// Enqueue acquires the lock and attempts to merge branch rm/<taskID> into main.
// It never checks out a branch in the project's working tree; on a conflict,
// main is left untouched and the conflicted files are reported.
func (mq *MergeQueue) Enqueue(ctx context.Context, taskID string) (*MergeResult, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	branchName := fmt.Sprintf("rm/%s", taskID)
	result := &MergeResult{
		TaskID: taskID,
		Branch: branchName,
	}

	if err := EnsureRepoRoot(ctx, mq.repoPath); err != nil {
		return nil, err
	}
	if err := EnsureGitIdentity(ctx, mq.repoPath); err != nil {
		return nil, err
	}

	// The project directory is the user's own working tree, so the merge is
	// computed without checking anything out: merge-tree builds the merged
	// tree, commit-tree records it, and only then is main moved forward.
	mainTip, err := RunGit(ctx, mq.repoPath, "rev-parse", "--verify", "refs/heads/main")
	if err != nil {
		return nil, fmt.Errorf("main branch not found: %w", err)
	}
	mainTip = strings.TrimSpace(mainTip)
	branchTip, err := RunGit(ctx, mq.repoPath, "rev-parse", "--verify", "refs/heads/"+branchName)
	if err != nil {
		return nil, fmt.Errorf("branch %s not found: %w", branchName, err)
	}
	branchTip = strings.TrimSpace(branchTip)

	// 1. Merge in memory. On conflict git exits non-zero and lists the files.
	mergeOut, mergeErr := RunGit(ctx, mq.repoPath, "merge-tree", "--write-tree", "--name-only", "--no-messages", mainTip, branchTip)
	lines := strings.Split(strings.TrimRight(mergeOut, "\n"), "\n")
	tree := strings.TrimSpace(lines[0])
	if !isObjectID(tree) {
		return nil, fmt.Errorf("merge of %s failed: %w", branchName, mergeErr)
	}
	if mergeErr != nil {
		var conflicts []string
		for _, line := range lines[1:] {
			if line = strings.TrimSpace(line); line != "" {
				conflicts = append(conflicts, line)
			}
		}
		result.Conflict = true
		result.ConflictedFiles = conflicts
		result.Message = fmt.Sprintf("Merge conflict encountered on %s", strings.Join(conflicts, ", "))
		return result, nil
	}

	// 2. Record the merge commit (main first, so main's first-parent line stays intact)
	mergeMsg := fmt.Sprintf("Merge task %s into main [Ringmaster]", taskID)
	commitOut, err := RunGit(ctx, mq.repoPath, "commit-tree", tree, "-p", mainTip, "-p", branchTip, "-m", mergeMsg, "-m", "Ringmaster-Task: "+taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge commit: %w", err)
	}
	mergeCommit := strings.TrimSpace(commitOut)

	// 3. Move main forward without disturbing the user's checkout
	if err := mq.advanceMain(ctx, mainTip, mergeCommit); err != nil {
		result.Message = err.Error()
		return result, nil
	}

	result.CommitHash = mergeCommit
	result.Merged = true
	result.Message = fmt.Sprintf("Successfully merged %s into main (%s)", branchName, mergeCommit[:min(7, len(mergeCommit))])
	return result, nil
}

// advanceMain moves main from oldTip to newTip. When main is what the user has
// checked out, a fast-forward merge updates their files too and refuses (rather
// than overwrite) if their uncommitted changes touch the same files. Otherwise
// only the branch ref moves, guarded against main having changed meanwhile.
func (mq *MergeQueue) advanceMain(ctx context.Context, oldTip, newTip string) error {
	head, _ := RunGit(ctx, mq.repoPath, "symbolic-ref", "-q", "HEAD")
	if strings.TrimSpace(head) == "refs/heads/main" {
		if _, err := RunGit(ctx, mq.repoPath, "merge", "--ff-only", "--no-edit", newTip); err != nil {
			return fmt.Errorf("main is checked out in %s with uncommitted changes that overlap this task; commit or stash them, then resume", mq.repoPath)
		}
		return nil
	}
	if _, err := RunGit(ctx, mq.repoPath, "update-ref", "-m", "ringmaster: merge task", "refs/heads/main", newTip, oldTip); err != nil {
		return fmt.Errorf("main moved while merging; resume to retry: %w", err)
	}
	return nil
}

func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

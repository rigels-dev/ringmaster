package cli

import (
	"context"

	"ringmaster/internal/workspace"
)

// gitIdentityWarning returns advice to print when the user has no git
// identity, and "" when they have one. Without one, commits are authored by
// Ringmaster and cannot be attributed to the user afterwards without
// rewriting history, so it is worth saying before the first commit is made.
// Ringmaster only gives the commands; it never edits the user's git config.
func gitIdentityWarning(ctx context.Context, dir string) string {
	if workspace.CommitAuthor(ctx, dir).IsUser {
		return ""
	}
	return `⚠ Git has no user.name / user.email set.
  Commits will be authored by "Ringmaster" and will not be linked to your account on a git host.
  To make them yours (use an email your git host knows), run once:
    git config --global user.name  "Your Name"
    git config --global user.email "you@example.com"
`
}

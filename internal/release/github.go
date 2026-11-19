package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// GitHub is what Ringmaster knows about publishing release pages for a
// project whose remote is on GitHub.
type GitHub struct {
	// Repo is "owner/name"; "" when the remote is not on GitHub.
	Repo string `json:"repo"`
	// CLI is whether the GitHub CLI (gh), which creates the pages, is installed.
	CLI bool `json:"cli"`
}

var gitHubRemote = regexp.MustCompile(`^(?:https://(?:[^@/]+@)?github\.com/|ssh://git@github\.com/|git@github\.com:)([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

// GitHubRepo returns "owner/name" for a GitHub remote URL, or "".
func GitHubRepo(remoteURL string) string {
	m := gitHubRemote.FindStringSubmatch(strings.TrimSpace(remoteURL))
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// GitHubInfo describes release pages for the given remote.
func GitHubInfo(remoteURL string) GitHub {
	return GitHub{Repo: GitHubRepo(remoteURL), CLI: hasGH()}
}

// ghTimeout bounds one gh call; creating a page is a single API request.
const ghTimeout = 60 * time.Second

// runGH is how gh is run; tests replace it.
var runGH = func(ctx context.Context, dir string, args ...string) (string, error) {
	tCtx, cancel := context.WithTimeout(ctx, ghTimeout)
	defer cancel()
	cmd := exec.CommandContext(tCtx, "gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = err.Error()
		}
		return "", errors.New(reason)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CreatePage creates the GitHub release page of version, which must be
// pushed already, with title and notes, and returns its URL. The page is
// published, not a draft: Tag and push is the user's choice to publish.
func CreatePage(ctx context.Context, projectRoot, remoteURL, version, title, notes string) (string, error) {
	repo := GitHubRepo(remoteURL)
	if repo == "" {
		return "", errors.New("release pages are made on GitHub, and the remote is not a GitHub repository")
	}
	if strings.HasPrefix(version, "-") {
		return "", errors.New("a version cannot start with '-'")
	}
	if !hasGH() {
		return "", errors.New("the GitHub CLI (gh) is not installed; install it and run 'gh auth login' to create release pages")
	}
	if err := CheckNotes(notes); err != nil {
		return "", err
	}

	file, err := os.CreateTemp("", "ringmaster-notes-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString(CleanNotes(notes) + "\n")
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}

	if title == "" {
		title = version
	}
	// --verify-tag: the page belongs to the tag that was pushed, never to one gh would make
	out, err := runGH(ctx, projectRoot, "release", "create", version,
		"--repo", repo, "--title", title, "--notes-file", file.Name(), "--verify-tag")
	if err != nil {
		reason := err.Error()
		switch {
		case strings.Contains(reason, "already exists"):
			return "", fmt.Errorf("%s already has a release page on GitHub", version)
		case strings.Contains(reason, "gh auth login"), strings.Contains(strings.ToLower(reason), "not logged"):
			return "", errors.New("the GitHub CLI is not logged in; run 'gh auth login' first")
		}
		return "", fmt.Errorf("gh could not create the release page: %s", reason)
	}
	// gh prints the page's URL last
	lines := strings.Split(out, "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

// hasGH reports whether gh is installed; tests replace it with runGH.
var hasGH = func() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

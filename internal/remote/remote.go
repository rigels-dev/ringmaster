// Package remote connects a project to a git remote and pushes Ringmaster's
// branches to it: main, plus the rm/Txx task branches.
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"ringmaster/internal/workspace"
)

const (
	// Name is the only remote Ringmaster manages.
	Name = "origin"
	// MainBranch is the branch tasks are merged into.
	MainBranch = "main"
	// TaskPrefix is the namespace of task branches.
	TaskPrefix = "rm/"

	pushTimeout = 3 * time.Minute
)

// Branch states, comparing a local branch with what was last pushed.
const (
	StateSynced   = "synced"   // the remote has this commit
	StateAhead    = "ahead"    // pushed before, and has moved since
	StateUnpushed = "unpushed" // never pushed
)

// Branch is one local branch Ringmaster would push.
type Branch struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// Status is the project's remote and how its branches compare with it. It is
// read from local refs only, so it reflects the last push or fetch.
type Status struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"` // "" when the project has no remote yet
	Branches []Branch `json:"branches"`
}

// Pending counts the branches that a push would send something for.
func (s Status) Pending() int {
	n := 0
	for _, b := range s.Branches {
		if b.State != StateSynced {
			n++
		}
	}
	return n
}

var scpLikeURL = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._~/-]+$`)

// ValidateURL accepts the remote forms people paste from a git host:
// https://host/owner/repo(.git), ssh://…, git@host:owner/repo(.git), and
// file:// for a local repository. Everything else is refused, including
// anything git could read as an option or as a command to run (ext::…).
func ValidateURL(raw string) error {
	if raw == "" {
		return errors.New("the remote URL is empty")
	}
	if strings.HasPrefix(raw, "-") {
		return errors.New("a remote URL cannot start with '-'")
	}
	for _, r := range raw {
		if r <= ' ' || r == 0x7f {
			return errors.New("a remote URL cannot contain spaces or control characters")
		}
	}

	if !strings.Contains(raw, "://") {
		if scpLikeURL.MatchString(raw) {
			return nil
		}
		return errors.New("use an https:// or ssh:// URL, or the git@host:owner/repo form")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	switch u.Scheme {
	case "https", "http":
		if u.Host == "" {
			return errors.New("the URL has no host")
		}
		if u.User != nil {
			// It would be stored in .git/config and shown in the UI
			return errors.New("leave credentials out of the URL; use a git credential helper or an SSH URL")
		}
	case "ssh":
		if u.Host == "" {
			return errors.New("the URL has no host")
		}
	case "file":
		if u.Path == "" {
			return errors.New("the URL has no path")
		}
	default:
		return fmt.Errorf("unsupported URL scheme %q; use https, ssh or git@host:owner/repo", u.Scheme)
	}
	return nil
}

// URL returns the project's origin URL, or "" when there is none.
func URL(ctx context.Context, projectRoot string) string {
	out, err := workspace.RunGit(ctx, projectRoot, "remote", "get-url", Name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SetURL points origin at url, adding the remote when the project has none.
func SetURL(ctx context.Context, projectRoot, remoteURL string) error {
	if err := ValidateURL(remoteURL); err != nil {
		return err
	}
	verb := "add"
	if URL(ctx, projectRoot) != "" {
		verb = "set-url"
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "remote", verb, "--", Name, remoteURL); err != nil {
		return err
	}
	return nil
}

// GetStatus reports the remote and the state of main and every task branch.
func GetStatus(ctx context.Context, projectRoot string) (Status, error) {
	st := Status{Name: Name, URL: URL(ctx, projectRoot), Branches: []Branch{}}

	out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref",
		"--format=%(refname) %(objectname)",
		"refs/heads/"+MainBranch, "refs/heads/"+strings.TrimSuffix(TaskPrefix, "/"), "refs/remotes/"+Name)
	if err != nil {
		return st, err
	}

	local := map[string]string{}
	pushed := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		ref, sha, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if name, isLocal := strings.CutPrefix(ref, "refs/heads/"); isLocal {
			local[name] = sha
		} else if name, isRemote := strings.CutPrefix(ref, "refs/remotes/"+Name+"/"); isRemote {
			pushed[name] = sha
		}
	}

	names := make([]string, 0, len(local))
	for name := range local {
		names = append(names, name)
	}
	// main first, then the task branches in order
	sort.Slice(names, func(i, j int) bool {
		if (names[i] == MainBranch) != (names[j] == MainBranch) {
			return names[i] == MainBranch
		}
		return names[i] < names[j]
	})

	for _, name := range names {
		state := StateUnpushed
		if sha, ok := pushed[name]; ok {
			state = StateAhead
			if sha == local[name] {
				state = StateSynced
			}
		}
		st.Branches = append(st.Branches, Branch{Name: name, State: state})
	}
	return st, nil
}

// Outcomes of pushing one branch.
const (
	ResultNew      = "new"        // created on the remote
	ResultUpdated  = "updated"    // fast-forwarded
	ResultSame     = "up-to-date" // nothing to send
	ResultRejected = "rejected"   // the remote refused it
)

// PushResult is what happened to one branch.
type PushResult struct {
	Branch string `json:"branch"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// Push sends main to origin, and the task branches too unless mainOnly is
// set. It never forces: a branch the remote has moved on from is rejected and
// reported. It does not prompt, so missing credentials fail fast instead of
// hanging. The results list what happened to each branch even when the push
// as a whole returns an error.
func Push(ctx context.Context, projectRoot string, mainOnly bool) ([]PushResult, error) {
	remoteURL := URL(ctx, projectRoot)
	if remoteURL == "" {
		return nil, errors.New("this project has no remote yet; set one first (ringmaster remote set <url>)")
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+MainBranch); err != nil {
		return nil, fmt.Errorf("the project has no %q branch to push", MainBranch)
	}

	args := []string{"-C", projectRoot, "push", "--porcelain", Name,
		"refs/heads/" + MainBranch + ":refs/heads/" + MainBranch}
	if !mainOnly {
		args = append(args, "refs/heads/"+TaskPrefix+"*:refs/heads/"+TaskPrefix+"*")
	}

	tCtx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	cmd := exec.CommandContext(tCtx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",                // never wait for a username or password
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes", // nor for a passphrase or host-key answer
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	results := parsePorcelain(stdout.String())
	if tCtx.Err() == context.DeadlineExceeded {
		return results, fmt.Errorf("the push to %s timed out after %s", remoteURL, pushTimeout)
	}
	if runErr != nil {
		return results, fmt.Errorf("push to %s failed: %s", remoteURL, pushFailure(results, stderr.String()))
	}
	return results, nil
}

// parsePorcelain reads `git push --porcelain` lines of the form
// "<flag>\t<from>:<to>\t<summary>".
func parsePorcelain(out string) []PushResult {
	results := []PushResult{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue // "To <url>" and "Done"
		}
		from, _, _ := strings.Cut(fields[1], ":")
		branch, ok := strings.CutPrefix(from, "refs/heads/")
		if !ok {
			continue
		}
		res := PushResult{Branch: branch}
		switch fields[0] {
		case "*":
			res.Result = ResultNew
		case "=":
			res.Result = ResultSame
		case "!":
			res.Result = ResultRejected
			res.Detail = strings.TrimSpace(fields[2])
		default: // " " fast-forward (and "+" forced, which Push never asks for)
			res.Result = ResultUpdated
		}
		results = append(results, res)
	}
	return results
}

// pushFailure picks the most useful line to show for a failed push.
func pushFailure(results []PushResult, stderr string) string {
	var rejected []string
	for _, r := range results {
		if r.Result == ResultRejected {
			rejected = append(rejected, r.Branch)
		}
	}
	if len(rejected) > 0 {
		return fmt.Sprintf("the remote rejected %s (it has commits this project does not; pull or merge them first)",
			strings.Join(rejected, ", "))
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "remote:") {
			return line
		}
	}
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return last
	}
	return "git push exited with an error"
}

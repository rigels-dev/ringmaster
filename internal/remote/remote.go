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
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

const (
	// Name is the only remote Ringmaster manages.
	Name = "origin"
	// MainBranch is the branch tasks are merged into.
	MainBranch = "main"
	// TaskPrefix is the namespace task branches had before they were given
	// readable names. Branches in it are still treated as task branches.
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
	// Merged marks a task branch whose commits are all in main. Its work is
	// published with main, so it need not be pushed, and it can be pruned.
	Merged bool `json:"merged,omitempty"`
}

// Status is the project's remote and how its branches compare with it. It is
// read from local refs only, so it reflects the last push or fetch.
type Status struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"` // "" when the project has no remote yet
	Branches []Branch `json:"branches"`
	// Author is who the project's commits are attributed to, which is what a
	// git host will show once they are pushed.
	Author workspace.Author `json:"author"`
}

// Pending counts the branches whose work is not on the remote yet: main, and
// task branches not merged into main. A merged task branch's commits go
// with main, so it is not counted even when the branch itself was never
// pushed.
func (s Status) Pending() int {
	n := 0
	for _, b := range s.Branches {
		if b.State != StateSynced && !b.Merged {
			n++
		}
	}
	return n
}

// Merged counts the task branches that are merged into main and could be
// pruned.
func (s Status) Merged() int {
	n := 0
	for _, b := range s.Branches {
		if b.Merged {
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
	st := Status{
		Name:     Name,
		URL:      URL(ctx, projectRoot),
		Branches: []Branch{},
		Author:   workspace.CommitAuthor(ctx, projectRoot),
	}

	out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref",
		"--format=%(refname) %(objectname)", "refs/heads", "refs/remotes/"+Name)
	if err != nil {
		return st, err
	}

	// Only main and the task branches are Ringmaster's to report and push
	mine := map[string]bool{MainBranch: true}
	for _, b := range recordedTaskBranches(projectRoot) {
		mine[b] = true
	}

	local := map[string]string{}
	pushed := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		ref, sha, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if name, isLocal := strings.CutPrefix(ref, "refs/heads/"); isLocal {
			if mine[name] || strings.HasPrefix(name, TaskPrefix) {
				local[name] = sha
			}
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

	merged := mergedIntoMain(ctx, projectRoot)
	for _, name := range names {
		state := StateUnpushed
		if sha, ok := pushed[name]; ok {
			state = StateAhead
			if sha == local[name] {
				state = StateSynced
			}
		}
		st.Branches = append(st.Branches, Branch{Name: name, State: state, Merged: name != MainBranch && merged[name]})
	}
	return st, nil
}

// mergedIntoMain lists the local branches whose tips main contains. A
// branch checked out in a worktree is left out: a task that is running, or
// waiting on the user, may not have committed yet, and its branch is then
// "merged" only because it has nothing of its own.
func mergedIntoMain(ctx context.Context, projectRoot string) map[string]bool {
	out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref", "--merged", "refs/heads/"+MainBranch,
		"--format=%(refname:short)", "refs/heads")
	merged := map[string]bool{}
	if err != nil {
		return merged
	}
	for _, name := range strings.Fields(out) {
		merged[name] = true
	}
	if wt, err := workspace.RunGit(ctx, projectRoot, "worktree", "list", "--porcelain"); err == nil {
		for _, line := range strings.Split(wt, "\n") {
			if name, ok := strings.CutPrefix(strings.TrimSpace(line), "branch refs/heads/"); ok {
				delete(merged, name)
			}
		}
	}
	return merged
}

// Prune outcomes for one branch.
const (
	PruneDeleted       = "deleted"        // the local branch is gone
	PruneRemoteDeleted = "remote-deleted" // the branch on the remote is gone
	PruneKept          = "kept"           // left alone; Detail says why
)

// PruneResult is what happened to one branch, locally or on the remote.
type PruneResult struct {
	Branch string `json:"branch"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// Prune deletes the task branches that are merged into main. Their commits
// stay in main's history, merge commits and Ringmaster-* trailers included;
// only the branch names go. main, a task branch that is not merged, and any
// branch of the user's own are never touched. A branch still checked out in
// a worktree is kept.
//
// With onRemote, the same branches are also deleted on origin, where the
// remote still has them as they were last fetched or pushed and main
// contains that commit: a branch someone has moved on the remote is kept.
func Prune(ctx context.Context, projectRoot string, onRemote bool) ([]PruneResult, error) {
	merged := mergedIntoMain(ctx, projectRoot)
	results := []PruneResult{}

	// What origin has, from the last fetch or push, before anything changes
	remoteTips := map[string]string{}
	if onRemote {
		if URL(ctx, projectRoot) == "" {
			return nil, errors.New("this project has no remote yet; set one first (ringmaster remote set <url>)")
		}
		out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref", "--format=%(refname) %(objectname)", "refs/remotes/"+Name)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			ref, sha, ok := strings.Cut(line, " ")
			if name, isBranch := strings.CutPrefix(ref, "refs/remotes/"+Name+"/"); ok && isBranch {
				remoteTips[name] = sha
			}
		}
	}

	var toDeleteRemote []string
	for _, branch := range TaskBranches(ctx, projectRoot) {
		if !merged[branch] {
			continue
		}
		// -d, not -D: git refuses a branch that is not merged or is checked out
		if _, err := workspace.RunGit(ctx, projectRoot, "branch", "-d", "--", branch); err != nil {
			results = append(results, PruneResult{Branch: branch, Result: PruneKept, Detail: gitReason(err)})
			continue
		}
		results = append(results, PruneResult{Branch: branch, Result: PruneDeleted})
		if sha, ok := remoteTips[branch]; ok {
			if _, err := workspace.RunGit(ctx, projectRoot, "merge-base", "--is-ancestor", sha, "refs/heads/"+MainBranch); err == nil {
				toDeleteRemote = append(toDeleteRemote, branch)
			} else {
				results = append(results, PruneResult{Branch: branch, Result: PruneKept,
					Detail: "on " + Name + " it has commits that are not in main"})
			}
		}
	}

	// Task branches already gone here but still on the remote
	if onRemote {
		recorded := map[string]bool{}
		for _, b := range recordedTaskBranches(projectRoot) {
			recorded[b] = true
		}
		var leftover []string
		for name := range remoteTips {
			if name == MainBranch || !(recorded[name] || strings.HasPrefix(name, TaskPrefix)) {
				continue
			}
			if slices.Contains(toDeleteRemote, name) {
				continue // deleted here just now, and already queued
			}
			if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
				continue // still here, so not merged or still in use: handled above
			}
			if _, err := workspace.RunGit(ctx, projectRoot, "merge-base", "--is-ancestor", remoteTips[name], "refs/heads/"+MainBranch); err == nil {
				leftover = append(leftover, name)
			}
		}
		sort.Strings(leftover)
		toDeleteRemote = append(toDeleteRemote, leftover...)
	}

	if len(toDeleteRemote) == 0 {
		return results, nil
	}
	// Each deletion is leased on the commit last seen there, so a branch
	// that moved on the remote since is refused instead of lost
	refspecs := make([]string, 0, 2*len(toDeleteRemote))
	for _, branch := range toDeleteRemote {
		refspecs = append(refspecs, fmt.Sprintf("--force-with-lease=refs/heads/%s:%s", branch, remoteTips[branch]))
	}
	for _, branch := range toDeleteRemote {
		refspecs = append(refspecs, ":refs/heads/"+branch)
	}
	pushed, err := gitPush(ctx, projectRoot, refspecs)
	for _, r := range pushed {
		switch r.Result {
		case ResultRejected:
			results = append(results, PruneResult{Branch: r.Branch, Result: PruneKept, Detail: "on " + Name + ": " + r.Detail})
		default:
			results = append(results, PruneResult{Branch: r.Branch, Result: PruneRemoteDeleted})
			// The remote-tracking ref would otherwise show it as still there
			_, _ = workspace.RunGit(ctx, projectRoot, "update-ref", "-d", "refs/remotes/"+Name+"/"+r.Branch)
		}
	}
	return results, err
}

// gitReason is the first line git gave for a failure.
func gitReason(err error) string {
	msg := strings.TrimSpace(err.Error())
	for _, line := range strings.Split(msg, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "fatal:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "error:"), "fatal:"))
		}
	}
	return msg
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
	refspecs := []string{"refs/heads/" + MainBranch + ":refs/heads/" + MainBranch}
	if !mainOnly {
		for _, branch := range TaskBranches(ctx, projectRoot) {
			refspecs = append(refspecs, "refs/heads/"+branch+":refs/heads/"+branch)
		}
	}
	return push(ctx, projectRoot, refspecs)
}

// TaskBranches lists the task branches that exist in the repository: the
// ones Ringmaster recorded for its tasks, and any in the older rm/ namespace.
// Named one by one, they are the only branches besides main that are ever
// pushed; a branch of the user's own is never among them.
func TaskBranches(ctx context.Context, projectRoot string) []string {
	out, err := workspace.RunGit(ctx, projectRoot, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil
	}
	recorded := map[string]bool{}
	for _, b := range recordedTaskBranches(projectRoot) {
		recorded[b] = true
	}
	var branches []string
	for _, name := range strings.Fields(out) {
		if name != MainBranch && (recorded[name] || strings.HasPrefix(name, TaskPrefix)) {
			branches = append(branches, name)
		}
	}
	sort.Strings(branches)
	return branches
}

// recordedTaskBranches reads the branch of every task from the project's
// state, where the scheduler records it when a task first starts.
func recordedTaskBranches(projectRoot string) []string {
	state, err := storage.ReadJSON[schemas.ProjectState](filepath.Join(projectRoot, ".ringmaster", "state.json"))
	if err != nil || state == nil {
		return nil
	}
	var branches []string
	for _, ts := range state.TaskStates {
		if ts.Branch != "" && ts.Branch != MainBranch && !strings.HasPrefix(ts.Branch, "-") {
			branches = append(branches, ts.Branch)
		}
	}
	return branches
}

// PushRelease sends main and one release tag to origin. Only that tag is
// pushed, never the user's other tags. Like Push it never forces, so a tag
// that already exists on the remote with other contents is rejected.
func PushRelease(ctx context.Context, projectRoot, tag string) ([]PushResult, error) {
	if tag == "" || strings.HasPrefix(tag, "-") {
		return nil, errors.New("no release tag to push")
	}
	return push(ctx, projectRoot, []string{
		"refs/heads/" + MainBranch + ":refs/heads/" + MainBranch,
		"refs/tags/" + tag + ":refs/tags/" + tag,
	})
}

func push(ctx context.Context, projectRoot string, refspecs []string) ([]PushResult, error) {
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+MainBranch); err != nil {
		return nil, fmt.Errorf("the project has no %q branch to push", MainBranch)
	}
	return gitPush(ctx, projectRoot, refspecs)
}

// gitPush runs one non-interactive git push to origin. Options such as
// --force-with-lease may come before the refspecs.
func gitPush(ctx context.Context, projectRoot string, refspecs []string) ([]PushResult, error) {
	remoteURL := URL(ctx, projectRoot)
	if remoteURL == "" {
		return nil, errors.New("this project has no remote yet; set one first (ringmaster remote set <url>)")
	}

	var opts, specs []string
	for _, r := range refspecs {
		if strings.HasPrefix(r, "--") {
			opts = append(opts, r)
		} else {
			specs = append(specs, r)
		}
	}
	args := append([]string{"-C", projectRoot, "push", "--porcelain"}, opts...)
	args = append(append(args, Name), specs...)

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
		from, to, _ := strings.Cut(fields[1], ":")
		if from == "" || from == "(delete)" {
			// A deletion: "(delete):refs/heads/<branch>"
			from = to
		}
		// A branch is reported by its name, a tag as "tag <name>"
		branch, ok := strings.CutPrefix(from, "refs/heads/")
		if !ok {
			tag, isTag := strings.CutPrefix(from, "refs/tags/")
			if !isTag {
				continue
			}
			branch = "tag " + tag
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

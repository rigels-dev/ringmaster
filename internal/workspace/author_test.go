package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGit gives the test its own global git config (empty unless global
// is given) and clears any author set in the environment.
func isolateGit(t *testing.T, global string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(global), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
}

const adaGlobal = "[user]\n\tname = Ada Lovelace\n\temail = ada@example.com\n"

func newRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if _, err := RunGit(context.Background(), repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	return repo
}

// commitFile makes a commit the way a task attempt does and returns
// "author name <email> | committer name <email>".
func commitFile(t *testing.T, repo string) string {
	t.Helper()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte(t.Name()), 0644); err != nil {
		t.Fatal(err)
	}
	made, err := CommitAll(ctx, repo, "T01: work", []Trailer{{Key: "Ringmaster-Task", Value: "T01"}})
	if err != nil || !made {
		t.Fatalf("CommitAll: made=%v err=%v", made, err)
	}
	return identities(t, repo, "HEAD")
}

func identities(t *testing.T, repo, rev string) string {
	t.Helper()
	out, err := RunGit(context.Background(), repo, "log", "-1", "--format=%an <%ae> | %cn <%ce>", rev)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestCommitIsAuthoredByTheUserAndCommittedByRingmaster(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)

	if got, want := commitFile(t, repo), "Ada Lovelace <ada@example.com> | Ringmaster <ringmaster@local>"; got != want {
		t.Fatalf("identities = %q, want %q", got, want)
	}
}

func TestRepositoryIdentityWinsOverTheGlobalOne(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	ctx := context.Background()
	_, _ = RunGit(ctx, repo, "config", "user.name", "Work Ada")
	_, _ = RunGit(ctx, repo, "config", "user.email", "ada@work.example")

	if got := commitFile(t, repo); !strings.HasPrefix(got, "Work Ada <ada@work.example> | ") {
		t.Fatalf("identities = %q, want the repository's identity as author", got)
	}
}

// With no identity anywhere, the commit still succeeds, attributed to
// Ringmaster, and nothing is written into the user's git config.
func TestCommitWithoutAnyUserIdentity(t *testing.T) {
	isolateGit(t, "")
	repo := newRepo(t)

	if got, want := commitFile(t, repo), "Ringmaster <ringmaster@local> | Ringmaster <ringmaster@local>"; got != want {
		t.Fatalf("identities = %q, want %q", got, want)
	}
	if name := gitConfig(context.Background(), repo, "user.name"); name != "" {
		t.Fatalf("user.name was written into the repository config: %q", name)
	}
}

// Older versions stored Ringmaster's identity in the repository config when
// the user had none. Once the user has a global identity, that one is used.
func TestOldPlaceholderIdentityYieldsToTheGlobalOne(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	ctx := context.Background()
	_, _ = RunGit(ctx, repo, "config", "user.name", "Ringmaster")
	_, _ = RunGit(ctx, repo, "config", "user.email", "ringmaster@local")

	if got := commitFile(t, repo); !strings.HasPrefix(got, "Ada Lovelace <ada@example.com> | ") {
		t.Fatalf("identities = %q, want the global identity as author", got)
	}
}

func TestAuthorFromTheEnvironmentWins(t *testing.T) {
	isolateGit(t, adaGlobal)
	t.Setenv("GIT_AUTHOR_NAME", "Env Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "env@example.com")
	repo := newRepo(t)

	if got := commitFile(t, repo); !strings.HasPrefix(got, "Env Author <env@example.com> | ") {
		t.Fatalf("identities = %q, want the environment's author", got)
	}
}

// Merges into main are written with commit-tree; they follow the same rule.
func TestMergeCommitIsAuthoredByTheUser(t *testing.T) {
	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo)

	tree, err := RunGit(ctx, repo, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	sha, err := RunGit(ctx, repo, "commit-tree", strings.TrimSpace(tree), "-p", "HEAD", "-m", "Merge task T01 into main")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := identities(t, repo, strings.TrimSpace(sha)), "Ada Lovelace <ada@example.com> | Ringmaster <ringmaster@local>"; got != want {
		t.Fatalf("identities = %q, want %q", got, want)
	}
}

func TestCommitAuthorSaysWhetherTheIdentityIsTheUsers(t *testing.T) {
	ctx := context.Background()

	isolateGit(t, adaGlobal)
	repo := newRepo(t)
	if a := CommitAuthor(ctx, repo); !a.IsUser || a.Name != "Ada Lovelace" || a.Email != "ada@example.com" {
		t.Fatalf("with a global identity: %+v", a)
	}

	isolateGit(t, "")
	if a := CommitAuthor(ctx, repo); a.IsUser || a.Name != "Ringmaster" {
		t.Fatalf("with no identity: %+v", a)
	}

	// Only a name, no email: not a usable identity
	isolateGit(t, "[user]\n\tname = Ada Lovelace\n")
	if a := CommitAuthor(ctx, repo); a.IsUser {
		t.Fatalf("with a name but no email: %+v", a)
	}

	// The placeholder older versions left in the repository is not the user's
	isolateGit(t, "")
	_, _ = RunGit(ctx, repo, "config", "user.name", "Ringmaster")
	_, _ = RunGit(ctx, repo, "config", "user.email", "ringmaster@local")
	if a := CommitAuthor(ctx, repo); a.IsUser {
		t.Fatalf("with only the old placeholder: %+v", a)
	}
}

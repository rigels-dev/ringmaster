package release

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"ringmaster/internal/remote"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// project is a repository with one commit on main, owned by Ada.
func project(t *testing.T) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Ada Lovelace\n\temail = ada@example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	// Unset for the test (t.Setenv restores them afterwards), so git falls back to the config above
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "x")
		os.Unsetenv(name)
	}

	dir := filepath.Join(t.TempDir(), "shorty")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, "main.go", "first")
	return dir
}

func commit(t *testing.T, dir, file, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(msg+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", msg)
}

func versions(ss []Suggestion) []string {
	out := []string{}
	for _, s := range ss {
		out = append(out, s.Kind+"="+s.Version)
	}
	return out
}

func TestSuggest(t *testing.T) {
	cases := []struct {
		latest string
		want   []string
	}{
		{"", []string{"first=v0.1.0"}},
		{"v0.1.0", []string{"patch=v0.1.1", "minor=v0.2.0", "major=v1.0.0"}},
		{"1.9.12", []string{"patch=1.9.13", "minor=1.10.0", "major=2.0.0"}},
		// A codename or pre-release suffix stays for a patch and goes for the rest
		{"v0.5.4-electra", []string{"patch=v0.5.5-electra", "minor=v0.6.0", "major=v1.0.0"}},
		{"v1.2", []string{"minor=v1.3", "major=v2.0"}},
		// Not a version number: the scheme is the user's own
		{"release-november", []string{}},
		{"stable", []string{}},
	}
	for _, c := range cases {
		if got := versions(Suggest(c.latest)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Suggest(%q) = %v, want %v", c.latest, got, c.want)
		}
	}
}

func TestInfoFollowsTheProjectsTags(t *testing.T) {
	ctx := context.Background()
	dir := project(t)

	info, err := GetInfo(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Latest != "" || info.Current != "" || versions(info.Suggestions)[0] != "first=v0.1.0" {
		t.Fatalf("a project without tags: %+v", info)
	}

	if err := Tag(ctx, dir, "v0.1.0", ""); err != nil {
		t.Fatal(err)
	}
	info, _ = GetInfo(ctx, dir)
	if info.Latest != "v0.1.0" || info.Current != "v0.1.0" {
		t.Fatalf("right after tagging: %+v", info)
	}

	commit(t, dir, "more.go", "second")
	info, _ = GetInfo(ctx, dir)
	if info.Latest != "v0.1.0" || info.Current != "" {
		t.Fatalf("after new work, main should be untagged with v0.1.0 as the last release: %+v", info)
	}
	if got := versions(info.Suggestions); got[1] != "minor=v0.2.0" {
		t.Fatalf("suggestions = %v", got)
	}
}

func TestTagIsAnnotatedAndMadeByTheUser(t *testing.T) {
	ctx := context.Background()
	dir := project(t)

	if err := Tag(ctx, dir, "v0.1.0", ""); err != nil {
		t.Fatal(err)
	}
	if kind := git(t, dir, "cat-file", "-t", "v0.1.0"); kind != "tag" {
		t.Fatalf("v0.1.0 is a %s, want an annotated tag", kind)
	}
	if got := git(t, dir, "for-each-ref", "--format=%(taggername) %(taggeremail) | %(contents:subject)", "refs/tags/v0.1.0"); got != "Ada Lovelace <ada@example.com> | Release v0.1.0" {
		t.Fatalf("tag = %q", got)
	}
	if git(t, dir, "rev-parse", "v0.1.0^{commit}") != git(t, dir, "rev-parse", "main") {
		t.Fatal("the tag is not on main's tip")
	}
}

// Any scheme is the user's to choose; a release that exists is never moved.
func TestTagAcceptsAnySchemeAndNeverMovesARelease(t *testing.T) {
	ctx := context.Background()
	dir := project(t)

	for _, v := range []string{"v0.1.0", "2026.11", "release-november", "v1.0.0-rc.1", "1.0"} {
		if err := Tag(ctx, dir, v, ""); err != nil {
			t.Errorf("Tag(%q) failed: %v", v, err)
		}
	}

	first := git(t, dir, "rev-parse", "v0.1.0^{commit}")
	commit(t, dir, "more.go", "second")
	if err := Tag(ctx, dir, "v0.1.0", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("tagging an existing version: %v", err)
	}
	if git(t, dir, "rev-parse", "v0.1.0^{commit}") != first {
		t.Fatal("an existing release tag was moved")
	}

	for _, v := range []string{"", "-f", "--force", "has space", "a..b", "a~1", "a:b", "tail.lock", strings.Repeat("x", 101)} {
		if err := Tag(ctx, dir, v, ""); err == nil {
			t.Errorf("Tag(%q) succeeded, want an error", v)
		}
	}
}

func TestArchiveHoldsOnlyCommittedFiles(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	if err := Tag(ctx, dir, "v0.1.0", ""); err != nil {
		t.Fatal(err)
	}
	// Later work, an untracked file and Ringmaster's state must all stay out of the v0.1.0 archive
	commit(t, dir, "later.go", "after the release")
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("untracked"), 0644)
	_ = os.MkdirAll(filepath.Join(dir, ".ringmaster"), 0755)
	_ = os.WriteFile(filepath.Join(dir, ".ringmaster", "state.json"), []byte("{}"), 0644)

	out := t.TempDir()
	path, err := Archive(ctx, dir, "v0.1.0", FormatZip, out)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "shorty-v0.1.0.zip" {
		t.Errorf("archive name = %s", filepath.Base(path))
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	if want := []string{"shorty-v0.1.0/main.go"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("archive holds %v, want %v", names, want)
	}

	// main by default, as tar.gz
	path, err = Archive(ctx, dir, "", FormatTarGz, out)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "shorty-main.tar.gz" {
		t.Errorf("archive name = %s", filepath.Base(path))
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		t.Errorf("the tar.gz archive is missing or empty: %v", err)
	}

	for _, bad := range [][2]string{{"no-such-tag", FormatZip}, {"--output=/tmp/x", FormatZip}, {"main", "rar"}} {
		if _, err := Archive(ctx, dir, bad[0], bad[1], out); err == nil {
			t.Errorf("Archive(ref=%q, format=%q) succeeded, want an error", bad[0], bad[1])
		}
	}
}

// Publishing a release sends main and that one tag, not the user's other tags.
func TestPushReleaseSendsOnlyTheReleaseTag(t *testing.T) {
	ctx := context.Background()
	dir := project(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	git(t, filepath.Dir(bare), "init", "-q", "--bare", "-b", "main", bare)
	if err := remote.SetURL(ctx, dir, "file://"+bare); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "tag", "private-experiment")
	if err := Tag(ctx, dir, "v0.1.0", ""); err != nil {
		t.Fatal(err)
	}

	results, err := remote.PushRelease(ctx, dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range results {
		got[r.Branch] = r.Result
	}
	if got["main"] != remote.ResultNew || got["tag v0.1.0"] != remote.ResultNew {
		t.Fatalf("results = %v", got)
	}
	if tags := git(t, bare, "tag", "--list"); tags != "v0.1.0" {
		t.Fatalf("tags on the remote = %q, want only the release tag", tags)
	}
}

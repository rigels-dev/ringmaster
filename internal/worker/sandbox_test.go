package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathRejectsSiblingWithSharedPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "T01")
	sibling := filepath.Join(base, "T010")
	for _, dir := range []string{root, sibling} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"../T010/secret.txt", "../T010/new.txt", "../T010"} {
		if got, err := ResolvePath(root, rel); err == nil {
			t.Errorf("ResolvePath(%q) = %q, want an error", rel, got)
		}
	}
}

func TestResolvePathRejectsSymlinkEscapes(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "T01")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	// A dangling symlink: writing through it would create the file outside
	if err := os.Symlink(filepath.Join(outside, "created.txt"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"link",                 // existing target outside
		"link/new.txt",         // new file in a directory outside
		"link/newdir/deep.txt", // parent does not exist yet
		"dangling",
	} {
		if got, err := ResolvePath(root, rel); err == nil {
			t.Errorf("ResolvePath(%q) = %q, want an error", rel, got)
		}
	}
}

func TestResolvePathWithSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(realRoot, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "src", "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "alias")
	if err := os.Symlink(realRoot, root); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"src/main.go", "src/new.go", "newdir/deep/file.go", "."} {
		if _, err := ResolvePath(root, rel); err != nil {
			t.Errorf("ResolvePath(%q) failed under a symlinked root: %v", rel, err)
		}
	}
}

func TestSearchDoesNotReadThroughSymlinksOutsideWorktree(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "T01")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{filepath.Join(root, "src"), outside} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(outside, "secret.txt"), "needle outside")
	write(filepath.Join(root, "src", "real.txt"), "needle inside")
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	link(filepath.Join(outside, "secret.txt"), "leak.txt")
	link(filepath.Join(root, "src", "real.txt"), "alias.txt")
	link(outside, "leakdir")

	got := NewToolSet(root, nil).search("needle", "")
	if strings.Contains(got, "outside") {
		t.Errorf("search read a file outside the worktree:\n%s", got)
	}
	for _, want := range []string{"src/real.txt:1: needle inside", "alias.txt:1: needle inside"} {
		if !strings.Contains(got, want) {
			t.Errorf("search result is missing %q:\n%s", want, got)
		}
	}
}

func TestSearchUnderSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(realRoot, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "src", "main.go"), []byte("package needle"), 0644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "alias")
	if err := os.Symlink(realRoot, root); err != nil {
		t.Fatal(err)
	}

	ts := NewToolSet(root, nil)
	for _, sub := range []string{"", "src"} {
		if got := ts.search("needle", sub); !strings.Contains(got, "src/main.go:1: package needle") {
			t.Errorf("search(%q) under a symlinked root = %q", sub, got)
		}
	}
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		m[name] = value
	}
	return m
}

func TestCommandEnvPassesOnlyNamedVariables(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("GOCACHE", "/cache/go")
	t.Setenv("LC_TIME", "C")
	// Credentials, with and without a telltale name
	t.Setenv("SECRET_API_KEY", "sensitive")
	t.Setenv("AUTH_TOKEN", "bearer")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db/app")
	t.Setenv("SSH_AUTH_SOCK", "/run/agent.sock")
	t.Setenv("SOME_APP_SETTING", "x")
	// Caught by a default wildcard, but named like a credential
	t.Setenv("XDG_SESSION_TOKEN", "t")

	env := envMap(CommandEnv(nil))
	for name, want := range map[string]string{"PATH": "/usr/bin", "GOCACHE": "/cache/go", "LC_TIME": "C"} {
		if env[name] != want {
			t.Errorf("%s = %q, want %q", name, env[name], want)
		}
	}
	for _, name := range []string{"SECRET_API_KEY", "AUTH_TOKEN", "DATABASE_URL", "SSH_AUTH_SOCK", "SOME_APP_SETTING", "XDG_SESSION_TOKEN"} {
		if _, ok := env[name]; ok {
			t.Errorf("%s reached the command environment without being listed", name)
		}
	}
}

func TestCommandEnvAddsUserPassEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@db/app")
	t.Setenv("AWS_REGION", "ap-southeast-3")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "listed-on-purpose")
	t.Setenv("AWSOME", "not-a-match")
	t.Setenv("OTHER_SECRET", "s")

	env := envMap(CommandEnv([]string{"DATABASE_URL", " AWS_* ", "*", ""}))
	for _, name := range []string{"DATABASE_URL", "AWS_REGION", "AWS_SECRET_ACCESS_KEY"} {
		if _, ok := env[name]; !ok {
			t.Errorf("%s was listed in pass_env but did not reach the command", name)
		}
	}
	// A bare * is not "everything": it would undo the allowlist by accident
	for _, name := range []string{"AWSOME", "OTHER_SECRET"} {
		if _, ok := env[name]; ok {
			t.Errorf("%s reached the command environment without being listed", name)
		}
	}
}

func TestRunCmdUsesCommandEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@db/app")
	t.Setenv("RM_TEST_VISIBLE", "shown")

	ts := NewToolSet(t.TempDir(), []string{"env"})
	ts.PassEnv = []string{"RM_TEST_VISIBLE"}
	out := ts.runCmd(context.Background(), "env")
	if strings.Contains(out, "DATABASE_URL") {
		t.Errorf("run_cmd leaked an unlisted variable:\n%s", out)
	}
	if !strings.Contains(out, "RM_TEST_VISIBLE=shown") {
		t.Errorf("run_cmd dropped a variable listed in pass_env:\n%s", out)
	}
}

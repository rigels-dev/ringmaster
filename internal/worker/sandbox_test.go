package worker

import (
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

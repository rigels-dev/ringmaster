package worker

import (
	"os"
	"path/filepath"
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

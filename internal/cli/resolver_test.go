package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestResolveProjectDirWalksUpFromSubdirectory(t *testing.T) {
	project := t.TempDir()
	_ = os.MkdirAll(filepath.Join(project, ".ringmaster"), 0755)
	_ = os.WriteFile(filepath.Join(project, ".ringmaster", "BRIEF.md"), []byte("# Brief\n"), 0644)
	sub := filepath.Join(project, "internal", "api")
	_ = os.MkdirAll(sub, 0755)
	chdir(t, sub)

	got, err := ResolveProjectDir()
	if err != nil {
		t.Fatalf("ResolveProjectDir failed: %v", err)
	}
	want, _ := filepath.EvalSymlinks(project)
	if got, _ = filepath.EvalSymlinks(got); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// Outside a project there is no fallback (examples/, active project, cwd):
// project commands must fail instead of guessing.
func TestResolveProjectDirOutsideProjectFails(t *testing.T) {
	dir := t.TempDir()
	// A bare ringmaster.toml is the global fleet config, not a project
	_ = os.WriteFile(filepath.Join(dir, "ringmaster.toml"), []byte(""), 0644)
	_ = os.MkdirAll(filepath.Join(dir, "examples", "app", ".ringmaster"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "examples", "app", ".ringmaster", "BRIEF.md"), []byte("# Brief\n"), 0644)
	chdir(t, dir)

	if got, err := ResolveProjectDir(); err == nil {
		t.Fatalf("expected an error outside a project, got %s", got)
	}
}

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeServer(t *testing.T, dir string) string {
	t.Helper()
	script := filepath.Join(dir, "server", "index.mjs")
	if err := os.MkdirAll(filepath.Dir(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("// web"), 0644); err != nil {
		t.Fatal(err)
	}
	return script
}

// The web UI is found in a packaged install (<prefix>/share/ringmaster/web),
// in a source checkout (web/.output), or wherever RINGMASTER_WEB_DIR points.
func TestFindWebServer(t *testing.T) {
	t.Setenv("RINGMASTER_WEB_DIR", "")

	packaged := t.TempDir()
	want := writeServer(t, filepath.Join(packaged, "share", "ringmaster", "web"))
	if got := findWebServer(packaged); got != want {
		t.Errorf("packaged install: %q, want %q", got, want)
	}

	source := t.TempDir()
	want = writeServer(t, filepath.Join(source, "web", ".output"))
	if got := findWebServer(source); got != want {
		t.Errorf("source checkout: %q, want %q", got, want)
	}

	custom := t.TempDir()
	want = writeServer(t, custom)
	t.Setenv("RINGMASTER_WEB_DIR", custom)
	if got := findWebServer(source); got != want {
		t.Errorf("RINGMASTER_WEB_DIR should win: %q, want %q", got, want)
	}

	t.Setenv("RINGMASTER_WEB_DIR", "")
	if got := findWebServer(t.TempDir()); got != "" {
		t.Errorf("no web UI anywhere should return empty, got %q", got)
	}
}

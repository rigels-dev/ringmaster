package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setGlobalGitConfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
}

func TestGitIdentityWarningWhenTheUserHasNone(t *testing.T) {
	setGlobalGitConfig(t, "")
	warning := gitIdentityWarning(context.Background(), t.TempDir())
	for _, want := range []string{
		"no user.name / user.email",
		`git config --global user.name`,
		`git config --global user.email`,
	} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning is missing %q:\n%s", want, warning)
		}
	}
}

func TestNoGitIdentityWarningWhenTheUserHasOne(t *testing.T) {
	setGlobalGitConfig(t, "[user]\n\tname = Ada Lovelace\n\temail = ada@example.com\n")
	if warning := gitIdentityWarning(context.Background(), t.TempDir()); warning != "" {
		t.Fatalf("unexpected warning:\n%s", warning)
	}
}

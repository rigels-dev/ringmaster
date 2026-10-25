package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitProject(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	projectName := "demo_test"

	// 1. Initial init should succeed
	if err := RunInit(projectName, false); err != nil {
		t.Fatalf("first RunInit failed: %v", err)
	}

	// Verify directories and files
	expectedBrief := filepath.Join("projects", projectName, "BRIEF.md")
	if _, err := os.Stat(expectedBrief); os.IsNotExist(err) {
		t.Errorf("expected BRIEF.md at %s", expectedBrief)
	}

	expectedGit := filepath.Join("projects", projectName, "repo", ".git")
	if _, err := os.Stat(expectedGit); os.IsNotExist(err) {
		t.Errorf("expected .git directory at %s", expectedGit)
	}

	// 2. Second init without force should fail
	if err := RunInit(projectName, false); err == nil {
		t.Fatalf("expected error on duplicate init, got nil")
	}

	// 3. Init with force should succeed
	if err := RunInit(projectName, true); err != nil {
		t.Fatalf("expected force init to succeed, got: %v", err)
	}
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveProjectDir finds the Ringmaster project containing the current
// working directory, walking up through parent directories the way `go`
// looks for go.mod. Project commands only ever operate on that project.
func ResolveProjectDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for dir := cwd; ; dir = filepath.Dir(dir) {
		if isRingmasterProject(dir) {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}

	return "", fmt.Errorf("not a Ringmaster project (or any parent directory): %s\nRun this command inside a project, or create one with 'ringmaster init'", cwd)
}

// isRingmasterProject reports whether dir holds Ringmaster project state.
// ringmaster.toml alone is not enough: it is also the global fleet config.
func isRingmasterProject(dir string) bool {
	for _, marker := range []string{".ringmaster/BRIEF.md", ".ringmaster/plan.json"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

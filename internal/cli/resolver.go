package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveProjectDir smartly finds the target project directory:
//  1. If name is empty or ".", checks if current working directory has a ringmaster project.
//  2. If name is specified:
//     a. Checks relative path directly (e.g. "./my-app")
//     b. Checks "projects/<name>" (optional fallback)
//     c. Checks "examples/<name>"
//
// Returns the resolved directory path or an error if not found.
func ResolveProjectDir(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	if name == "" || name == "." {
		// Check current directory
		if isRingmasterProject(cwd) {
			return cwd, nil
		}
		// If not current directory, check if single project exists in projects/ or examples/
		return cwd, nil
	}

	// 1. Direct path relative to cwd
	directPath := filepath.Join(cwd, name)
	if fi, err := os.Stat(directPath); err == nil && fi.IsDir() {
		return directPath, nil
	}

	// 2. Check examples/<name>
	examplesPath := filepath.Join(cwd, "examples", name)
	if fi, err := os.Stat(examplesPath); err == nil && fi.IsDir() {
		return examplesPath, nil
	}

	// 3. Check projects/<name> (legacy/convenience fallback if directory exists)
	projectsPath := filepath.Join(cwd, "projects", name)
	if fi, err := os.Stat(projectsPath); err == nil && fi.IsDir() {
		return projectsPath, nil
	}

	return "", fmt.Errorf("project directory '%s' not found (checked ./%s, examples/%s)", name, name, name)
}

func isRingmasterProject(dir string) bool {
	for _, marker := range []string{"ringmaster.toml", "BRIEF.md", "plan.json"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

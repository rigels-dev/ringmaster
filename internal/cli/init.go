package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init <project_name>",
	Short: "Initialize a new project directory, workspace and git repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		return RunInit(projectName, forceInit)
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing project directory if present")
}

func RunInit(name string, force bool) error {
	projectDir := filepath.Join("projects", name)

	if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
		if !force {
			return fmt.Errorf("project '%s' already exists at %s (use --force to overwrite)", name, projectDir)
		}
	}

	dirsToCreate := []string{
		projectDir,
		filepath.Join(projectDir, "tasks"),
		filepath.Join(projectDir, "outputs"),
		filepath.Join(projectDir, ".ringmaster", "logs"),
		filepath.Join(projectDir, "repo"),
	}

	for _, d := range dirsToCreate {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Create BRIEF.md template
	briefPath := filepath.Join(projectDir, "BRIEF.md")
	briefContent := fmt.Sprintf(`# BRIEF: %s

## Objective
Tulis deskripsi tujuan dan ruang lingkup project ini di sini.

## Requirements
- Requirement 1
- Requirement 2

## Acceptance Criteria
- [ ] Kriteria keberhasilan 1
- [ ] Kriteria keberhasilan 2
`, name)

	if err := os.WriteFile(briefPath, []byte(briefContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", briefPath, err)
	}

	// Initialize git repository in repo/ with branch 'main'
	repoDir := filepath.Join(projectDir, "repo")
	gitDir := filepath.Join(repoDir, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = repoDir
		if output, err := cmd.CombinedOutput(); err != nil {
			// Fallback for older git that might not support -b flag
			cmdOld := exec.Command("git", "init")
			cmdOld.Dir = repoDir
			if _, errOld := cmdOld.CombinedOutput(); errOld != nil {
				return fmt.Errorf("failed to initialize git repository: %s (err: %w)", string(output), errOld)
			}
			branchCmd := exec.Command("git", "checkout", "-b", "main")
			branchCmd.Dir = repoDir
			_ = branchCmd.Run()
		}

		// Create initial README.md and commit so branch main exists
		initialReadme := filepath.Join(repoDir, "README.md")
		_ = os.WriteFile(initialReadme, []byte(fmt.Sprintf("# %s\nRepository managed by Ringmaster.\n", name)), 0644)

		addCmd := exec.Command("git", "add", "README.md")
		addCmd.Dir = repoDir
		_ = addCmd.Run()

		commitCmd := exec.Command("git", "commit", "-m", "chore: initial commit by ringmaster")
		commitCmd.Dir = repoDir
		// set dummy author if git not configured
		commitCmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Ringmaster",
			"GIT_AUTHOR_EMAIL=ringmaster@openclaw.local",
			"GIT_COMMITTER_NAME=Ringmaster",
			"GIT_COMMITTER_EMAIL=ringmaster@openclaw.local",
		)
		_ = commitCmd.Run()
	}

	fmt.Printf("✓ Initialized Ringmaster project '%s' in %s\n", name, projectDir)
	fmt.Printf("  - Brief: %s\n", briefPath)
	fmt.Printf("  - Repo:  %s (git initialized on branch 'main')\n", repoDir)
	return nil
}

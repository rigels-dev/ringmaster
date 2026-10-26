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
	Use:   "init [project_name]",
	Short: "Initialize a new project directory or initialize current directory as a Ringmaster project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := ""
		if len(args) > 0 {
			projectName = args[0]
		}
		return RunInit(projectName, forceInit)
	},
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing project files if present")
}

func RunInit(name string, force bool) error {
	var projectDir string
	var repoDir string
	isCurrentDir := false

	if name == "" || name == "." {
		// Init existing current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		projectDir = cwd
		name = filepath.Base(cwd)
		repoDir = projectDir
		isCurrentDir = true
	} else {
		// Init target directory directly
		projectDir = name
		repoDir = filepath.Join(projectDir, "repo")

		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			if !force {
				return fmt.Errorf("directory '%s' already exists (use --force to overwrite)", projectDir)
			}
		}
	}

	dirsToCreate := []string{
		projectDir,
		filepath.Join(projectDir, "tasks"),
		filepath.Join(projectDir, "outputs"),
		filepath.Join(projectDir, ".ringmaster", "logs"),
		repoDir,
	}

	for _, d := range dirsToCreate {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// 1. Create ringmaster project manifest file
	manifestPath := filepath.Join(projectDir, "ringmaster.toml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) || force {
		manifestContent := fmt.Sprintf(`# Ringmaster Project Manifest
name = %q
version = "0.1.0"
brief = "BRIEF.md"
plan = "plan.json"
created_at = %q
`, name, filepath.Base(projectDir))
		_ = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	}

	// 2. Create BRIEF.md template if not present
	briefPath := filepath.Join(projectDir, "BRIEF.md")
	if _, err := os.Stat(briefPath); os.IsNotExist(err) || force {
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
	}

	// 3. Initialize git repository if not already a git repository
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = repoDir
		if output, err := cmd.CombinedOutput(); err != nil {
			cmdOld := exec.Command("git", "init")
			cmdOld.Dir = repoDir
			if _, errOld := cmdOld.CombinedOutput(); errOld != nil {
				return fmt.Errorf("failed to initialize git repository: %s (err: %w)", string(output), errOld)
			}
			branchCmd := exec.Command("git", "checkout", "-b", "main")
			branchCmd.Dir = repoDir
			_ = branchCmd.Run()
		}

		initialReadme := filepath.Join(repoDir, "README.md")
		if _, err := os.Stat(initialReadme); os.IsNotExist(err) {
			_ = os.WriteFile(initialReadme, []byte(fmt.Sprintf("# %s\nRepository managed by Ringmaster.\n", name)), 0644)
		}

		addCmd := exec.Command("git", "add", ".")
		addCmd.Dir = repoDir
		_ = addCmd.Run()

		commitCmd := exec.Command("git", "commit", "-m", "chore: initial commit by ringmaster")
		commitCmd.Dir = repoDir
		commitCmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Ringmaster",
			"GIT_AUTHOR_EMAIL=ringmaster@openclaw.local",
			"GIT_COMMITTER_NAME=Ringmaster",
			"GIT_COMMITTER_EMAIL=ringmaster@openclaw.local",
		)
		_ = commitCmd.Run()
	}

	if isCurrentDir {
		fmt.Printf("✓ Initialized current directory as Ringmaster project '%s'\n", name)
	} else {
		fmt.Printf("✓ Initialized Ringmaster project '%s' in %s\n", name, projectDir)
	}
	fmt.Printf("  - Manifest: %s\n", manifestPath)
	fmt.Printf("  - Brief:    %s\n", briefPath)
	fmt.Printf("  - Repo:     %s\n", repoDir)
	return nil
}

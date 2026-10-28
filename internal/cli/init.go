package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
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
		cwd, _ := os.Getwd()
		projectDir = filepath.Join(cwd, name)
		repoDir = filepath.Join(projectDir, "repo")

		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			if !force {
				return fmt.Errorf("directory '%s' already exists (use --force to overwrite)", projectDir)
			}
		}
	}

	// Record this as the latest active project in user's global ringmaster profile
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		activeFile := filepath.Join(homeDir, ".ringmaster", "active_project")
		_ = os.MkdirAll(filepath.Dir(activeFile), 0755)
		_ = os.WriteFile(activeFile, []byte(projectDir), 0644)
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
Write a concise description of the project goal and scope here.

## Requirements
- Requirement 1
- Requirement 2

## Acceptance Criteria
- [ ] Criteria 1
- [ ] Criteria 2
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

	// Stylized Banner (Metro / Steampunk Circus theme)
	bannerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DEAA3B"))

	subBannerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#29F2EC"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8493A0"))

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EFE4CE"))

	cmdHighlight := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DEAA3B")).
		Padding(0, 1)

	banner := `
  ____  ___ _   _  ____ __  __    _    ____ _____ _____ ____  
 |  _ \|_ _| \ | |/ ___|  \/  |  / \  / ___|_   _| ____|  _ \ 
 | |_) || ||  \| | |  _| |\/| | / _ \ \___ \ | | |  _| | |_) |
 |  _ < | || |\  | |_| | |  | |/ ___ \ ___) || | | |___|  _ < 
 |_| \_\___|_| \_|\____|_|  |_/_/   \_\____/ |_| |_____|_| \_\`

	fmt.Println(bannerStyle.Render(banner))
	fmt.Println(subBannerStyle.Render("       🎪 LOCAL AI MULTI-AGENT CONTROL PLANE 🎪\n"))

	if isCurrentDir {
		fmt.Printf("✓ %s '%s'\n", labelStyle.Render("Initialized current directory as Ringmaster project"), cmdHighlight.Render(name))
	} else {
		fmt.Printf("✓ %s '%s' in %s\n", labelStyle.Render("Initialized Ringmaster project"), cmdHighlight.Render(name), pathStyle.Render(projectDir))
	}
	fmt.Printf("  • Manifest: %s\n", pathStyle.Render(manifestPath))
	fmt.Printf("  • Brief:    %s\n", pathStyle.Render(briefPath))
	fmt.Printf("  • Repo:     %s\n\n", pathStyle.Render(repoDir))

	// Next Steps Instructions Box
	var nextSteps string
	if isCurrentDir {
		nextSteps = fmt.Sprintf("Next steps:\n  1. Run %s to launch control plane\n  2. Discuss or plan your project interactively with Ringmaster\n     (or edit %s directly if you already have a prompt)",
			cmdHighlight.Render("ringmaster"),
			cmdHighlight.Render("BRIEF.md"))
	} else {
		nextSteps = fmt.Sprintf("Next steps:\n  1. %s\n  2. Run %s to launch control plane\n  3. Discuss or plan your project interactively with Ringmaster\n     (or edit %s directly if you already have a prompt)",
			cmdHighlight.Render(fmt.Sprintf("cd %s", name)),
			cmdHighlight.Render("ringmaster"),
			cmdHighlight.Render("BRIEF.md"))
	}
	fmt.Println(boxStyle.Render(nextSteps))
	fmt.Println()
	return nil
}

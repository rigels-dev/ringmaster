package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ringmaster/internal/workspace"

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
	ctx := context.Background()
	var projectDir string
	isCurrentDir := false

	if name == "" || name == "." {
		// Init existing current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		projectDir = cwd
		name = filepath.Base(cwd)
		isCurrentDir = true

		// Refuse a subdirectory of an existing repo before writing anything:
		// tasks would otherwise branch and merge into that outer repository.
		if !workspace.IsRepoRoot(ctx, projectDir) {
			if outer, err := workspace.RunGit(ctx, projectDir, "rev-parse", "--show-toplevel"); err == nil {
				return fmt.Errorf("%s is inside git repository %s; run 'ringmaster init' at the repository root, or 'ringmaster init <name>' to create a separate project", projectDir, strings.TrimSpace(outer))
			}
		}
	} else {
		// Init target directory directly
		cwd, _ := os.Getwd()
		projectDir = filepath.Join(cwd, name)

		if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
			if !force {
				return fmt.Errorf("directory '%s' already exists (use --force to overwrite)", projectDir)
			}
		}
	}

	// The project directory is the git repository itself; Ringmaster only adds
	// ringmaster.toml and the .ringmaster/ state directory to it.
	stateDir := filepath.Join(projectDir, workspace.StateDirName)
	dirsToCreate := []string{
		projectDir,
		filepath.Join(stateDir, "tasks"),
		filepath.Join(stateDir, "logs"),
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
brief = ".ringmaster/BRIEF.md"
plan = ".ringmaster/plan.json"
created_at = %q

# Providers and agents are configured globally ('ringmaster config' or the
# web UI); this project only assigns them to tasks in its plan.
#
# Optional overrides for this project:
# [sandbox]
# allow = ["go test", "go vet", "go build"]   # shell commands workers may run
# acceptance_timeout = 600                     # seconds per acceptance command (default 300)
# pass_env = ["DATABASE_URL", "AWS_*"]        # extra environment variables commands may see
#
# [budget]
# max_tokens_per_project = 2000000
# max_tokens_per_task = 300000
#
# [web]
# port = 12053                                 # web UI port for this project
#
# [skills]
# disabled = ["deploy"]                        # installed skills this project's workers do not get
`, name, time.Now().UTC().Format(time.RFC3339))
		_ = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	}

	// 2. Create BRIEF.md template if not present
	briefPath := filepath.Join(stateDir, "BRIEF.md")
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

	// 3. Make sure the project is a git repository with a main commit to branch
	// task worktrees from. Existing files are never staged or committed.
	// The project must be the root of its own repository (checked for the
	// current-dir case above); a new project nested in another checkout gets
	// its own repo so tasks never branch or merge into the outer one.
	if !workspace.IsRepoRoot(ctx, projectDir) {
		if _, err := workspace.RunGit(ctx, projectDir, "init", "-b", "main"); err != nil {
			if _, errOld := workspace.RunGit(ctx, projectDir, "init"); errOld != nil {
				return fmt.Errorf("failed to initialize git repository: %w", errOld)
			}
			_, _ = workspace.RunGit(ctx, projectDir, "symbolic-ref", "HEAD", "refs/heads/main")
		}
	}
	if _, err := workspace.RunGit(ctx, projectDir, "rev-parse", "--verify", "HEAD"); err != nil {
		if _, err := workspace.RunGit(ctx, projectDir, "commit", "--allow-empty", "-m", "chore: initial commit by ringmaster"); err != nil {
			return fmt.Errorf("failed to create initial commit: %w", err)
		}
	}
	if err := workspace.EnsureStateExcluded(ctx, projectDir); err != nil {
		return fmt.Errorf("failed to exclude %s from git: %w", workspace.StateDirName, err)
	}
	branch, _ := workspace.RunGit(ctx, projectDir, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)

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
	fmt.Printf("  • State:    %s %s\n", pathStyle.Render(stateDir), labelStyle.Render("(git-excluded)"))
	if branch != "main" {
		fmt.Printf("  ⚠ %s\n", labelStyle.Render(fmt.Sprintf("Current branch is '%s'; task branches are merged into 'main'", branch)))
	}
	fmt.Println()

	// Next Steps Instructions Box
	var nextSteps string
	if isCurrentDir {
		nextSteps = fmt.Sprintf("Next steps:\n  1. Run %s to launch control plane\n  2. Discuss or plan your project interactively with Ringmaster\n     (or edit %s directly if you already have a prompt)",
			cmdHighlight.Render("ringmaster"),
			cmdHighlight.Render(".ringmaster/BRIEF.md"))
	} else {
		nextSteps = fmt.Sprintf("Next steps:\n  1. %s\n  2. Run %s to launch control plane\n  3. Discuss or plan your project interactively with Ringmaster\n     (or edit %s directly if you already have a prompt)",
			cmdHighlight.Render(fmt.Sprintf("cd %s", name)),
			cmdHighlight.Render("ringmaster"),
			cmdHighlight.Render(".ringmaster/BRIEF.md"))
	}
	fmt.Println(boxStyle.Render(nextSteps))
	fmt.Println()
	return nil
}

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"ringmaster/internal/scheduler"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

var runMaxParallel int

var runCmd = &cobra.Command{
	Use:   "run [project_name]",
	Short: "Execute approved DAG tasks across parallel worker worktrees",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded. Please create ringmaster.toml or pass --config")
		}

		projectName := ""
		if len(args) > 0 {
			projectName = args[0]
		}

		projectDir, err := ResolveProjectDir(projectName)
		if err != nil {
			return err
		}

		engine, err := scheduler.NewEngine(projectDir, cfg, runMaxParallel)
		if err != nil {
			return err
		}

		// Advertise the live run so the web UI can detect it across processes
		pidFile := filepath.Join(projectDir, ".ringmaster", "run.pid")
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err == nil {
			defer os.Remove(pidFile)
		}

		// Graceful shutdown on SIGINT / SIGTERM
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("🎪 Ringmaster starting orchestration for '%s' (max_parallel=%d)...\n", projectName, runMaxParallel)
		err = engine.Run(ctx)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("\n⚠ Orchestration interrupted gracefully. State saved (can be resumed).")
				return nil
			}
			return err
		}

		fmt.Println("✓ Orchestration completed!")
		return nil
	},
}

var answerCmd = &cobra.Command{
	Use:   "answer <task_id>",
	Short: "Provide human answer to unblock a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := strings.ToUpper(args[0])
		answerText, _ := cmd.Flags().GetString("text")
		if strings.TrimSpace(answerText) == "" {
			return fmt.Errorf("--text cannot be empty")
		}

		projectName, _ := cmd.Flags().GetString("project")
		projectDir, err := ResolveProjectDir(projectName)
		if err != nil {
			return err
		}

		// 1. Find and update task markdown file
		tasksDir := filepath.Join(projectDir, "tasks")
		entries, err := os.ReadDir(tasksDir)
		if err != nil {
			return fmt.Errorf("failed reading %s: %w", tasksDir, err)
		}

		var targetFile string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), taskID) && strings.HasSuffix(e.Name(), ".md") {
				targetFile = filepath.Join(tasksDir, e.Name())
				break
			}
		}

		if targetFile == "" {
			return fmt.Errorf("task file for '%s' not found", taskID)
		}

		contentBytes, _ := os.ReadFile(targetFile)
		content := string(contentBytes)
		if !strings.Contains(content, "## Human Answers") {
			content += fmt.Sprintf("\n\n## Human Answers\n%s\n", answerText)
		} else {
			content += fmt.Sprintf("\n%s\n", answerText)
		}
		_ = os.WriteFile(targetFile, []byte(content), 0644)

		// 2. Reset task status in state.json from BLOCKED back to PENDING
		statePath := filepath.Join(projectDir, ".ringmaster", "state.json")
		state, err := storage.ReadJSON[schemas.ProjectState](statePath)
		if err == nil && state != nil {
			if ts, ok := state.TaskStates[taskID]; ok {
				ts.Status = schemas.TaskStatePending
				ts.Error = ""
				state.TaskStates[taskID] = ts
				_ = storage.WriteJSONAtomic(statePath, state)
			}
		}

		fmt.Printf("✓ Answer recorded for %s. Task reset to PENDING.\n", taskID)
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume [project_name]",
	Short: "Resume paused or halted project orchestration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCmd.RunE(cmd, args)
	},
}

func init() {
	runCmd.Flags().IntVarP(&runMaxParallel, "parallel", "p", 4, "Maximum parallel worker worktrees")
	answerCmd.Flags().StringP("text", "t", "", "Answer text for the blocked task")
	answerCmd.Flags().String("project", "demo", "Project name")
	_ = answerCmd.MarkFlagRequired("text")

	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(answerCmd)
	RootCmd.AddCommand(resumeCmd)
}

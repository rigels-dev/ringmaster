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
)

var runMaxParallel int

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute approved DAG tasks across parallel worker worktrees",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		cfg, err := projectConfig(projectDir)
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

		// Persist this run's output to .ringmaster/logs/run.jsonl (kept across runs)
		footer := "Run ended"
		if stopLog, err := startRunLog(projectDir, fmt.Sprintf("Run started · pid %d · max_parallel %d", os.Getpid(), runMaxParallel)); err == nil {
			defer func() { stopLog(footer) }()
		} else {
			fmt.Fprintf(os.Stderr, "Warning: run output is not being saved: %v\n", err)
		}

		// Graceful shutdown on SIGINT / SIGTERM
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("🎪 Ringmaster starting orchestration for '%s' (max_parallel=%d)...\n", filepath.Base(projectDir), runMaxParallel)
		// Said here, inside the run log, so the web UI's Logs view shows it too
		if warning := gitIdentityWarning(ctx, projectDir); warning != "" {
			fmt.Print(warning)
		}
		err = engine.Run(ctx)
		if err != nil {
			if ctx.Err() != nil {
				footer = "Run interrupted (state saved, can be resumed)"
				fmt.Println("\n⚠ Orchestration interrupted gracefully. State saved (can be resumed).")
				return nil
			}
			footer = fmt.Sprintf("Run failed: %v", err)
			return err
		}

		final := engine.GetState()
		footer = fmt.Sprintf("Run ended · project %s", final.Status)
		if final.Status == "COMPLETED" {
			fmt.Println("✓ Orchestration completed!")
			return nil
		}
		fmt.Printf("⚠ Orchestration halted (status %s). Task outcomes:\n", final.Status)
		for _, id := range engine.TaskIDs() {
			status := final.TaskStates[id].Status
			if status == "" {
				status = schemas.TaskStatePending
			}
			fmt.Printf("   %s %s\n", id, status)
		}
		return nil
	},
}

var answerCmd = &cobra.Command{
	Use:   "answer <task_id>",
	Short: "Answer a task that is waiting on you (AWAITING_INPUT, BLOCKED, NEEDS_REVIEW or PAUSED)",
	Long: `Queue an answer for a task. A running orchestration applies it on its next tick;
otherwise it is applied when you run 'ringmaster resume'.

Examples:
  ringmaster answer T06 --reply r1=approve --allow "docker build -t app ."
  ringmaster answer T06 --reply r2=distroless --reply r3=API_KEY
  ringmaster answer T02 --text "Drop the gofmt check; go vet covers it."`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ans := schemas.HumanAnswer{TaskID: strings.ToUpper(args[0])}
		ans.Text, _ = cmd.Flags().GetString("text")
		ans.Allow, _ = cmd.Flags().GetStringArray("allow")
		ans.Scope, _ = cmd.Flags().GetString("scope")

		replies, _ := cmd.Flags().GetStringArray("reply")
		for _, r := range replies {
			id, value, ok := strings.Cut(r, "=")
			if !ok || strings.TrimSpace(id) == "" {
				return fmt.Errorf("--reply must look like <request_id>=<answer>, got %q", r)
			}
			if ans.Replies == nil {
				ans.Replies = map[string]string{}
			}
			ans.Replies[strings.TrimSpace(id)] = strings.TrimSpace(value)
		}
		if strings.TrimSpace(ans.Text) == "" && len(ans.Replies) == 0 && len(ans.Allow) == 0 {
			return fmt.Errorf("nothing to answer: pass --text, --reply or --allow")
		}

		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		if err := scheduler.QueueAnswer(projectDir, ans); err != nil {
			return err
		}

		fmt.Printf("✓ Answer queued for %s. A running orchestration picks it up shortly; otherwise run 'ringmaster resume'.\n", ans.TaskID)
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume paused or halted project orchestration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCmd.RunE(cmd, args)
	},
}

func init() {
	runCmd.Flags().IntVarP(&runMaxParallel, "parallel", "p", 4, "Maximum parallel worker worktrees")
	answerCmd.Flags().StringP("text", "t", "", "Free-form guidance for the task")
	answerCmd.Flags().StringArray("reply", nil, "Answer to one request, as <request_id>=<answer> (repeatable)")
	answerCmd.Flags().StringArray("allow", nil, "Approve a command beyond the sandbox allow list (repeatable)")
	answerCmd.Flags().String("scope", "task", "Approval scope for --allow: task or project")

	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(answerCmd)
	RootCmd.AddCommand(resumeCmd)
}

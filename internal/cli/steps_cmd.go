package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/steps"
)

var stepsJSON bool

var stepsCmd = &cobra.Command{
	Use:   "steps",
	Short: "List the finished steps of the project: each plan whose tasks are all done",
	Long: `Every plan whose tasks are all done and merged becomes a step of the
project: Step 1, Step 2, and so on. A step ends at the commit on main where
its last task was merged.

A step can be released (ringmaster release <version> --step <n>), or
finished without a release, which stops Ringmaster from offering one:

  ringmaster steps                       # every step, and its release
  ringmaster steps name 1 "First version"
  ringmaster steps close 2               # finished without a release
  ringmaster steps reopen 2              # offer a release again`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		list, err := steps.List(context.Background(), projectDir)
		if err != nil {
			return err
		}
		if stepsJSON {
			if list == nil {
				list = []steps.Step{}
			}
			return printJSON(map[string]any{"steps": list})
		}
		fmt.Print(buildStepsView(list))
		return nil
	},
}

func stepNumber(arg string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%q is not a step number", arg)
	}
	return n, nil
}

var stepsNameCmd = &cobra.Command{
	Use:   "name <step> <name>",
	Short: "Name a step; an empty name goes back to 'Step <n>'",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		n, err := stepNumber(args[0])
		if err != nil {
			return err
		}
		if err := steps.Rename(context.Background(), projectDir, n, strings.Join(args[1:], " ")); err != nil {
			return err
		}
		fmt.Printf("✓ Step %d renamed\n", n)
		return nil
	},
}

var stepsCloseCmd = &cobra.Command{
	Use:   "close <step>",
	Short: "Mark a step as finished without a release",
	Args:  cobra.ExactArgs(1),
	RunE:  setStepClosed(true),
}

var stepsReopenCmd = &cobra.Command{
	Use:   "reopen <step>",
	Short: "Undo 'close': offer a release for the step again",
	Args:  cobra.ExactArgs(1),
	RunE:  setStepClosed(false),
}

func setStepClosed(closed bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		n, err := stepNumber(args[0])
		if err != nil {
			return err
		}
		if err := steps.SetClosed(context.Background(), projectDir, n, closed); err != nil {
			return err
		}
		if closed {
			fmt.Printf("✓ Step %d is finished without a release\n", n)
		} else {
			fmt.Printf("✓ Step %d can be released again\n", n)
		}
		return nil
	}
}

func buildStepsView(list []steps.Step) string {
	if len(list) == 0 {
		return "No finished step yet. A plan becomes a step once all its tasks are done.\n"
	}
	var sb strings.Builder
	for i := len(list) - 1; i >= 0; i-- {
		s := list[i]
		state := "not released"
		switch {
		case len(s.Releases) > 0:
			state = "released as " + strings.Join(s.Releases, ", ")
		case s.Closed:
			state = "finished without a release"
		}
		title := s.Title()
		if s.Name != "" {
			title = fmt.Sprintf("Step %d: %s", s.Number, s.Name)
		}
		fmt.Fprintf(&sb, "%s\n  %s · %d task(s) %s · ends at %.7s · %s\n",
			title, s.FinishedAt.Local().Format("2006-01-02 15:04"), len(s.Tasks), taskRange(s.Tasks), s.Commit, state)
	}
	return sb.String()
}

func taskRange(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return ids[0] + "–" + ids[len(ids)-1]
}

func init() {
	stepsCmd.Flags().BoolVar(&stepsJSON, "json", false, "Print the steps as JSON")
	stepsCmd.AddCommand(stepsNameCmd, stepsCloseCmd, stepsReopenCmd)
	RootCmd.AddCommand(stepsCmd)
}

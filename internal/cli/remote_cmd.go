package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/remote"
)

var (
	remoteJSON   bool
	pushJSON     bool
	pushMainOnly bool
	pruneJSON    bool
	pruneRemote  bool
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Show the project's git remote and which branches still need pushing",
	Long: `Ringmaster merges tasks into your local main. To publish the result,
give the project a remote once and push:

  ringmaster remote set https://github.com/you/project.git
  ringmaster push

The branch states shown here come from the last push or fetch; nothing is
contacted.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		st, err := remote.GetStatus(context.Background(), projectDir)
		if err != nil {
			return err
		}
		if remoteJSON {
			return printJSON(st)
		}
		fmt.Print(buildRemoteView(st))
		return nil
	},
}

var remoteSetCmd = &cobra.Command{
	Use:   "set <url>",
	Short: "Point the project's origin at a repository (adds the remote if there is none)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		if err := remote.SetURL(context.Background(), projectDir, args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ origin is now %s\n", args[0])
		return nil
	},
}

// pushReport is what `ringmaster push --json` prints, on success and on failure.
type pushReport struct {
	OK      bool                `json:"ok"`
	URL     string              `json:"url"`
	Results []remote.PushResult `json:"results"`
	Error   string              `json:"error,omitempty"`
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push main and the task branches (rm/Txx) to the project's remote",
	Long: `Pushes main and every task branch, merged or not, to origin. Other
branches are left alone. Nothing is ever forced: if the remote has commits
this project does not, that branch is rejected and reported.

It uses your own git credentials (an SSH key or a credential helper) and
does not prompt, so a missing login fails at once instead of hanging.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		ctx := context.Background()
		results, pushErr := remote.Push(ctx, projectDir, pushMainOnly)

		if pushJSON {
			report := pushReport{OK: pushErr == nil, URL: remote.URL(ctx, projectDir), Results: results}
			if report.Results == nil {
				report.Results = []remote.PushResult{}
			}
			if pushErr != nil {
				report.Error = pushErr.Error()
			}
			if err := printJSON(report); err != nil {
				return err
			}
			return pushErr
		}

		fmt.Print(buildPushView(results))
		return pushErr
	},
}

// pruneReport is what `ringmaster prune --json` prints.
type pruneReport struct {
	OK      bool                 `json:"ok"`
	Results []remote.PruneResult `json:"results"`
	Error   string               `json:"error,omitempty"`
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete the task branches that are merged into main",
	Long: `Deletes every task branch whose commits are all in main. The work stays
in main's history, merge commits included; only the branch names go.
main, unmerged task branches, branches checked out in a worktree and your
own branches are never touched.

With --remote, the same task branches are deleted on origin too, as long as
the remote still has them where this project last saw them. A branch that
was moved on the remote since is kept.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		results, pruneErr := remote.Prune(context.Background(), projectDir, pruneRemote)
		if pruneJSON {
			report := pruneReport{OK: pruneErr == nil, Results: results}
			if report.Results == nil {
				report.Results = []remote.PruneResult{}
			}
			if pruneErr != nil {
				report.Error = pruneErr.Error()
			}
			if err := printJSON(report); err != nil {
				return err
			}
			return pruneErr
		}
		fmt.Print(buildPruneView(results))
		return pruneErr
	},
}

func buildPruneView(results []remote.PruneResult) string {
	if len(results) == 0 {
		return "No merged task branches to delete.\n"
	}
	var sb strings.Builder
	labels := map[string]string{
		remote.PruneDeleted:       "deleted",
		remote.PruneRemoteDeleted: "deleted on " + remote.Name,
		remote.PruneKept:          "kept",
	}
	for _, r := range results {
		mark := "✓"
		if r.Result == remote.PruneKept {
			mark = "·"
		}
		line := fmt.Sprintf("%s %s  %s", mark, r.Branch, labels[r.Result])
		if r.Detail != "" {
			line += ": " + r.Detail
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func buildRemoteView(st remote.Status) string {
	var sb strings.Builder
	if st.URL == "" {
		sb.WriteString("No remote yet. Set one with: ringmaster remote set <url>\n")
	} else {
		fmt.Fprintf(&sb, "%s  %s\n", st.Name, st.URL)
	}
	if st.Author.IsUser {
		fmt.Fprintf(&sb, "author  %s <%s>\n", st.Author.Name, st.Author.Email)
	} else {
		sb.WriteString("author  Ringmaster (git has no user.name / user.email set; see 'git config --global user.name')\n")
	}
	if len(st.Branches) == 0 {
		return sb.String()
	}
	sb.WriteString("\n")
	labels := map[string]string{
		remote.StateSynced:   "pushed",
		remote.StateAhead:    "has new commits",
		remote.StateUnpushed: "not pushed",
	}
	width := 0
	for _, b := range st.Branches {
		if len(b.Name) > width {
			width = len(b.Name)
		}
	}
	for _, b := range st.Branches {
		label := labels[b.State]
		if b.Merged {
			label += ", merged into main"
		}
		fmt.Fprintf(&sb, "  %-*s  %s\n", width, b.Name, label)
	}
	if st.URL != "" && st.Pending() > 0 {
		fmt.Fprintf(&sb, "\n%d branch(es) to push: ringmaster push\n", st.Pending())
	}
	if st.Merged() > 0 {
		fmt.Fprintf(&sb, "%d merged task branch(es) can be deleted: ringmaster prune\n", st.Merged())
	}
	return sb.String()
}

func buildPushView(results []remote.PushResult) string {
	var sb strings.Builder
	labels := map[string]string{
		remote.ResultNew:      "new on the remote",
		remote.ResultUpdated:  "updated",
		remote.ResultSame:     "already up to date",
		remote.ResultRejected: "rejected",
	}
	width := 0
	for _, r := range results {
		if len(r.Branch) > width {
			width = len(r.Branch)
		}
	}
	for _, r := range results {
		mark := "✓"
		if r.Result == remote.ResultRejected {
			mark = "✗"
		}
		fmt.Fprintf(&sb, "%s %-*s  %s\n", mark, width, r.Branch, labels[r.Result])
	}
	return sb.String()
}

func init() {
	remoteCmd.Flags().BoolVar(&remoteJSON, "json", false, "Print the remote and branch states as JSON")
	remoteCmd.AddCommand(remoteSetCmd)
	pushCmd.Flags().BoolVar(&pushMainOnly, "main-only", false, "Push only main, not the task branches")
	pushCmd.Flags().BoolVar(&pushJSON, "json", false, "Print the result of each branch as JSON")
	pruneCmd.Flags().BoolVar(&pruneRemote, "remote", false, "Also delete the merged task branches on origin")
	pruneCmd.Flags().BoolVar(&pruneJSON, "json", false, "Print what happened to each branch as JSON")
	RootCmd.AddCommand(remoteCmd, pushCmd, pruneCmd)
}

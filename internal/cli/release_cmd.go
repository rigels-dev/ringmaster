package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/planner"
	"ringmaster/internal/release"
	"ringmaster/internal/remote"
	"ringmaster/internal/schemas"
	"ringmaster/internal/steps"
	"ringmaster/internal/storage"
)

var (
	releaseJSON   bool
	releasePush   bool
	releaseForce  bool
	releaseStep   int
	archiveFormat string
	archiveOutDir string
	archiveJSON   bool
)

// releaseReport is what `ringmaster release --json` prints.
type releaseReport struct {
	release.Info
	// Unfinished lists the tasks of the current plan that are not done; a
	// release is normally cut once it is empty.
	Unfinished []string `json:"unfinished"`
	// Tagged and Pushed are set after `ringmaster release <version>`.
	Tagged string              `json:"tagged,omitempty"`
	Pushed []remote.PushResult `json:"pushed,omitempty"`
	Error  string              `json:"error,omitempty"`
}

var releaseCmd = &cobra.Command{
	Use:   "release [version]",
	Short: "Tag the finished project as a release; without a version, suggest the next one",
	Long: `Marks main as a release with an annotated git tag.

The version is yours to choose, in any scheme. Run it without one to see the
last release and what could follow:

  ringmaster release              # the last tag, and suggestions
  ringmaster release v0.2.0       # tag main as v0.2.0
  ringmaster release v0.2.0 --push   # and push main and that tag to origin

A version that already exists is never moved or replaced. While tasks of the
current plan are unfinished the release is refused; --force overrides that.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		ctx := context.Background()

		info, err := release.GetInfo(ctx, projectDir)
		if err != nil {
			return err
		}
		info.Recommended = recommendedKind(projectDir, info.Suggestions)
		report := releaseReport{Info: info, Unfinished: unfinishedTasks(projectDir)}

		if len(args) == 0 {
			if releaseJSON {
				return printJSON(report)
			}
			fmt.Print(buildReleaseView(report))
			return nil
		}

		version := args[0]
		fail := func(err error) error {
			if releaseJSON {
				report.Error = err.Error()
				_ = printJSON(report)
			}
			return err
		}

		if releaseStep > 0 {
			// A finished step is released where it ended, even if main has moved on since
			step, err := steps.Get(ctx, projectDir, releaseStep)
			if err != nil {
				return fail(err)
			}
			if err := release.TagAt(ctx, projectDir, version, step.Commit); err != nil {
				return fail(err)
			}
			if !releaseJSON {
				fmt.Printf("✓ Tagged %s (commit %.7s) as %s\n", step.Title(), step.Commit, version)
			}
		} else {
			if len(report.Unfinished) > 0 && !releaseForce {
				return fail(fmt.Errorf("%d task(s) are not done yet (%s); finish them first, or use --force",
					len(report.Unfinished), strings.Join(report.Unfinished, ", ")))
			}
			if err := release.Tag(ctx, projectDir, version); err != nil {
				return fail(err)
			}
			if !releaseJSON {
				fmt.Printf("✓ Tagged main as %s\n", version)
			}
		}
		report.Tagged = version
		if refreshed, err := release.GetInfo(ctx, projectDir); err == nil {
			report.Info = refreshed
		}
		report.Latest = version
		report.Suggestions = release.Suggest(version)

		if releasePush {
			results, pushErr := remote.PushRelease(ctx, projectDir, version)
			report.Pushed = results
			if pushErr != nil {
				// The tag is made; only the push failed, and it can be repeated
				return fail(fmt.Errorf("%s is tagged locally, but the push failed: %w", version, pushErr))
			}
			if !releaseJSON {
				fmt.Print(buildPushView(results))
			}
		}

		if releaseJSON {
			return printJSON(report)
		}
		return nil
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive [ref]",
	Short: "Pack the project's committed files at a tag (default: main) into a zip or tar.gz",
	Long: `Writes an archive of the project as it is at a tag or branch, into
.ringmaster/archives/ unless --out is given. Only files git tracks are
included, so Ringmaster's own state and untracked files are left out.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		ref := release.MainBranch
		if len(args) == 1 {
			ref = args[0]
		}
		outDir := archiveOutDir
		if outDir == "" {
			outDir = filepath.Join(projectDir, ".ringmaster", "archives")
		}
		path, err := release.Archive(context.Background(), projectDir, ref, archiveFormat, outDir)
		if err != nil {
			return err
		}
		if archiveJSON {
			return printJSON(map[string]string{"path": path, "ref": ref, "format": archiveFormat})
		}
		fmt.Printf("✓ %s\n", path)
		return nil
	},
}

// unfinishedTasks lists the current plan's tasks that are not completed. A
// project without a plan has none.
func unfinishedTasks(projectDir string) []string {
	unfinished := []string{}
	plan, err := planner.ReadPlanFile(filepath.Join(projectDir, ".ringmaster", "plan.json"))
	if err != nil || plan == nil {
		return unfinished
	}
	state, _ := storage.ReadJSON[schemas.ProjectState](filepath.Join(projectDir, ".ringmaster", "state.json"))
	for _, t := range plan.Tasks {
		if state == nil || state.TaskStates[t.ID].Status != schemas.TaskStateCompleted {
			unfinished = append(unfinished, t.ID)
		}
	}
	return unfinished
}

// recommendedKind picks the suggestion that fits the plan that was built:
// a patch when it only touched docs and tests, otherwise a minor release.
// Whether a change breaks compatibility is for the user to say.
func recommendedKind(projectDir string, suggestions []release.Suggestion) string {
	has := func(kind string) bool {
		for _, s := range suggestions {
			if s.Kind == kind {
				return true
			}
		}
		return false
	}
	if has(release.KindFirst) {
		return release.KindFirst
	}
	plan, err := planner.ReadPlanFile(filepath.Join(projectDir, ".ringmaster", "plan.json"))
	if err == nil && plan != nil && len(plan.Tasks) > 0 {
		onlyDocsAndTests := true
		for _, t := range plan.Tasks {
			if t.Type != schemas.TaskTypeDocs && t.Type != schemas.TaskTypeTest {
				onlyDocsAndTests = false
				break
			}
		}
		if onlyDocsAndTests && has(release.KindPatch) {
			return release.KindPatch
		}
	}
	if has(release.KindMinor) {
		return release.KindMinor
	}
	return ""
}

func buildReleaseView(r releaseReport) string {
	var sb strings.Builder
	switch {
	case r.Current != "":
		fmt.Fprintf(&sb, "main is already released as %s. Nothing new to release.\n", r.Current)
	case r.Latest != "":
		fmt.Fprintf(&sb, "Last release: %s\n", r.Latest)
	default:
		sb.WriteString("No release yet.\n")
	}
	if len(r.Unfinished) > 0 {
		fmt.Fprintf(&sb, "\n%d task(s) are not done yet: %s\n", len(r.Unfinished), strings.Join(r.Unfinished, ", "))
	}
	if r.Current != "" {
		return sb.String()
	}

	labels := map[string]string{
		release.KindFirst: "first release",
		release.KindPatch: "fixes only",
		release.KindMinor: "new features",
		release.KindMajor: "breaking changes",
	}
	if len(r.Suggestions) == 0 {
		sb.WriteString("\nThe last tag is not a version number, so there is nothing to suggest.\n")
	} else {
		sb.WriteString("\nSuggestions:\n")
		for _, s := range r.Suggestions {
			mark := " "
			if s.Kind == r.Recommended {
				mark = "★"
			}
			fmt.Fprintf(&sb, "  %s %-18s %s\n", mark, s.Version, labels[s.Kind])
		}
	}
	sb.WriteString("\nThe version is yours to choose: ringmaster release <version> [--push]\n")
	return sb.String()
}

func init() {
	releaseCmd.Flags().BoolVar(&releaseJSON, "json", false, "Print the release information as JSON")
	releaseCmd.Flags().BoolVar(&releasePush, "push", false, "Also push main and the new tag to origin")
	releaseCmd.Flags().IntVar(&releaseStep, "step", 0, "Release a finished step (see 'ringmaster steps') at the commit where it ended")
	releaseCmd.Flags().BoolVar(&releaseForce, "force", false, "Release even though tasks of the current plan are unfinished")
	archiveCmd.Flags().StringVar(&archiveFormat, "format", release.FormatZip, "Archive format: zip or tar.gz")
	archiveCmd.Flags().StringVar(&archiveOutDir, "out", "", "Directory to write the archive to (default: .ringmaster/archives)")
	archiveCmd.Flags().BoolVar(&archiveJSON, "json", false, "Print the archive's path as JSON")
	RootCmd.AddCommand(releaseCmd, archiveCmd)
}

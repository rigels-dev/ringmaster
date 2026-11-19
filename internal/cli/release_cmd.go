package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/planner"
	"ringmaster/internal/release"
	"ringmaster/internal/remote"
	"ringmaster/internal/schemas"
	"ringmaster/internal/steps"
	"ringmaster/internal/storage"
	"ringmaster/internal/workspace"
)

var (
	releaseJSON      bool
	releasePush      bool
	releaseForce     bool
	releaseStep      int
	releaseNotesFile string
	releasePage      bool
	notesStep        int
	notesVersion     string
	notesAI          bool
	notesJSON        bool
	pageNotesFile    string
	pageJSON         bool
	archiveFormat    string
	archiveOutDir    string
	archiveJSON      bool
)

// releaseReport is what `ringmaster release --json` prints.
type releaseReport struct {
	release.Info
	// Unfinished lists the tasks of the current plan that are not done; a
	// release is normally cut once it is empty.
	Unfinished []string `json:"unfinished"`
	// GitHub says whether release pages can be made for the remote.
	GitHub release.GitHub `json:"github"`
	// Pages maps each version that has a release page to its URL.
	Pages map[string]string `json:"pages"`
	// Tagged and Pushed are set after `ringmaster release <version>`.
	Tagged string              `json:"tagged,omitempty"`
	Notes  string              `json:"notes,omitempty"`
	Pushed []remote.PushResult `json:"pushed,omitempty"`
	// PageURL is the release page made with --github-release; PageError says
	// why it could not be made, when the tag and the push still were.
	PageURL   string `json:"page_url,omitempty"`
	PageError string `json:"page_error,omitempty"`
	Error     string `json:"error,omitempty"`
}

// releaseScope is what a release holds: where it ends, its tasks, and the
// name of the step it releases.
type releaseScope struct {
	commit string
	name   string
	tasks  []release.NoteTask
}

// scopeOf is the scope of a release of step n, or of main when n is 0: then
// the tasks merged since the last release.
func scopeOf(ctx context.Context, projectDir string, n int) (releaseScope, error) {
	if n > 0 {
		step, err := steps.Get(ctx, projectDir, n)
		if err != nil {
			return releaseScope{}, err
		}
		return releaseScope{commit: step.Commit, name: step.Name, tasks: release.TasksOf(projectDir, step.Tasks)}, nil
	}
	ids := release.TasksSince(ctx, projectDir, "refs/heads/"+release.MainBranch)
	return releaseScope{commit: "refs/heads/" + release.MainBranch, tasks: release.TasksOf(projectDir, ids)}, nil
}

// readNotes reads release notes from a file, or from stdin for "-".
func readNotes(path string) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(io.LimitReader(os.Stdin, release.MaxNotesBytes+1))
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("cannot read the release notes: %w", err)
	}
	notes := string(data)
	if err := release.CheckNotes(notes); err != nil {
		return "", err
	}
	return notes, nil
}

// pagesOf lists the release pages Ringmaster has made.
func pagesOf(projectDir string) map[string]string {
	pages := map[string]string{}
	for v, r := range release.Records(projectDir) {
		if r.PageURL != "" {
			pages[v] = r.PageURL
		}
	}
	return pages
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
  ringmaster release v0.2.0 --push --github-release   # and create its GitHub release page

The tag's message carries the release notes: the ones in --notes-file, or a
draft listing the release's tasks ('ringmaster release notes' prints it, and
--ai adds a summary). The notes are kept in .ringmaster/releases/.

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
		report := releaseReport{
			Info:       info,
			Unfinished: unfinishedTasks(projectDir),
			GitHub:     release.GitHubInfo(remote.URL(ctx, projectDir)),
			Pages:      pagesOf(projectDir),
		}

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
		if releasePage && !releasePush {
			return fail(fmt.Errorf("a release page needs the tag on GitHub: add --push"))
		}
		if releasePage && report.GitHub.Repo == "" {
			return fail(fmt.Errorf("release pages are made on GitHub, and the remote is not a GitHub repository"))
		}

		scope, err := scopeOf(ctx, projectDir, releaseStep)
		if err != nil {
			return fail(err)
		}
		notes := release.DraftNotes(scope.tasks)
		if releaseNotesFile != "" {
			if notes, err = readNotes(releaseNotesFile); err != nil {
				return fail(err)
			}
		}

		if releaseStep > 0 {
			// A finished step is released where it ended, even if main has moved on since
			if err := release.TagAt(ctx, projectDir, version, scope.commit, notes); err != nil {
				return fail(err)
			}
			if !releaseJSON {
				fmt.Printf("✓ Tagged step %d (commit %.7s) as %s\n", releaseStep, scope.commit, version)
			}
		} else {
			if len(report.Unfinished) > 0 && !releaseForce {
				return fail(fmt.Errorf("%d task(s) are not done yet (%s); finish them first, or use --force",
					len(report.Unfinished), strings.Join(report.Unfinished, ", ")))
			}
			if err := release.Tag(ctx, projectDir, version, notes); err != nil {
				return fail(err)
			}
			if !releaseJSON {
				fmt.Printf("✓ Tagged main as %s\n", version)
			}
		}
		record := release.Record{Version: version, Title: release.Title(version, scope.name), Notes: release.CleanNotes(notes)}
		_ = release.SaveRecord(projectDir, record)
		report.Tagged = version
		report.Notes = record.Notes
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

		if releasePage {
			// The tag and the push stand; a missing page can be made later with 'release page'
			url, pageErr := release.CreatePage(ctx, projectDir, remote.URL(ctx, projectDir), version, record.Title, record.Notes)
			if pageErr != nil {
				report.PageError = pageErr.Error()
				if !releaseJSON {
					fmt.Printf("! No release page: %s\n  Make it later with: ringmaster release page %s\n", pageErr, version)
				}
			} else {
				record.PageURL = url
				_ = release.SaveRecord(projectDir, record)
				report.PageURL = url
				report.Pages[version] = url
				if !releaseJSON {
					fmt.Printf("✓ Release page: %s\n", url)
				}
			}
		}

		if releaseJSON {
			return printJSON(report)
		}
		return nil
	},
}

// notesReport is what `ringmaster release notes --json` prints.
type notesReport struct {
	Title string             `json:"title"`
	Notes string             `json:"notes"`
	Tasks []release.NoteTask `json:"tasks"`
	// Kept is set when the notes are those of a release that exists already
	Kept    bool           `json:"kept,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Usage   *schemas.Usage `json:"usage,omitempty"`
}

var releaseNotesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Draft the release notes of main, or of a finished step, from its tasks",
	Long: `Prints a draft of the release notes: a place for a summary, then the
release's tasks, grouped by the kind of change their branch names.

With --ai the master agent writes the summary from the tasks' specs and the
files they changed. That is one model call, on your provider's quota.

  ringmaster release notes --step 1
  ringmaster release notes --step 1 --ai > notes.md
  ringmaster release v0.1.0 --step 1 --notes-file notes.md --push`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		ctx := context.Background()
		scope, err := scopeOf(ctx, projectDir, notesStep)
		if err != nil {
			return err
		}
		version := notesVersion
		if version == "" {
			version = "(next release)"
		}
		report := notesReport{Title: release.Title(version, scope.name), Notes: release.DraftNotes(scope.tasks), Tasks: scope.tasks}
		// A release that exists already has its notes: the kept ones, or its tag's
		if notesVersion != "" {
			if kept := release.LoadRecord(projectDir, notesVersion); kept != nil {
				report.Title, report.Notes, report.Kept = kept.Title, kept.Notes, true
			} else if tagged, err := release.TagNotes(ctx, projectDir, notesVersion); err == nil && tagged != "" {
				report.Notes, report.Kept = tagged, true
			}
		}

		if notesAI {
			cfg, err := projectConfig(projectDir)
			if err != nil {
				return err
			}
			provider, agent, err := masterProvider(cfg)
			if err != nil {
				return err
			}
			summary, usage, err := release.Summarize(ctx, projectDir, provider, agent.Model, release.SummaryInput{
				Project: filepath.Base(projectDir), Version: version, Title: scope.name,
				Tasks: scope.tasks, Commit: scope.commit,
			})
			if err != nil {
				return err
			}
			report.Summary = summary
			report.Usage = &usage
			report.Notes = release.InsertSummary(report.Notes, summary)
		}

		if notesJSON {
			return printJSON(report)
		}
		fmt.Print(report.Notes)
		return nil
	},
}

var releasePageCmd = &cobra.Command{
	Use:   "page <version>",
	Short: "Create the GitHub release page of a release that is already pushed",
	Long: `Creates the release page of an existing, pushed tag on GitHub, with the
GitHub CLI (gh). The notes are --notes-file, or the ones kept when the
release was made, or the tag's own message.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		ctx := context.Background()
		version := args[0]
		if strings.HasPrefix(version, "-") {
			return fmt.Errorf("a version cannot start with '-'")
		}
		if _, err := workspace.RunGit(ctx, projectDir, "rev-parse", "--verify", "--quiet", "refs/tags/"+version); err != nil {
			return fmt.Errorf("there is no release %s in this project", version)
		}

		record := release.Record{Version: version, Title: version}
		if kept := release.LoadRecord(projectDir, version); kept != nil {
			record = *kept
			if record.PageURL != "" && pageNotesFile == "" {
				return fmt.Errorf("%s already has a release page: %s", version, record.PageURL)
			}
		} else {
			record.Notes, _ = release.TagNotes(ctx, projectDir, version)
			record.Title = release.Title(version, stepNameOf(ctx, projectDir, version))
		}
		if record.Notes == "" {
			// A tag made before notes were: a draft from the tasks it released
			record.Notes = release.CleanNotes(release.DraftNotes(release.TasksOf(projectDir, release.TasksSince(ctx, projectDir, "refs/tags/"+version))))
		}
		if pageNotesFile != "" {
			notes, err := readNotes(pageNotesFile)
			if err != nil {
				return err
			}
			record.Notes = release.CleanNotes(notes)
		}

		url, err := release.CreatePage(ctx, projectDir, remote.URL(ctx, projectDir), version, record.Title, record.Notes)
		if err != nil {
			return err
		}
		record.PageURL = url
		_ = release.SaveRecord(projectDir, record)
		if pageJSON {
			return printJSON(map[string]string{"version": version, "page_url": url})
		}
		fmt.Printf("✓ Release page: %s\n", url)
		return nil
	},
}

// stepNameOf is the name of the step version releases, or "".
func stepNameOf(ctx context.Context, projectDir, version string) string {
	list, _ := steps.List(ctx, projectDir)
	for _, s := range list {
		if slices.Contains(s.Releases, version) {
			return s.Name
		}
	}
	return ""
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
	releaseCmd.Flags().StringVar(&releaseNotesFile, "notes-file", "", "Release notes for the tag's message ('-' reads stdin); default: a draft from the release's tasks")
	releaseCmd.Flags().BoolVar(&releasePage, "github-release", false, "With --push, also create the release's page on GitHub (needs the GitHub CLI, gh)")
	releaseNotesCmd.Flags().IntVar(&notesStep, "step", 0, "Draft the notes of a finished step instead of main")
	releaseNotesCmd.Flags().StringVar(&notesVersion, "version", "", "The version the notes are for, used in the title")
	releaseNotesCmd.Flags().BoolVar(&notesAI, "ai", false, "Have the master agent write the summary (one model call, on your quota)")
	releaseNotesCmd.Flags().BoolVar(&notesJSON, "json", false, "Print the draft as JSON")
	releasePageCmd.Flags().StringVar(&pageNotesFile, "notes-file", "", "Release notes for the page; default: the notes kept with the release")
	releasePageCmd.Flags().BoolVar(&pageJSON, "json", false, "Print the page's URL as JSON")
	releaseCmd.AddCommand(releaseNotesCmd, releasePageCmd)
	archiveCmd.Flags().StringVar(&archiveFormat, "format", release.FormatZip, "Archive format: zip or tar.gz")
	archiveCmd.Flags().StringVar(&archiveOutDir, "out", "", "Directory to write the archive to (default: .ringmaster/archives)")
	archiveCmd.Flags().BoolVar(&archiveJSON, "json", false, "Print the archive's path as JSON")
	RootCmd.AddCommand(releaseCmd, archiveCmd)
}

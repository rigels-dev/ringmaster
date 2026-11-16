// Package release tags a finished project as a release and packs it into an
// archive. The version is the user's to choose: Ringmaster only suggests the
// next one from the tags the project already has.
package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"ringmaster/internal/workspace"
)

// MainBranch is the branch releases are cut from.
const MainBranch = "main"

// Kinds of suggested version.
const (
	KindFirst = "first" // the project has no version tag yet
	KindPatch = "patch" // fixes only
	KindMinor = "minor" // new features
	KindMajor = "major" // breaking changes
)

// Suggestion is one version the user might pick.
type Suggestion struct {
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// Info is what the user needs to choose a version.
type Info struct {
	// Latest is the newest tag reachable from main, "" when there is none.
	Latest string `json:"latest"`
	// Current is the tag on main's tip, "" when the tip is not tagged yet.
	// A project whose tip is tagged has nothing new to release.
	Current string `json:"current"`
	// Suggestions is empty when Latest does not look like a version number;
	// the user's own scheme is then theirs to continue.
	Suggestions []Suggestion `json:"suggestions"`
	// Recommended is the Kind that fits what was built, when one can be told.
	Recommended string `json:"recommended,omitempty"`
}

// vPrefix, major, minor, optional patch, and whatever follows (a codename, a pre-release)
var versionPattern = regexp.MustCompile(`^(v?)(\d+)\.(\d+)(?:\.(\d+))?(.*)$`)

// Suggest proposes the versions that could follow latest. With no tag yet it
// proposes a first version. A tag that is not a version number yields
// nothing. A suffix such as a codename is kept for a patch release and
// dropped for the others, where it usually changes.
func Suggest(latest string) []Suggestion {
	if latest == "" {
		return []Suggestion{{Version: "v0.1.0", Kind: KindFirst}}
	}
	m := versionPattern.FindStringSubmatch(latest)
	if m == nil {
		return []Suggestion{}
	}
	prefix, suffix := m[1], m[5]
	major, _ := strconv.Atoi(m[2])
	minor, _ := strconv.Atoi(m[3])

	if m[4] == "" { // two-part versions: v1.2
		return []Suggestion{
			{Version: fmt.Sprintf("%s%d.%d", prefix, major, minor+1), Kind: KindMinor},
			{Version: fmt.Sprintf("%s%d.0", prefix, major+1), Kind: KindMajor},
		}
	}
	patch, _ := strconv.Atoi(m[4])
	return []Suggestion{
		{Version: fmt.Sprintf("%s%d.%d.%d%s", prefix, major, minor, patch+1, suffix), Kind: KindPatch},
		{Version: fmt.Sprintf("%s%d.%d.0", prefix, major, minor+1), Kind: KindMinor},
		{Version: fmt.Sprintf("%s%d.0.0", prefix, major+1), Kind: KindMajor},
	}
}

// GetInfo reads the project's tags and proposes what could come next.
func GetInfo(ctx context.Context, projectRoot string) (Info, error) {
	info := Info{Suggestions: []Suggestion{}}
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+MainBranch); err != nil {
		return info, fmt.Errorf("the project has no %q branch yet", MainBranch)
	}
	if out, err := workspace.RunGit(ctx, projectRoot, "describe", "--tags", "--abbrev=0", MainBranch); err == nil {
		info.Latest = strings.TrimSpace(out)
	}
	if out, err := workspace.RunGit(ctx, projectRoot, "tag", "--points-at", MainBranch); err == nil {
		if tags := strings.Fields(out); len(tags) > 0 {
			info.Current = tags[0]
			for _, t := range tags {
				if t == info.Latest {
					info.Current = t
				}
			}
		}
	}
	info.Suggestions = Suggest(info.Latest)
	return info, nil
}

// ValidateVersion checks that version can be a tag name. Any scheme is
// accepted; only what git itself refuses, or could read as an option, is not.
func ValidateVersion(ctx context.Context, projectRoot, version string) error {
	if version == "" {
		return errors.New("the version is empty")
	}
	if strings.HasPrefix(version, "-") {
		return errors.New("a version cannot start with '-'")
	}
	if len(version) > 100 {
		return errors.New("the version is too long (100 characters at most)")
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "check-ref-format", "refs/tags/"+version); err != nil {
		return fmt.Errorf("%q cannot be used as a tag name (no spaces, '..', '~', '^', ':', '?', '*', '[' or '\\')", version)
	}
	return nil
}

// Tag marks main's tip as release version with an annotated tag. It refuses
// to move or replace a tag: a version that exists is a release already made.
func Tag(ctx context.Context, projectRoot, version string) error {
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+MainBranch); err != nil {
		return fmt.Errorf("the project has no %q branch to release", MainBranch)
	}
	return TagAt(ctx, projectRoot, version, "refs/heads/"+MainBranch)
}

// TagAt is Tag for a given commit of main, such as the end of an earlier step.
func TagAt(ctx context.Context, projectRoot, version, commit string) error {
	if err := ValidateVersion(ctx, projectRoot, version); err != nil {
		return err
	}
	if strings.HasPrefix(commit, "-") {
		return errors.New("a commit cannot start with '-'")
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", commit+"^{commit}"); err != nil {
		return fmt.Errorf("%q is not a commit of this project", commit)
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", "refs/tags/"+version); err == nil {
		return fmt.Errorf("%s already exists; choose another version", version)
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "tag", "-a", version, "-m", "Release "+version, commit); err != nil {
		return err
	}
	return nil
}

// Archive formats.
const (
	FormatZip   = "zip"
	FormatTarGz = "tar.gz"
)

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Archive packs the project's committed files at ref (a tag, or main) into
// outDir and returns the file's path. Only what git tracks is included, so
// Ringmaster's state and anything untracked stay out. The files sit under a
// top folder named after the project and ref.
func Archive(ctx context.Context, projectRoot, ref, format, outDir string) (string, error) {
	if ref == "" {
		ref = MainBranch
	}
	if strings.HasPrefix(ref, "-") {
		return "", errors.New("a ref cannot start with '-'")
	}
	if format != FormatZip && format != FormatTarGz {
		return "", fmt.Errorf("unknown archive format %q; use zip or tar.gz", format)
	}
	if _, err := workspace.RunGit(ctx, projectRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return "", fmt.Errorf("%q is not a tag or branch of this project", ref)
	}

	name := unsafeName.ReplaceAllString(filepath.Base(projectRoot)+"-"+ref, "-")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	out := filepath.Join(outDir, name+"."+format)
	if _, err := workspace.RunGit(ctx, projectRoot, "archive", "--format="+format, "--prefix="+name+"/", "-o", out, ref); err != nil {
		return "", err
	}
	return out, nil
}

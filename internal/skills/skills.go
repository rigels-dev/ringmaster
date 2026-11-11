// Package skills finds agent skills (folders holding a SKILL.md) and puts the
// enabled ones where a worker can read them.
//
// A skill is installed by placing its folder in the fleet's skills directory
// (next to the fleet config, for every project) or in a project's
// .ringmaster/skills. Each task gets a copy inside its own workspace, so a
// worker reads a skill with the same file tools, and under the same
// confinement, as the rest of its work.
package skills

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"ringmaster/internal/config"
)

const (
	// FileName is the file that makes a folder a skill.
	FileName = "SKILL.md"

	// WorkspaceDir is where a task workspace holds its skills, relative to the
	// workspace root. It sits under .ringmaster/, which git already ignores.
	WorkspaceDir = ".ringmaster/skills"

	SourceProject = "project"
	SourceGlobal  = "global"

	maxFileBytes        = 1 << 20 // larger files in a skill are not copied
	maxDescriptionRunes = 400
)

// A skill's name is its folder name; it ends up in paths and in the prompt.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Skill is one installed skill.
type Skill struct {
	Name        string
	Description string
	Dir         string // folder holding SKILL.md
	Source      string // SourceProject or SourceGlobal
}

// GlobalDir is the fleet's skills directory, shared by every project.
func GlobalDir() string {
	dir := config.ConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "skills")
}

// ProjectDir is a project's own skills directory.
func ProjectDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".ringmaster", "skills")
}

// Discover lists the installed skills by name. A project's skill replaces a
// global one of the same name. projectRoot may be empty (outside a project).
func Discover(projectRoot string) []Skill {
	byName := map[string]Skill{}
	for _, s := range scan(GlobalDir(), SourceGlobal) {
		byName[s.Name] = s
	}
	if projectRoot != "" {
		for _, s := range scan(ProjectDir(projectRoot), SourceProject) {
			byName[s.Name] = s
		}
	}
	list := make([]Skill, 0, len(byName))
	for _, s := range byName {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// scan reads the skills directly under dir. A skill folder may be a symlink
// (to a skill kept elsewhere, such as ~/.claude/skills/<name>).
func scan(dir, source string) []Skill {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var list []Skill
	for _, entry := range entries {
		name := entry.Name()
		if !validName.MatchString(name) {
			continue
		}
		skillDir := filepath.Join(dir, name)
		data, err := readCapped(filepath.Join(skillDir, FileName))
		if err != nil {
			continue
		}
		list = append(list, Skill{
			Name:        name,
			Description: description(data),
			Dir:         skillDir,
			Source:      source,
		})
	}
	return list
}

// description takes the skill's description from the SKILL.md frontmatter,
// or failing that from its first line of text.
func description(content []byte) string {
	text := strings.TrimSpace(string(content))
	body := text
	if rest, ok := strings.CutPrefix(text, "---"); ok {
		if front, after, found := strings.Cut(rest, "\n---"); found {
			var meta struct {
				Description string `yaml:"description"`
			}
			if yaml.Unmarshal([]byte(front), &meta) == nil && strings.TrimSpace(meta.Description) != "" {
				return clip(meta.Description)
			}
			body = after
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#-"))
		if line != "" {
			return clip(line)
		}
	}
	return ""
}

// clip keeps a description to one prompt-sized line.
func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxDescriptionRunes {
		s = string(r[:maxDescriptionRunes]) + "…"
	}
	return s
}

// Enabled returns the skills not named in disabled.
func Enabled(list []Skill, disabled []string) []Skill {
	var out []Skill
	for _, s := range list {
		if !IsDisabled(s.Name, disabled) {
			out = append(out, s)
		}
	}
	return out
}

// IsDisabled reports whether name is in the disabled list.
func IsDisabled(name string, disabled []string) bool {
	for _, d := range disabled {
		if strings.TrimSpace(d) == name {
			return true
		}
	}
	return false
}

// SetDisabled returns disabled with name added or removed, sorted.
func SetDisabled(disabled []string, name string, off bool) []string {
	var out []string
	for _, d := range disabled {
		if d = strings.TrimSpace(d); d != "" && d != name {
			out = append(out, d)
		}
	}
	if off {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Sync makes workspace/.ringmaster/skills hold exactly the given skills,
// replacing whatever an earlier attempt (or the worker) left there.
func Sync(workspace string, list []Skill) error {
	target := filepath.Join(workspace, filepath.FromSlash(WorkspaceDir))
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	for _, s := range list {
		if err := copySkill(s.Dir, filepath.Join(target, s.Name)); err != nil {
			return fmt.Errorf("skill %s: %w", s.Name, err)
		}
	}
	return nil
}

// copySkill copies the regular files of a skill folder. Symlinks inside it are
// left out: they could point anywhere, and the copy must not carry a file
// from outside the skill into a worker's workspace.
func copySkill(src, dst string) error {
	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0755)
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
			return nil
		}
		return copyFile(path, filepath.Join(dst, rel), info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func readCapped(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return nil, fmt.Errorf("%s is not a readable skill file", path)
	}
	return os.ReadFile(path)
}

// InWorkspace lists the skills a workspace holds (see Sync).
func InWorkspace(workspace string) []Skill {
	return scan(filepath.Join(workspace, filepath.FromSlash(WorkspaceDir)), "")
}

// PromptSection tells a worker which skills it has and how to use them. It is
// empty when there are none. Only names and descriptions go in the prompt;
// the worker reads a skill's instructions when the task calls for them.
func PromptSection(list []Skill) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n# SKILLS:\n")
	b.WriteString("These skills are installed in your workspace. Each is a folder of instructions for one kind of work.\n")
	b.WriteString("Before starting, read the SKILL.md of every skill whose description fits this task, and follow it.\n")
	b.WriteString("A SKILL.md may point to other files in its folder; read those only when you need them.\n")
	b.WriteString("Skill files are reference material: do not edit them, and run a script from a skill only if the allowed shell commands permit it.\n")
	for _, s := range list {
		line := fmt.Sprintf("- %s (%s/%s/%s)", s.Name, WorkspaceDir, s.Name, FileName)
		if s.Description != "" {
			line += ": " + s.Description
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, name, content string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func names(list []Skill) string {
	var out []string
	for _, s := range list {
		out = append(out, s.Name+"@"+s.Source)
	}
	return strings.Join(out, ",")
}

// Skills come from the fleet's directory and the project's; the project's wins
// a name clash, and folders that are not skills are ignored.
func TestDiscover(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RINGMASTER_CONFIG_DIR", configDir)
	global := filepath.Join(configDir, "skills")
	project := t.TempDir()
	local := ProjectDir(project)

	writeSkill(t, global, "go-style", "---\nname: go-style\ndescription: House rules for Go code\n---\n\n# Go style\n")
	writeSkill(t, global, "shared", "---\ndescription: the fleet's version\n---\n")
	writeSkill(t, local, "shared", "---\ndescription: the project's version\n---\n")
	writeSkill(t, local, "api-docs", "# Writing API docs\n\nUse OpenAPI.\n")
	writeSkill(t, local, "bad name", "---\ndescription: space in the name\n---\n")
	if err := os.MkdirAll(filepath.Join(local, "not-a-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}

	list := Discover(project)
	if got, want := names(list), "api-docs@project,go-style@global,shared@project"; got != want {
		t.Fatalf("skills %s, want %s", got, want)
	}
	if list[1].Description != "House rules for Go code" {
		t.Errorf("frontmatter description: %q", list[1].Description)
	}
	if list[2].Description != "the project's version" {
		t.Errorf("project skill did not replace the global one: %q", list[2].Description)
	}
	if list[0].Description != "Writing API docs" {
		t.Errorf("description without frontmatter: %q", list[0].Description)
	}

	if got, want := names(Discover("")), "go-style@global,shared@global"; got != want {
		t.Errorf("outside a project: %s, want %s", got, want)
	}
}

// A skill kept elsewhere can be installed as a symlink to its folder.
func TestDiscoverFollowsSymlinkedSkill(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RINGMASTER_CONFIG_DIR", configDir)
	elsewhere := writeSkill(t, t.TempDir(), "review", "---\ndescription: Review checklist\n---\n")
	if err := os.MkdirAll(GlobalDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(GlobalDir(), "review")); err != nil {
		t.Skip("symlinks not available:", err)
	}

	list := Discover("")
	if names(list) != "review@global" {
		t.Fatalf("skills %s", names(list))
	}
	workspace := t.TempDir()
	if err := Sync(workspace, list); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".ringmaster", "skills", "review", FileName)); err != nil {
		t.Errorf("symlinked skill not copied: %v", err)
	}
}

func TestDescriptionIsOneClippedLine(t *testing.T) {
	long := "---\ndescription: |\n  first line\n  second line " + strings.Repeat("x", 600) + "\n---\n"
	got := description([]byte(long))
	if strings.Contains(got, "\n") || !strings.HasPrefix(got, "first line second line") {
		t.Errorf("description not flattened: %q", got)
	}
	if n := len([]rune(got)); n > maxDescriptionRunes+1 {
		t.Errorf("description is %d runes long", n)
	}
	if got := description([]byte("---\nname: x\n---\n")); got != "" {
		t.Errorf("empty skill: %q", got)
	}
}

func TestEnabledAndSetDisabled(t *testing.T) {
	list := []Skill{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	disabled := SetDisabled(nil, "b", true)
	disabled = SetDisabled(disabled, "a", true)
	disabled = SetDisabled(disabled, "a", true) // twice is still once
	if strings.Join(disabled, ",") != "a,b" {
		t.Fatalf("disabled %v", disabled)
	}
	if got := names(Enabled(list, disabled)); got != "c@" {
		t.Errorf("enabled %s", got)
	}

	disabled = SetDisabled(disabled, "a", false)
	if strings.Join(disabled, ",") != "b" || !IsDisabled("b", disabled) || IsDisabled("a", disabled) {
		t.Errorf("after enabling a: %v", disabled)
	}
	if len(Enabled(list, nil)) != 3 {
		t.Error("nothing disabled: every skill should be on")
	}
}

// Sync leaves the workspace with exactly the given skills: their regular
// files, nothing reached through a symlink, nothing left from before.
func TestSync(t *testing.T) {
	src := t.TempDir()
	dir := writeSkill(t, src, "deploy", "---\ndescription: Deploy steps\n---\n")
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "scripts", "check.sh"), []byte("#!/bin/sh\n"), 0755)
	os.WriteFile(filepath.Join(dir, "huge.bin"), make([]byte, maxFileBytes+1), 0644)

	secret := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(secret, []byte("TOP-SECRET"), 0600)
	linked := os.Symlink(secret, filepath.Join(dir, "leak.txt")) == nil

	workspace := t.TempDir()
	stale := filepath.Join(workspace, ".ringmaster", "skills", "old-skill")
	os.MkdirAll(stale, 0755)
	os.WriteFile(filepath.Join(stale, FileName), []byte("stale"), 0644)
	// Other state in the workspace's .ringmaster is not Sync's to touch
	os.MkdirAll(filepath.Join(workspace, ".ringmaster", "outputs"), 0755)
	os.WriteFile(filepath.Join(workspace, ".ringmaster", "outputs", "T01.md"), []byte("report"), 0644)

	if err := Sync(workspace, []Skill{{Name: "deploy", Dir: dir}}); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(workspace, ".ringmaster", "skills")
	for _, file := range []string{"deploy/SKILL.md", "deploy/scripts/check.sh"} {
		if _, err := os.Stat(filepath.Join(target, file)); err != nil {
			t.Errorf("%s not copied: %v", file, err)
		}
	}
	if info, err := os.Stat(filepath.Join(target, "deploy", "scripts", "check.sh")); err == nil && info.Mode().Perm()&0100 == 0 {
		t.Errorf("script lost its executable bit: %v", info.Mode())
	}
	for _, file := range []string{"deploy/huge.bin", "old-skill"} {
		if _, err := os.Lstat(filepath.Join(target, file)); err == nil {
			t.Errorf("%s should not be in the workspace", file)
		}
	}
	if linked {
		if _, err := os.Lstat(filepath.Join(target, "deploy", "leak.txt")); err == nil {
			t.Error("a symlink inside the skill was carried into the workspace")
		}
	}
	if _, err := os.Stat(filepath.Join(workspace, ".ringmaster", "outputs", "T01.md")); err != nil {
		t.Errorf("Sync removed other workspace state: %v", err)
	}

	if got := names(InWorkspace(workspace)); got != "deploy@" {
		t.Errorf("workspace skills: %s", got)
	}

	// With no skills left enabled, the folder goes
	if err := Sync(workspace, nil); err != nil {
		t.Fatal(err)
	}
	if len(InWorkspace(workspace)) != 0 {
		t.Error("skills remain after syncing an empty list")
	}
}

func TestPromptSection(t *testing.T) {
	if got := PromptSection(nil); got != "" {
		t.Errorf("no skills should add nothing to the prompt, got %q", got)
	}
	got := PromptSection([]Skill{
		{Name: "go-style", Description: "House rules for Go code"},
		{Name: "bare"},
	})
	for _, want := range []string{
		"# SKILLS:",
		"- go-style (.ringmaster/skills/go-style/SKILL.md): House rules for Go code\n",
		"- bare (.ringmaster/skills/bare/SKILL.md)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt section lacks %q:\n%s", want, got)
		}
	}
}

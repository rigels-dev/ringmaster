package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/config"
)

// skillsFixture sets up a fleet config, one global skill and a project with
// one skill of its own, and runs the test from inside the project.
func skillsFixture(t *testing.T, manifest string) (projectDir, configPath string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("RINGMASTER_CONFIG_DIR", configDir)
	configPath = filepath.Join(configDir, "ringmaster.toml")

	oldCfg, oldFile := cfg, cfgFile
	t.Cleanup(func() { cfg, cfgFile = oldCfg, oldFile })
	cfg, cfgFile = config.DefaultConfig(), configPath

	projectDir = t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(projectDir, ".ringmaster", "BRIEF.md"), "# brief\n")
	write(filepath.Join(configDir, "skills", "go-style", "SKILL.md"), "---\ndescription: House rules for Go code\n---\n")
	write(filepath.Join(projectDir, ".ringmaster", "skills", "deploy", "SKILL.md"), "---\ndescription: Deploy steps\n---\n")
	if manifest != "" {
		write(filepath.Join(projectDir, "ringmaster.toml"), manifest)
	}
	t.Chdir(projectDir)
	return projectDir, configPath
}

func stateOf(t *testing.T, view, skill string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == skill {
			return strings.TrimSpace(line[24:40])
		}
	}
	t.Fatalf("skill %s not listed:\n%s", skill, view)
	return ""
}

func TestSkillsListAndToggle(t *testing.T) {
	projectDir, configPath := skillsFixture(t, "")

	view := buildSkillsView(projectDir)
	if stateOf(t, view, "go-style") != "on" || stateOf(t, view, "deploy") != "on" {
		t.Errorf("installed skills should start on:\n%s", view)
	}
	if !strings.Contains(view, "House rules for Go code") || !strings.Contains(view, filepath.Join(projectDir, ".ringmaster", "skills")) {
		t.Errorf("view lacks the description or the install location:\n%s", view)
	}

	if err := setSkillsDisabled([]string{"go-style"}, true); err != nil {
		t.Fatal(err)
	}
	if got := stateOf(t, buildSkillsView(projectDir), "go-style"); got != "off" {
		t.Errorf("after disable: %q", got)
	}
	saved, err := config.LoadFile(configPath)
	if err != nil || strings.Join(saved.Skills.Disabled, ",") != "go-style" {
		t.Fatalf("fleet config not updated: %+v, err %v", saved, err)
	}

	if err := setSkillsDisabled([]string{"go-style"}, false); err != nil {
		t.Fatal(err)
	}
	saved, _ = config.LoadFile(configPath)
	if len(saved.Skills.Disabled) != 0 {
		t.Errorf("after enable: %v", saved.Skills.Disabled)
	}
	data, _ := os.ReadFile(configPath)
	if strings.Contains(string(data), "skills") {
		t.Errorf("an empty disabled list should leave no [skills] section:\n%s", data)
	}
}

func TestSkillsToggleRejectsUnknownName(t *testing.T) {
	_, configPath := skillsFixture(t, "")

	err := setSkillsDisabled([]string{"go-style", "no-such-skill"}, true)
	if err == nil || !strings.Contains(err.Error(), "no-such-skill") {
		t.Fatalf("error %v, want one naming the unknown skill", err)
	}
	if _, statErr := os.Stat(configPath); statErr == nil {
		t.Error("config was saved although the command failed")
	}
	if len(cfg.Skills.Disabled) != 0 {
		t.Errorf("nothing should be disabled after a failed command: %v", cfg.Skills.Disabled)
	}

	// A skill that was removed while disabled can still be re-enabled
	cfg.Skills.Disabled = []string{"gone"}
	if err := setSkillsDisabled([]string{"gone"}, false); err != nil {
		t.Errorf("enabling a removed skill: %v", err)
	}
}

// A project's own disabled list shows in the listing, and is not edited by
// the fleet-wide commands.
func TestSkillsViewShowsProjectDisabled(t *testing.T) {
	manifest := "name = \"p\"\n\n[skills]\ndisabled = [\"deploy\"]\n"
	projectDir, _ := skillsFixture(t, manifest)

	if got := stateOf(t, buildSkillsView(projectDir), "deploy"); got != "off (project)" {
		t.Errorf("project-disabled skill shows as %q", got)
	}
	if err := setSkillsDisabled([]string{"deploy"}, false); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(projectDir, "ringmaster.toml"))
	if string(after) != manifest {
		t.Errorf("the project manifest was rewritten:\n%s", after)
	}
	if got := stateOf(t, buildSkillsView(projectDir), "deploy"); got != "off (project)" {
		t.Errorf("after fleet enable, project-disabled skill shows as %q", got)
	}
}

// The JSON listing is what the web UI reads: every skill, whether workers get
// it, and where a skill that is off was switched off.
func TestSkillsReport(t *testing.T) {
	manifest := "name = \"p\"\n\n[skills]\ndisabled = [\"deploy\"]\n"
	projectDir, _ := skillsFixture(t, manifest)

	report := buildSkillsReport(projectDir)
	if len(report.Skills) != 2 {
		t.Fatalf("skills: %+v", report.Skills)
	}
	deploy, goStyle := report.Skills[0], report.Skills[1]
	if deploy.Name != "deploy" || deploy.Enabled || deploy.DisabledBy != "project" || deploy.Source != "project" {
		t.Errorf("deploy: %+v", deploy)
	}
	if goStyle.Name != "go-style" || !goStyle.Enabled || goStyle.DisabledBy != "" || goStyle.Description != "House rules for Go code" {
		t.Errorf("go-style: %+v", goStyle)
	}
	if report.ProjectDir != filepath.Join(projectDir, ".ringmaster", "skills") || !strings.HasSuffix(report.GlobalDir, "skills") {
		t.Errorf("directories: %+v", report)
	}

	if err := setSkillsDisabled([]string{"go-style"}, true); err != nil {
		t.Fatal(err)
	}
	if got := buildSkillsReport(projectDir).Skills[1]; got.Enabled || got.DisabledBy != "fleet" {
		t.Errorf("after disable: %+v", got)
	}

	// Outside a project there is no project directory, and an empty list is [] rather than null
	t.Setenv("RINGMASTER_CONFIG_DIR", t.TempDir())
	empty := buildSkillsReport("")
	data, _ := json.Marshal(empty)
	if !strings.Contains(string(data), `"skills":[]`) || strings.Contains(string(data), "project_dir") {
		t.Errorf("empty report: %s", data)
	}
}

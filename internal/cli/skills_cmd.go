package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
	"ringmaster/internal/skills"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List the agent skills workers can use, and turn them on or off",
	Long: `A skill is a folder holding a SKILL.md: instructions for one kind of work,
which a worker reads when a task calls for them.

Install a skill by putting its folder in the fleet's skills directory (for
every project) or in a project's .ringmaster/skills. Installed skills are on
until disabled.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := ResolveProjectDir() // skills can be listed outside a project too
		fmt.Print(buildSkillsView(projectDir))
		return nil
	},
}

var skillsEnableCmd = &cobra.Command{
	Use:   "enable <name>...",
	Short: "Turn skills on for every project",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setSkillsDisabled(args, false)
	},
}

var skillsDisableCmd = &cobra.Command{
	Use:   "disable <name>...",
	Short: "Turn skills off for every project",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setSkillsDisabled(args, true)
	},
}

func init() {
	skillsCmd.AddCommand(skillsEnableCmd, skillsDisableCmd)
	RootCmd.AddCommand(skillsCmd)
}

// skillDisabledLists returns the fleet's and the project's disabled skills.
func skillDisabledLists(projectDir string) (fleet, project []string) {
	if cfg != nil {
		fleet = cfg.Skills.Disabled
	}
	if projectDir != "" {
		if manifest, err := config.LoadProjectManifest(projectDir); err == nil {
			project = manifest.Skills.Disabled
		}
	}
	return fleet, project
}

func buildSkillsView(projectDir string) string {
	var b strings.Builder
	list := skills.Discover(projectDir)
	fleetOff, projectOff := skillDisabledLists(projectDir)

	if len(list) == 0 {
		b.WriteString("No skills installed.\n\n")
	} else {
		fmt.Fprintf(&b, "%-24s %-14s %-8s %s\n", "SKILL", "STATE", "SOURCE", "DESCRIPTION")
		for _, s := range list {
			state := "on"
			switch {
			case skills.IsDisabled(s.Name, fleetOff):
				state = "off"
			case skills.IsDisabled(s.Name, projectOff):
				state = "off (project)"
			}
			fmt.Fprintf(&b, "%-24s %-14s %-8s %s\n", s.Name, state, s.Source, oneLineDescription(s.Description, 70))
		}
		b.WriteString("\n")
	}

	b.WriteString("Install a skill by copying (or symlinking) its folder into:\n")
	fmt.Fprintf(&b, "  %s   (every project)\n", skills.GlobalDir())
	if projectDir != "" {
		fmt.Fprintf(&b, "  %s   (this project)\n", skills.ProjectDir(projectDir))
	}
	b.WriteString("Turn one off or on with 'ringmaster skills disable <name>' / 'enable <name>'.\n")
	return b.String()
}

func oneLineDescription(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func setSkillsDisabled(names []string, off bool) error {
	if cfg == nil {
		return errNoConfig()
	}
	projectDir, _ := ResolveProjectDir()
	installed := map[string]bool{}
	for _, s := range skills.Discover(projectDir) {
		installed[s.Name] = true
	}
	for _, name := range names {
		// A skill that is gone can still be taken off the disabled list
		if !installed[name] && !(!off && skills.IsDisabled(name, cfg.Skills.Disabled)) {
			return fmt.Errorf("no skill named %q is installed; run 'ringmaster skills' to see them", name)
		}
	}

	for _, name := range names {
		cfg.Skills.Disabled = skills.SetDisabled(cfg.Skills.Disabled, name, off)
	}
	if err := config.SaveFile(config.ConfigPath(cfgFile), cfg); err != nil {
		return err
	}

	_, projectOff := skillDisabledLists(projectDir)
	for _, name := range names {
		switch {
		case off:
			fmt.Printf("✓ Skill '%s' is off\n", name)
		case skills.IsDisabled(name, projectOff):
			fmt.Printf("✓ Skill '%s' is on for the fleet, but this project's ringmaster.toml still disables it\n", name)
		default:
			fmt.Printf("✓ Skill '%s' is on\n", name)
		}
	}
	return nil
}

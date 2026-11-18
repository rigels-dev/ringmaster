package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
)

var presetsJSON bool

// presetEntry is one preset in `ringmaster presets --json`.
type presetEntry struct {
	Name      string   `json:"name"`
	Allow     []string `json:"allow"`
	AlwaysAsk []string `json:"always_ask"`
	// Source is "builtin" or "fleet" (defined in the fleet config)
	Source string `json:"source"`
	// Active is set when the project uses the preset
	Active bool `json:"active"`
}

type presetsReport struct {
	Presets []presetEntry `json:"presets"`
	// The project's sandbox as it is now
	Allow     []string `json:"allow"`
	AlwaysAsk []string `json:"always_ask"`
	Unknown   []string `json:"unknown,omitempty"`
}

var presetsCmd = &cobra.Command{
	Use:   "presets",
	Short: "List the command presets, and what the project's automatons may run",
	Long: `A preset is a package of commands for one language or tool: what
automatons may run without asking, and what must still be asked about
although an allowed command covers it (npm publish under npm).

Ringmaster has presets for go, python, node, rust and docker; [presets.<name>]
in the fleet config adds more or replaces one. A plan names the presets it
needs, and approving the plan adds them to the project's [sandbox] presets.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := ResolveProjectDir() // presets can be listed outside a project too
		pcfg := cfg
		if projectDir != "" {
			if c, err := projectConfig(projectDir); err == nil {
				pcfg = c
			}
		}
		if pcfg == nil {
			pcfg = config.DefaultConfig()
		}
		report := buildPresetsReport(pcfg)
		if presetsJSON {
			return printJSON(report)
		}
		fmt.Print(buildPresetsView(report))
		return nil
	},
}

func buildPresetsReport(c *config.Config) presetsReport {
	active := map[string]bool{}
	for _, n := range c.Sandbox.Presets {
		active[strings.ToLower(n)] = true
	}
	report := presetsReport{Presets: []presetEntry{}}
	for _, name := range c.PresetNames() {
		p, _ := c.Preset(name)
		source := "builtin"
		if _, own := c.Presets[name]; own {
			source = "fleet"
		}
		report.Presets = append(report.Presets, presetEntry{
			Name: name, Allow: orEmpty(p.Allow), AlwaysAsk: orEmpty(p.AlwaysAsk), Source: source, Active: active[name],
		})
	}
	sb := c.EffectiveSandbox()
	report.Allow, report.AlwaysAsk = orEmpty(sb.Allow), orEmpty(sb.AlwaysAsk)
	report.Unknown = c.UnknownPresets(c.Sandbox.Presets)
	return report
}

func buildPresetsView(r presetsReport) string {
	var sb strings.Builder
	for _, p := range r.Presets {
		mark := " "
		if p.Active {
			mark = "✓"
		}
		fmt.Fprintf(&sb, "%s %-8s %s\n", mark, p.Name, strings.Join(p.Allow, ", "))
		if len(p.AlwaysAsk) > 0 {
			fmt.Fprintf(&sb, "  %-8s always asked: %s\n", "", strings.Join(p.AlwaysAsk, ", "))
		}
	}
	if len(r.Unknown) > 0 {
		fmt.Fprintf(&sb, "\n⚠ Unknown presets in the project: %s\n", strings.Join(r.Unknown, ", "))
	}
	fmt.Fprintf(&sb, "\nThis project's automatons may run: %s\n", strings.Join(r.Allow, ", "))
	fmt.Fprintf(&sb, "and must ask first for: %s\n", strings.Join(r.AlwaysAsk, ", "))
	return sb.String()
}

// stackPreview says what approving a plan with this stack allows.
func stackPreview(c *config.Config, stack []string) string {
	if len(stack) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  Stack: %s\n", strings.Join(stack, ", "))
	for _, name := range stack {
		p, ok := c.Preset(name)
		if !ok {
			fmt.Fprintf(&sb, "    ⚠ %s is not a preset; its commands will be asked about one by one\n", name)
			continue
		}
		fmt.Fprintf(&sb, "    %s: approving allows %s", name, strings.Join(p.Allow, ", "))
		if len(p.AlwaysAsk) > 0 {
			fmt.Fprintf(&sb, "; still asks for %s", strings.Join(p.AlwaysAsk, ", "))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func init() {
	presetsCmd.Flags().BoolVar(&presetsJSON, "json", false, "Print the presets and the project's sandbox as JSON")
	RootCmd.AddCommand(presetsCmd)
}

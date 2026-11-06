package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

// Version is set at build time: go build -ldflags "-X ringmaster/internal/cli.Version=v0.1.0"
var Version = "dev"

var RootCmd = &cobra.Command{
	Use:     "ringmaster",
	Short:   "Ringmaster — Local AI Agent Orchestration Tool (Control Plane)",
	Version: Version,
	Long: `Ringmaster is an open-source, provider-agnostic Control Plane designed
to orchestrate AI agent task workflows with a "cheap-by-default" philosophy.
A smart planner decomposes specifications into DAG tasks executed by
focused, budget-friendly workers in isolated git worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// When run without subcommands, launch interactive UI selector
		return RunInteractiveLauncher()
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "fleet config file (default: ringmaster.toml in the Ringmaster install root)")

	RootCmd.AddCommand(initCmd)
}

func initConfig() {
	// The fleet (providers, agents, routing) always comes from the global config
	// or an explicit --config; a project's ringmaster.toml is only its manifest.
	path := config.ConfigPath(cfgFile)
	if path == "" {
		return
	}
	loaded, err := config.LoadFile(path)
	if err != nil {
		if cfgFile != "" || !os.IsNotExist(errors.Unwrap(err)) {
			fmt.Fprintf(os.Stderr, "Warning: failed to load config from %s: %v\n", path, err)
		}
		return
	}
	cfg = loaded
}

// projectConfig is the fleet config with the project's own budget and sandbox
// overrides (from its ringmaster.toml) applied, for commands that run tasks.
func projectConfig(projectDir string) (*config.Config, error) {
	if cfg == nil {
		return nil, errNoConfig()
	}
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		return nil, err
	}
	return cfg.ForProject(manifest), nil
}

func errNoConfig() error {
	return fmt.Errorf("configuration not loaded: create %s or pass --config", config.ConfigPath(cfgFile))
}

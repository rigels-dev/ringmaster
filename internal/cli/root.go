package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
	"ringmaster/internal/version"
)

var (
	cfgFile string
	cfg     *config.Config
)

var RootCmd = &cobra.Command{
	Use:     "ringmaster",
	Short:   "Ringmaster — Local AI Agent Orchestration Tool (Control Plane)",
	Version: version.Version,
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
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "fleet config file (default: ~/.config/ringmaster/ringmaster.toml)")

	RootCmd.AddCommand(initCmd)
}

func initConfig() {
	// The fleet (providers, agents, routing) always comes from the global config
	// or an explicit --config; a project's ringmaster.toml is only its manifest.
	if cfgFile == "" {
		if moved, err := config.MigrateLegacyConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not move the fleet config to %s: %v\n", config.ConfigDir(), err)
		} else if moved != "" {
			fmt.Fprintf(os.Stderr, "Fleet config copied to %s (from %s); edit it there from now on.\n", moved, config.LegacyConfigPath())
		}
	}
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

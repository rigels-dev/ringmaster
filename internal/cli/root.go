package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var RootCmd = &cobra.Command{
	Use:   "ringmaster",
	Short: "Ringmaster — Local AI Agent Orchestration Tool (Control Plane)",
	Long: `Ringmaster is an open-source, provider-agnostic Control Plane designed
to orchestrate AI agent task workflows with a "cheap-by-default" philosophy.
A smart planner decomposes specifications into DAG tasks executed by
focused, budget-friendly workers in isolated git worktrees.`,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./ringmaster.toml)")

	RootCmd.AddCommand(initCmd)
}

func initConfig() {
	if cfgFile != "" {
		loaded, err := config.LoadFile(cfgFile)
		if err == nil {
			cfg = loaded
		}
	} else {
		found, err := config.FindConfigFile()
		if err == nil {
			loaded, err := config.LoadFile(found)
			if err == nil {
				cfg = loaded
			}
		}
	}
}

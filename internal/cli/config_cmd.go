package cli

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"ringmaster/internal/config"
	"ringmaster/internal/tui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactive TUI to configure providers and model registry",
	Long: `Open an interactive Terminal User Interface (TUI) to add, view,
edit, and remove LLM Providers and Agent Models in the global ringmaster.toml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ConfigPath(cfgFile)
		if path == "" {
			return fmt.Errorf("cannot locate the Ringmaster install root; pass --config")
		}

		loadedCfg := cfg
		if loadedCfg == nil {
			c, err := config.LoadFile(path)
			if err != nil {
				loadedCfg = config.DefaultConfig()
			} else {
				loadedCfg = c
			}
		}

		absPath, _ := filepath.Abs(path)
		model := tui.NewConfigModel(loadedCfg, absPath)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(configCmd)
}

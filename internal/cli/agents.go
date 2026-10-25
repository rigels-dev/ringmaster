package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"ringmaster/internal/llm"
	"ringmaster/internal/registry"
	"ringmaster/internal/storage"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage and inspect agent registry",
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured agents and their roles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded. Please create ringmaster.toml or pass --config")
		}

		reg := registry.NewRegistry(cfg)

		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#DEAA3B")).
			Padding(0, 1)

		cellStyle := lipgloss.NewStyle().
			Padding(0, 1)

		roleMasterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#29F2EC")).
			Bold(true)

		roleWorkerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E05436"))

		fmt.Println("\nRegistered Agents in Ringmaster:")
		fmt.Printf("%-18s %-15s %-32s %-24s %-10s %-12s\n",
			headerStyle.Render("ALIAS"),
			headerStyle.Render("PROVIDER"),
			headerStyle.Render("BASE URL"),
			headerStyle.Render("MODEL"),
			headerStyle.Render("ROLE"),
			headerStyle.Render("TOOL CALLING"),
		)
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#3E4C59")).Render(
			"-------------------------------------------------------------------------------------------------------------------",
		))

		var aliases []string
		for alias := range cfg.Agents {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)

		for _, alias := range aliases {
			a := cfg.Agents[alias]
			p := cfg.Providers[a.Provider]

			roleRendered := roleWorkerStyle.Render(a.Role)
			if a.Role == "master" {
				roleRendered = roleMasterStyle.Render(a.Role)
			}

			toolCallingStr := "no"
			if a.ToolCalling {
				toolCallingStr = "yes"
			}

			fmt.Printf("%-18s %-15s %-32s %-24s %-10s %-12s\n",
				cellStyle.Render(alias),
				cellStyle.Render(a.Provider),
				cellStyle.Render(p.BaseURL),
				cellStyle.Render(a.Model),
				roleRendered,
				cellStyle.Render(toolCallingStr),
			)
		}
		fmt.Println()
		_ = reg
		return nil
	},
}

type AgentCheckResult struct {
	Timestamp string            `json:"timestamp"`
	Results   map[string]string `json:"results"` // alias -> "PASSED" / "FAILED: ..."
}

var agentsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify tool-calling capabilities of configured worker agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded. Please create ringmaster.toml or pass --config")
		}

		reg := registry.NewRegistry(cfg)
		checkResults := AgentCheckResult{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Results:   make(map[string]string),
		}

		dummyTool := llm.ToolDefinition{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "ping",
				Description: "Respond to test ping with pong",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
					},
					"required": []string{"message"},
				},
			},
		}

		fmt.Println("\nChecking Agent Tool Calling capabilities:")

		for alias, a := range cfg.Agents {
			if a.Role != "worker" {
				continue
			}

			provider, agentCfg, err := reg.GetProviderForAgent(alias)
			if err != nil {
				checkResults.Results[alias] = fmt.Sprintf("FAILED: %v", err)
				fmt.Printf("✗ %s (%s): %v\n", alias, a.Model, err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			resp, err := provider.Chat(ctx, agentCfg.Model, llm.ChatRequest{
				Messages: []llm.Message{
					{Role: "user", Content: "Call tool ping with message 'pong' immediately."},
				},
				Tools: []llm.ToolDefinition{dummyTool},
			})
			cancel()

			if err != nil {
				checkResults.Results[alias] = fmt.Sprintf("FAILED: %v", err)
				fmt.Printf("✗ %s (%s): %v\n", alias, a.Model, err)
				continue
			}

			hasPing := false
			for _, tc := range resp.ToolCalls {
				if tc.Function.Name == "ping" {
					hasPing = true
					break
				}
			}

			if hasPing {
				checkResults.Results[alias] = "PASSED"
				fmt.Printf("✓ %s (%s): PASSED (Tool Calling verified)\n", alias, a.Model)
			} else {
				checkResults.Results[alias] = "FAILED: model returned text instead of tool call"
				fmt.Printf("✗ %s (%s): FAILED (No tool call returned)\n", alias, a.Model)
			}
		}

		outDir := ".ringmaster"
		_ = os.MkdirAll(outDir, 0755)
		outPath := filepath.Join(outDir, "agent_check.json")
		if err := storage.WriteJSONAtomic(outPath, checkResults); err != nil {
			return fmt.Errorf("failed to save agent check results: %w", err)
		}
		fmt.Printf("\nSaved check results to %s\n", outPath)
		return nil
	},
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsCheckCmd)
	RootCmd.AddCommand(agentsCmd)
}

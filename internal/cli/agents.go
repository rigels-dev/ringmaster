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
		fmt.Printf("%-18s %-15s %-12s %-30s %-24s %-10s %-12s\n",
			headerStyle.Render("ALIAS"),
			headerStyle.Render("PROVIDER"),
			headerStyle.Render("TYPE"),
			headerStyle.Render("ENDPOINT / BASE URL"),
			headerStyle.Render("MODEL"),
			headerStyle.Render("ROLE"),
			headerStyle.Render("TOOL CALLING"),
		)
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#3E4C59")).Render(
			"----------------------------------------------------------------------------------------------------------------------------------",
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

			pType := p.Type
			if pType == "" {
				pType = "openai"
			}

			baseURLDisplay := p.BaseURL
			if pType == "claude_subscription" || pType == "claude_sub" {
				baseURLDisplay = "local (claude-cli / oauth)"
			}

			fmt.Printf("%-18s %-15s %-12s %-30s %-24s %-10s %-12s\n",
				cellStyle.Render(alias),
				cellStyle.Render(a.Provider),
				cellStyle.Render(pType),
				cellStyle.Render(baseURLDisplay),
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

var agentsPingCmd = &cobra.Command{
	Use:   "ping <alias>",
	Short: "Ping an agent to test connectivity and response latency",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded. Please create ringmaster.toml or pass --config")
		}

		alias := args[0]
		reg := registry.NewRegistry(cfg)
		provider, agentCfg, err := reg.GetProviderForAgent(alias)
		if err != nil {
			return err
		}

		fmt.Printf("Pinging agent '%s' (%s via %s)...\n", alias, agentCfg.Model, agentCfg.Provider)
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := provider.Chat(ctx, agentCfg.Model, llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "user", Content: "Respond with exactly the single word 'PONG'."},
			},
		})
		latency := time.Since(start)

		if err != nil {
			fmt.Printf("✗ Ping failed (%s): %v\n", latency.Round(time.Millisecond), err)
			return err
		}

		fmt.Printf("✓ Ping succeeded (%s)!\n", latency.Round(time.Millisecond))
		fmt.Printf("  Response: %s\n", resp.Content)
		if resp.Usage.TotalTokens > 0 {
			fmt.Printf("  Tokens: %d in, %d out (%d total)\n", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
		}
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
	agentsCmd.AddCommand(agentsPingCmd)
	RootCmd.AddCommand(agentsCmd)
}

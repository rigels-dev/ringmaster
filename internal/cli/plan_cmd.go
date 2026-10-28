package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ringmaster/internal/planner"
	"ringmaster/internal/registry"
	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

var (
	headlessPlan bool
	approvePlan  bool
)

var planCmd = &cobra.Command{
	Use:   "plan [project_name]",
	Short: "Generate, preview, or approve DAG plan for a project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("configuration not loaded. Please create ringmaster.toml or pass --config")
		}

		projectName := ""
		if len(args) > 0 {
			projectName = args[0]
		}

		projectDir, err := ResolveProjectDir(projectName)
		if err != nil {
			return err
		}

		planPath := filepath.Join(projectDir, "plan.json")
		var p *schemas.Plan

		// If plan.json doesn't exist, generate it using master agent
		if _, err := os.Stat(planPath); os.IsNotExist(err) {
			fmt.Printf("Generating plan for '%s' using Master Planner agent...\n", projectName)
			reg := registry.NewRegistry(cfg)
			masterAlias := cfg.Routing.DefaultMaster
			if masterAlias == "" {
				for aName, a := range cfg.Agents {
					if a.Role == "master" {
						masterAlias = aName
						break
					}
				}
			}
			if masterAlias == "" {
				return fmt.Errorf("no master agent configured in ringmaster.toml")
			}

			provider, agentCfg, err := reg.GetProviderForAgent(masterAlias)
			if err != nil {
				return fmt.Errorf("failed resolving provider for master agent '%s': %w", masterAlias, err)
			}

			generatedPlan, usage, err := planner.GeneratePlan(context.Background(), projectDir, cfg, *agentCfg, provider)
			if err != nil {
				return fmt.Errorf("plan generation failed: %w", err)
			}
			p = generatedPlan
			fmt.Printf("✓ Plan generated successfully! (Tokens used: %d)\n", usage.TokensTotal())
		} else {
			loadedPlan, err := storage.ReadJSON[schemas.Plan](planPath)
			if err != nil {
				return fmt.Errorf("failed reading %s: %w", planPath, err)
			}
			p = loadedPlan
		}

		// Check if called with --approve flag
		if approvePlan {
			hash, err := planner.ApprovePlan(projectDir, p)
			if err != nil {
				return fmt.Errorf("failed to approve plan: %w", err)
			}
			fmt.Printf("✓ Plan APPROVED successfully!\n  Canonical Hash: %s\n  Saved to: %s\n",
				hash, filepath.Join(projectDir, "plan.approved.json"))
			return nil
		}

		// Check if called with --headless flag (for Web UI / API consumption)
		if headlessPlan {
			fmt.Println("✓ Plan generated and ready for approval.")
			return nil
		}

		// Render interactive preview gate
		estimates := planner.CalculateEstimates(p, cfg.Estimates)
		fmt.Print(planner.RenderPreview(p, estimates, cfg.Budget))

		// Check approval status
		isApproved, _ := planner.VerifyPlanApproval(projectDir, p)
		if isApproved {
			fmt.Println("★ Status: APPROVED (Ready for execution)")
		} else {
			fmt.Println("⚠ Status: PENDING APPROVAL")
		}

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("\nAction: [y] approve · [a] assign worker · [e] edit in $EDITOR · [q] quit > ")
			input, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "y":
				hash, err := planner.ApprovePlan(projectDir, p)
				if err != nil {
					return fmt.Errorf("failed to approve plan: %w", err)
				}
				fmt.Printf("✓ Plan APPROVED successfully!\n  Canonical Hash: %s\n  Saved to: %s\n",
					hash, filepath.Join(projectDir, "plan.approved.json"))
				return nil

			case "a":
				fmt.Print("Enter Task ID to assign (e.g. T01): ")
				tID, _ := reader.ReadString('\n')
				tID = strings.TrimSpace(tID)

				taskFound := false
				for i := range p.Tasks {
					if strings.EqualFold(p.Tasks[i].ID, tID) {
						taskFound = true
						fmt.Printf("Current worker for %s: %s\nNew worker alias: ", p.Tasks[i].ID, p.Tasks[i].Worker)
						wAlias, _ := reader.ReadString('\n')
						wAlias = strings.TrimSpace(wAlias)
						if wAlias != "" {
							p.Tasks[i].Worker = wAlias
							_ = storage.WriteJSONAtomic(planPath, p)
							_ = planner.RevokeApproval(projectDir)
							fmt.Printf("✓ Task %s assigned to worker '%s' (Approval revoked, please re-approve)\n", tID, wAlias)
						}
						break
					}
				}
				if !taskFound {
					fmt.Printf("✗ Task '%s' not found in plan.\n", tID)
				}

			case "e":
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "nano"
				}
				c := exec.Command(editor, planPath)
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				_ = c.Run()

				// Reload plan and revoke approval
				loadedPlan, err := storage.ReadJSON[schemas.Plan](planPath)
				if err == nil {
					p = loadedPlan
					_ = planner.RevokeApproval(projectDir)
					fmt.Println("✓ Plan reloaded from editor (Approval revoked).")
					estimates = planner.CalculateEstimates(p, cfg.Estimates)
					fmt.Print(planner.RenderPreview(p, estimates, cfg.Budget))
				}

			case "q", "exit":
				return nil
			}
		}

		return nil
	},
}

func init() {
	planCmd.Flags().BoolVar(&headlessPlan, "headless", false, "Generate plan without opening interactive terminal prompt")
	planCmd.Flags().BoolVarP(&approvePlan, "approve", "y", false, "Directly approve current plan and generate plan.approved.json")
	RootCmd.AddCommand(planCmd)
}

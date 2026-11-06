package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	Use:   "plan",
	Short: "Generate, preview, or approve DAG plan for a project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := ResolveProjectDir()
		if err != nil {
			return err
		}
		cfg, err := projectConfig(projectDir)
		if err != nil {
			return err
		}
		projectName := filepath.Base(projectDir)

		planPath := filepath.Join(projectDir, ".ringmaster", "plan.json")
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
				hash, filepath.Join(projectDir, ".ringmaster", "plan.approved.json"))
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
		printOpenQuestions(p)

		// Check approval status
		isApproved, _ := planner.VerifyPlanApproval(projectDir, p)
		if isApproved {
			fmt.Println("★ Status: APPROVED (Ready for execution)")
		} else {
			fmt.Println("⚠ Status: PENDING APPROVAL")
		}

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("\nAction: [y] approve · [o] answer open questions · [a] assign worker · [e] edit in $EDITOR · [q] quit > ")
			input, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "y":
				hash, err := planner.ApprovePlan(projectDir, p)
				if err != nil {
					fmt.Printf("✗ Cannot approve: %v\n", err)
					continue
				}
				fmt.Printf("✓ Plan APPROVED successfully!\n  Canonical Hash: %s\n  Saved to: %s\n",
					hash, filepath.Join(projectDir, ".ringmaster", "plan.approved.json"))
				return nil

			case "o":
				for i := range p.OpenQuestions {
					q := &p.OpenQuestions[i]
					fmt.Printf("\n%s: %s\n", q.ID, q.Question)
					for n, opt := range q.Options {
						marker := ""
						if opt == q.Recommended {
							marker = " (recommended)"
						}
						fmt.Printf("  [%d] %s%s\n", n+1, opt, marker)
					}
					if q.Answer != "" {
						fmt.Printf("  Current answer: %s\n", q.Answer)
					}
					fmt.Print("Answer (number, free text, or empty to keep): ")
					ans, _ := reader.ReadString('\n')
					ans = strings.TrimSpace(ans)
					if ans == "" {
						continue
					}
					if n, err := strconv.Atoi(ans); err == nil && n >= 1 && n <= len(q.Options) {
						ans = q.Options[n-1]
					}
					q.Answer = ans
				}
				_ = storage.WriteJSONAtomic(planPath, p)
				_ = planner.RevokeApproval(projectDir)
				printOpenQuestions(p)

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

// printOpenQuestions lists the master's open questions and assumptions, if any.
func printOpenQuestions(p *schemas.Plan) {
	if len(p.OpenQuestions) == 0 && len(p.Assumptions) == 0 {
		return
	}
	fmt.Println("\n❓ Open questions (answer all before approving):")
	for _, q := range p.OpenQuestions {
		answer := "UNANSWERED"
		if q.Answer != "" {
			answer = "→ " + q.Answer
		}
		fmt.Printf("   %s %s  %s\n", q.ID, q.Question, answer)
	}
	for _, a := range p.Assumptions {
		fmt.Printf("   • Assumption: %s\n", a)
	}
}

func init() {
	planCmd.Flags().BoolVar(&headlessPlan, "headless", false, "Generate plan without opening interactive terminal prompt")
	planCmd.Flags().BoolVarP(&approvePlan, "approve", "y", false, "Directly approve current plan and generate plan.approved.json")
	RootCmd.AddCommand(planCmd)
}

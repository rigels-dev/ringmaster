package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ringmaster/internal/config"
	"ringmaster/internal/llm"
	"ringmaster/internal/schemas"
	"ringmaster/internal/skills"
	"ringmaster/internal/storage"
	"ringmaster/internal/verify"
)

// GeneratePlan reads BRIEF.md, calls the master agent, validates Plan schema,
// and writes plan.json + tasks/Txx-*.md files.
func GeneratePlan(
	ctx context.Context,
	projectRoot string,
	cfg *config.Config,
	masterAgent config.AgentConfig,
	provider llm.Provider,
) (*schemas.Plan, schemas.Usage, error) {
	briefPath := filepath.Join(projectRoot, ".ringmaster", "BRIEF.md")
	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		return nil, schemas.Usage{}, fmt.Errorf("failed to read %s: %w", briefPath, err)
	}

	projectName := filepath.Base(projectRoot)
	active := make(map[string]bool)
	for _, alias := range ActiveWorkers(cfg) {
		active[alias] = true
	}

	// Prefer the configured default, then the fallback, then the first enabled worker
	defaultWorker := ""
	for _, candidate := range []string{cfg.Routing.DefaultWorker, cfg.Routing.FallbackWorker} {
		if active[candidate] {
			defaultWorker = candidate
			break
		}
	}
	if defaultWorker == "" {
		if workers := ActiveWorkers(cfg); len(workers) > 0 {
			defaultWorker = workers[0]
		}
	}

	// The skills workers of this project will get: the fleet's switches plus the project's own
	disabledSkills := cfg.Skills.Disabled
	if manifest, mErr := config.LoadProjectManifest(projectRoot); mErr == nil {
		disabledSkills = cfg.ForProject(manifest).Skills.Disabled
	}
	systemPrompt := MasterSystemPrompt(cfg, defaultWorker, skills.Enabled(skills.Discover(projectRoot), disabledSkills))
	userPrompt := fmt.Sprintf("Project Name: %s\n\n# BRIEF.MD:\n%s\n\nGenerate the complete project DAG Plan JSON now.", projectName, string(briefBytes))

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var totalUsage schemas.Usage
	var plan *schemas.Plan

	// The master gets its mistakes back and may correct them: malformed JSON,
	// an invalid DAG, or acceptance commands that cannot run.
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := provider.Chat(ctx, masterAgent.Model, llm.ChatRequest{
			Messages: messages,
		})
		if err != nil {
			return nil, totalUsage, fmt.Errorf("master planning LLM call failed: %w", err)
		}

		totalUsage.Calls++
		totalUsage.TokensIn += resp.Usage.PromptTokens
		totalUsage.TokensCached += resp.Usage.CachedTokens
		totalUsage.CostUSD += resp.Usage.CostUSD
		totalUsage.TokensOut += resp.Usage.CompletionTokens

		content := strings.TrimSpace(resp.Content)
		// Clean markdown code block if present
		if strings.HasPrefix(content, "```") {
			lines := strings.Split(content, "\n")
			if len(lines) >= 2 {
				content = strings.Join(lines[1:len(lines)-1], "\n")
			}
		}

		type rawTask struct {
			schemas.PlanTask
			Description string `json:"description"`
		}
		var raw struct {
			Project       string                 `json:"project"`
			Tasks         []rawTask              `json:"tasks"`
			OpenQuestions []schemas.PlanQuestion `json:"open_questions"`
			Assumptions   []string               `json:"assumptions"`
		}

		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			if attempt < maxAttempts {
				// Retry with error feedback
				messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Your JSON output failed to parse: %v. Please return ONLY a valid raw JSON object matching the schema.", err),
				})
				continue
			}
			return nil, totalUsage, fmt.Errorf("failed to parse plan JSON: %w (raw response: %s)", err, content)
		}

		if raw.Project == "" {
			raw.Project = projectName
		}

		p := schemas.Plan{
			Project:       raw.Project,
			Tasks:         make([]schemas.PlanTask, len(raw.Tasks)),
			OpenQuestions: raw.OpenQuestions,
			Assumptions:   raw.Assumptions,
		}
		// The master proposes questions; answers only ever come from the user
		for i := range p.OpenQuestions {
			p.OpenQuestions[i].Answer = ""
			if p.OpenQuestions[i].ID == "" {
				p.OpenQuestions[i].ID = fmt.Sprintf("Q%d", i+1)
			}
		}

		for i, rt := range raw.Tasks {
			if rt.MaxRetries <= 0 {
				rt.MaxRetries = 2
			}
			// Replace missing, unknown or disabled workers with the default
			if !active[rt.Worker] {
				rt.Worker = defaultWorker
			}
			raw.Tasks[i] = rt
			p.Tasks[i] = rt.PlanTask
		}

		if err := schemas.ValidatePlan(&p); err != nil {
			if attempt < maxAttempts {
				messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Plan validation failed: %v. Please fix the DAG dependencies and return valid JSON.", err),
				})
				continue
			}
			return nil, totalUsage, err
		}

		// Caught here, the master rewrites the command. Caught at run time, it
		// costs the user a finished attempt that can never be accepted.
		if problems := unrunnableAcceptance(p.Tasks); len(problems) > 0 {
			if attempt < maxAttempts {
				messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
				messages = append(messages, llm.Message{
					Role: "user",
					Content: "These acceptance commands cannot run:\n" + strings.Join(problems, "\n") +
						"\n\nAcceptance commands are executed directly, NOT through a shell. Replace each of them with ONE plain command " +
						"with space-separated arguments: no quotes, no $(...), backticks, pipes, &&, ||, ;, or redirects. " +
						"If a check needs logic, have the task add a test file and run that (e.g. \"pytest -q\", \"go test ./...\", \"npm test\"). " +
						"Return the complete corrected JSON.",
				})
				continue
			}
			return nil, totalUsage, fmt.Errorf("the master kept writing acceptance commands that cannot run:\n%s\n"+
				"Generate the plan again, or tell the master in the brief to use plain commands such as 'pytest -q'",
				strings.Join(problems, "\n"))
		}

		plan = &p

		// Write plan.json atomically
		planPath := filepath.Join(projectRoot, ".ringmaster", "plan.json")
		if err := storage.WriteJSONAtomic(planPath, plan); err != nil {
			return nil, totalUsage, fmt.Errorf("failed to write plan.json: %w", err)
		}

		// Write tasks/Txx-title.md files
		tasksDir := filepath.Join(projectRoot, ".ringmaster", "tasks")
		_ = os.MkdirAll(tasksDir, 0755)

		for _, rt := range raw.Tasks {
			spec := schemas.TaskSpec{
				ID:          rt.ID,
				Title:       rt.Title,
				Type:        rt.Type,
				DependsOn:   rt.DependsOn,
				Worker:      rt.Worker,
				MaxRetries:  rt.MaxRetries,
				Acceptance:  rt.Acceptance,
				Description: rt.Description,

				AcceptanceTimeout: rt.AcceptanceTimeout,
			}
			timeoutLine := ""
			if spec.AcceptanceTimeout > 0 {
				timeoutLine = fmt.Sprintf("acceptance_timeout: %d\n", spec.AcceptanceTimeout)
			}

			mdContent := fmt.Sprintf(`---
id: %s
title: %q
type: %s
depends_on: %v
worker: %q
max_retries: %d
acceptance:
%s
%s---

%s
`, spec.ID, spec.Title, spec.Type, formatSliceYAML(spec.DependsOn), spec.Worker, spec.MaxRetries, formatListYAML(spec.Acceptance), timeoutLine, spec.Description)

			slug := strings.ToLower(strings.ReplaceAll(spec.Title, " ", "-"))
			slug = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
					return r
				}
				return -1
			}, slug)

			taskFileName := fmt.Sprintf("%s-%s.md", spec.ID, slug)
			if slug == "" {
				taskFileName = fmt.Sprintf("%s.md", spec.ID)
			}

			_ = os.WriteFile(filepath.Join(tasksDir, taskFileName), []byte(mdContent), 0644)
		}

		break
	}

	return plan, totalUsage, nil
}

// unrunnableAcceptance lists, one line per command, the acceptance commands
// Ringmaster could never run.
func unrunnableAcceptance(tasks []schemas.PlanTask) []string {
	var problems []string
	for _, t := range tasks {
		for _, command := range t.Acceptance {
			if why := verify.UnrunnableReason(strings.TrimSpace(command)); why != "" {
				problems = append(problems, fmt.Sprintf("- %s: %s\n  %s", t.ID, command, why))
			}
		}
	}
	return problems
}

func formatSliceYAML(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func formatListYAML(items []string) string {
	var sb strings.Builder
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("  - %q\n", it))
	}
	return strings.TrimRight(sb.String(), "\n")
}

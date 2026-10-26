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
	"ringmaster/internal/storage"
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
	briefPath := filepath.Join(projectRoot, "BRIEF.md")
	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		return nil, schemas.Usage{}, fmt.Errorf("failed to read %s: %w", briefPath, err)
	}

	projectName := filepath.Base(projectRoot)
	defaultWorker := cfg.Routing.DefaultWorker
	if defaultWorker == "" {
		for aName, a := range cfg.Agents {
			if a.Role == "worker" {
				defaultWorker = aName
				break
			}
		}
	}

	systemPrompt := MasterSystemPrompt(cfg, defaultWorker)
	userPrompt := fmt.Sprintf("Project Name: %s\n\n# BRIEF.MD:\n%s\n\nGenerate the complete project DAG Plan JSON now.", projectName, string(briefBytes))

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var totalUsage schemas.Usage
	var plan *schemas.Plan

	// Attempt generation (with 1 self-repair retry if JSON is malformed)
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := provider.Chat(ctx, masterAgent.Model, llm.ChatRequest{
			Messages: messages,
		})
		if err != nil {
			return nil, totalUsage, fmt.Errorf("master planning LLM call failed: %w", err)
		}

		totalUsage.Calls++
		totalUsage.TokensIn += resp.Usage.PromptTokens
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
			Project string    `json:"project"`
			Tasks   []rawTask `json:"tasks"`
		}

		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			if attempt == 1 {
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
			Project: raw.Project,
			Tasks:   make([]schemas.PlanTask, len(raw.Tasks)),
		}

		for i, rt := range raw.Tasks {
			if rt.MaxRetries <= 0 {
				rt.MaxRetries = 2
			}
			if rt.Worker == "" {
				rt.Worker = defaultWorker
			}
			p.Tasks[i] = rt.PlanTask
		}

		if err := schemas.ValidatePlan(&p); err != nil {
			if attempt == 1 {
				messages = append(messages, llm.Message{Role: "assistant", Content: resp.Content})
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Plan validation failed: %v. Please fix the DAG dependencies and return valid JSON.", err),
				})
				continue
			}
			return nil, totalUsage, err
		}

		plan = &p

		// Write plan.json atomically
		planPath := filepath.Join(projectRoot, "plan.json")
		if err := storage.WriteJSONAtomic(planPath, plan); err != nil {
			return nil, totalUsage, fmt.Errorf("failed to write plan.json: %w", err)
		}

		// Write tasks/Txx-title.md files
		tasksDir := filepath.Join(projectRoot, "tasks")
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
---

%s
`, spec.ID, spec.Title, spec.Type, formatSliceYAML(spec.DependsOn), spec.Worker, spec.MaxRetries, formatListYAML(spec.Acceptance), spec.Description)

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

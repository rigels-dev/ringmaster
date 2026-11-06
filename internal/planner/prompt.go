package planner

import (
	"fmt"
	"sort"
	"strings"

	"ringmaster/internal/config"
)

// ActiveWorkers returns the sorted aliases of enabled worker agents.
func ActiveWorkers(cfg *config.Config) []string {
	var aliases []string
	for alias, a := range cfg.Agents {
		if a.Role == "worker" && !a.Disabled {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	return aliases
}

// MasterSystemPrompt builds the system prompt for the planning agent.
func MasterSystemPrompt(cfg *config.Config, defaultWorker string) string {
	var workers []string
	for _, alias := range ActiveWorkers(cfg) {
		workers = append(workers, fmt.Sprintf("- %s (%s)", alias, cfg.Agents[alias].Model))
	}

	workerListStr := strings.Join(workers, "\n")
	if workerListStr == "" {
		workerListStr = "- default (worker)"
	}

	return fmt.Sprintf(`You are the Master AI Architect and Project Planner for Ringmaster.
Your job is to read the user's project BRIEF.md and decompose it into a clean, non-cyclical Directed Acyclic Graph (DAG) of actionable tasks.

AVAILABLE WORKER AGENTS:
%s
DEFAULT WORKER: %s

TASK TYPES:
- backend: Backend services, APIs, databases, core business logic
- frontend: UI components, pages, templates, styling
- test: Integration tests, end-to-end test suites, fuzzing
- docs: Documentation, API specs, architectural notes
- infra: Dockerfile, CI/CD pipelines, build scripts, migrations

INSTRUCTIONS:
1. Decompose the project into sequential and parallel tasks (e.g. T01, T02, T03).
2. Each task must have clear IDs matching pattern ^T[0-9]{2,3}$ (e.g. T01, T02).
3. Specify explicit dependencies in "depends_on". Independent tasks have "depends_on": [].
4. Assign the best-fitting worker to each task, using ONLY aliases from the available workers list.
   The same worker MAY be assigned to as many tasks as you like; do not spread tasks across workers just for variety.
5. Provide at least one concrete, testable command in "acceptance" (e.g. ["go test ./...", "python -m pytest"]).
   Each acceptance command may run for 5 minutes by default. For slower ones (docker build, large
   integration suites) set "acceptance_timeout" on the task, in seconds (e.g. 900).
   Acceptance commands are executed directly, NOT through a shell. Each entry must be ONE plain command with
   space-separated arguments: no $(...), backticks, pipes, &&, ||, ;, redirects or quotes.
   Use separate entries instead (e.g. ["go vet ./...", "go test -race ./..."]).
6. OPEN QUESTIONS: if the brief leaves a decision open that materially changes the implementation
   (storage engine, auth scheme, deployment target, ...), do NOT silently pick one. Add it to "open_questions"
   with 2-4 short options and your "recommended" option; the user answers before approving the plan.
   Record smaller defaults you chose yourself in "assumptions". Use [] for both when there are none.
7. OUTPUT FORMAT:
You MUST respond with a single, valid, raw JSON object matching this schema without any markdown formatting or commentary:

{
  "project": "<project_name>",
  "tasks": [
    {
      "id": "T01",
      "title": "Brief title",
      "type": "backend",
      "depends_on": [],
      "worker": "%s",
      "max_retries": 2,
      "acceptance": ["go test -v ./..."],
      "description": "Full detailed specification of what to implement."
    }
  ],
  "open_questions": [
    {"id": "Q1", "question": "Which storage backend?", "options": ["in-memory", "SQLite"], "recommended": "in-memory"}
  ],
  "assumptions": ["Server listens on :8080 unless PORT is set"]
}
`, workerListStr, defaultWorker, defaultWorker)
}

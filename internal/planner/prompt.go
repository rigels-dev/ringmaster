package planner

import (
	"fmt"
	"sort"
	"strings"

	"ringmaster/internal/config"
	"ringmaster/internal/skills"
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
// skillList is the enabled skills workers will have (names and descriptions).
func MasterSystemPrompt(cfg *config.Config, defaultWorker string, skillList []skills.Skill) string {
	var workers []string
	for _, alias := range ActiveWorkers(cfg) {
		workers = append(workers, fmt.Sprintf("- %s (%s)", alias, cfg.Agents[alias].Model))
	}

	// What acceptance commands may start with: the project's sandbox as it
	// is now, and the presets the plan can add to it
	sb := cfg.EffectiveSandbox()
	allowedCommands := strings.Join(append(append([]string{}, sb.Allow...), sb.Approved...), ", ")
	var presetLines []string
	for _, name := range cfg.PresetNames() {
		p, _ := cfg.Preset(name)
		line := fmt.Sprintf("- %s: %s", name, strings.Join(p.Allow, ", "))
		if len(p.AlwaysAsk) > 0 {
			line += fmt.Sprintf(" (always asked: %s)", strings.Join(p.AlwaysAsk, ", "))
		}
		presetLines = append(presetLines, line)
	}
	presetList := strings.Join(presetLines, "\n")
	alwaysAsk := strings.Join(sb.AlwaysAsk, ", ")

	workerListStr := strings.Join(workers, "\n")
	if workerListStr == "" {
		workerListStr = "- default (worker)"
	}

	// Workers get every enabled skill anyway; the planner only needs to know
	// they exist, so it can point a task at the one that applies
	skillsSection := ""
	if len(skillList) > 0 {
		var lines []string
		for _, s := range skillList {
			line := "- " + s.Name
			if s.Description != "" {
				line += ": " + s.Description
			}
			lines = append(lines, line)
		}
		skillsSection = "\nINSTALLED SKILLS (instructions every worker can read in its workspace):\n" + strings.Join(lines, "\n") +
			"\nWhen a task is the kind of work a skill covers, say so in its description (e.g. \"Follow the go-style skill.\").\n" +
			"Do not restate a skill's content, and do not invent skills that are not listed.\n"
	}

	return fmt.Sprintf(`You are the Master AI Architect and Project Planner for Ringmaster.
Your job is to read the user's project BRIEF.md and decompose it into a clean, non-cyclical Directed Acyclic Graph (DAG) of actionable tasks.

AVAILABLE WORKER AGENTS:
%s
DEFAULT WORKER: %s
%s
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
   Acceptance commands are executed directly, NOT through a shell. Each entry must be ONE command:
   no pipes, &&, ||, ;, redirects, $(...) or backticks. Use separate entries instead
   (e.g. ["go vet ./...", "go test -race ./..."]).
   An argument that contains spaces MUST be quoted, or it is split into several arguments:
   write python -c "import mypkg", never python -c import mypkg. Nothing inside quotes is expanded.
   Prefer a command that runs the project's tests over inline code.
   ALLOWED COMMANDS: the project allows these commands now: %s.
   A plan adds the command PRESETS it needs (see STACK below); their commands are allowed once the user
   approves the plan. Any other command stops the task until the user approves it, and so do these,
   even where an allowed command covers them: %s.
   Use a command outside the allowed ones and the presets only when nothing else can check the work;
   a test run usually can (a test command also compiles the code, so a separate build check is rarely needed).
   EXISTING TESTS are protected: a worker may add test files, but any change it makes to a test file that
   already exists in the repository is undone before the acceptance commands run. If a task must change
   existing tests because the intended behaviour changes, set "edits_tests": true on that task (omit it
   otherwise). Tasks of type "test" may always change tests.
6. OPEN QUESTIONS: if the brief leaves a decision open that materially changes the implementation
   (storage engine, auth scheme, deployment target, ...), do NOT silently pick one. Add it to "open_questions"
   with 2-4 short options and your "recommended" option; the user answers before approving the plan.
   Record smaller defaults you chose yourself in "assumptions". Use [] for both when there are none.
7. BRANCH: give every task a "branch", named the way people name branches by hand: <prefix>/<slug>.
   The prefix is one of feat, fix, docs, test, refactor, chore. The slug is two to four lower-case words
   joined by dashes that say what the change is (feat/link-store, fix/empty-file-exit-code, docs/usage).
   Each task needs a different branch.
8. EARLIER WORK: when the user's message lists tasks that are ALREADY DONE, their code is already in the
   repository. Do not plan them again. Plan only what is still missing, number your tasks from the first
   free ID you are given, and let "depends_on" name only tasks of this new plan.
9. STACK: list in "stack" the presets this plan's tasks and acceptance commands need, by name, from:
%s
   Name only presets from this list, and only ones the plan really uses (a Go service with a Dockerfile:
   ["go", "docker"]). The user sees them before approving.
10. OUTPUT FORMAT:
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
      "branch": "feat/short-slug",
      "description": "Full detailed specification of what to implement."
    }
  ],
  "stack": ["go"],
  "open_questions": [
    {"id": "Q1", "question": "Which storage backend?", "options": ["in-memory", "SQLite"], "recommended": "in-memory"}
  ],
  "assumptions": ["Server listens on :8080 unless PORT is set"]
}
`, workerListStr, defaultWorker, skillsSection, allowedCommands, alwaysAsk, presetList, defaultWorker)
}

package schemas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type PlanTask struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Type       TaskType `json:"type"`
	DependsOn  []string `json:"depends_on"`
	Worker     string   `json:"worker"`
	MaxRetries int      `json:"max_retries"`
	Acceptance []string `json:"acceptance"`
	// AcceptanceTimeout overrides how long each acceptance command may run, in seconds
	AcceptanceTimeout int `json:"acceptance_timeout,omitempty"`
}

// PlanQuestion is a decision the master could not settle from the brief; it must be
// answered before the plan can be approved.
type PlanQuestion struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	Recommended string   `json:"recommended,omitempty"`
	Answer      string   `json:"answer,omitempty"`
}

type Plan struct {
	Project       string         `json:"project"`
	Tasks         []PlanTask     `json:"tasks"`
	OpenQuestions []PlanQuestion `json:"open_questions,omitempty"`
	Assumptions   []string       `json:"assumptions,omitempty"`
}

// UnansweredQuestions returns the open questions that still need an answer.
func (p *Plan) UnansweredQuestions() []PlanQuestion {
	var res []PlanQuestion
	for _, q := range p.OpenQuestions {
		if strings.TrimSpace(q.Answer) == "" {
			res = append(res, q)
		}
	}
	return res
}

// DecisionsMarkdown renders answered questions and assumptions for worker context.
func (p *Plan) DecisionsMarkdown() string {
	var sb strings.Builder
	for _, q := range p.OpenQuestions {
		if strings.TrimSpace(q.Answer) != "" {
			sb.WriteString(fmt.Sprintf("- %s → %s\n", q.Question, q.Answer))
		}
	}
	for _, a := range p.Assumptions {
		sb.WriteString(fmt.Sprintf("- Assumption: %s\n", a))
	}
	if sb.Len() == 0 {
		return ""
	}
	return "# DECISIONS CONFIRMED BY THE USER:\n" + sb.String()
}

func ValidatePlan(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan must contain at least one task")
	}

	taskMap := make(map[string]PlanTask)
	for _, t := range plan.Tasks {
		if _, exists := taskMap[t.ID]; exists {
			return fmt.Errorf("duplicate task ID '%s' in plan", t.ID)
		}
		taskMap[t.ID] = t
	}

	// Validate dependencies exist
	for _, t := range plan.Tasks {
		for _, dep := range t.DependsOn {
			if _, exists := taskMap[dep]; !exists {
				return fmt.Errorf("task '%s' depends on unknown task '%s'", t.ID, dep)
			}
			if dep == t.ID {
				return fmt.Errorf("task '%s' has self-dependency cycle", t.ID)
			}
		}
	}

	// Cycle detection using Kahn's algorithm
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, t := range plan.Tasks {
		inDegree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			adj[dep] = append(adj[dep], t.ID)
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visitedCount != len(plan.Tasks) {
		var cycleNodes []string
		for id, deg := range inDegree {
			if deg > 0 {
				cycleNodes = append(cycleNodes, id)
			}
		}
		sort.Strings(cycleNodes)
		return fmt.Errorf("dependency cycle detected among tasks: %v", cycleNodes)
	}

	return nil
}

// TopologicalLevels returns groups of tasks that can run in parallel level-by-level
func (p *Plan) TopologicalLevels() ([][]string, error) {
	if err := ValidatePlan(p); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, t := range p.Tasks {
		inDegree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			adj[dep] = append(adj[dep], t.ID)
		}
	}

	var currentLevel []string
	for id, deg := range inDegree {
		if deg == 0 {
			currentLevel = append(currentLevel, id)
		}
	}
	sort.Strings(currentLevel)

	var levels [][]string
	for len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
		var nextLevel []string

		for _, node := range currentLevel {
			for _, neighbor := range adj[node] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextLevel = append(nextLevel, neighbor)
				}
			}
		}

		sort.Strings(nextLevel)
		currentLevel = nextLevel
	}

	return levels, nil
}

// PlanHash computes a stable canonical SHA-256 hash of the plan
func (p *Plan) PlanHash() (string, error) {
	// Normalize task order by ID for stability
	tasksCopy := make([]PlanTask, len(p.Tasks))
	copy(tasksCopy, p.Tasks)

	sort.Slice(tasksCopy, func(i, j int) bool {
		return tasksCopy[i].ID < tasksCopy[j].ID
	})

	for i := range tasksCopy {
		sort.Strings(tasksCopy[i].DependsOn)
		sort.Strings(tasksCopy[i].Acceptance)
	}

	// Questions and assumptions are omitted when empty, so older plan hashes stay stable
	normalized := struct {
		Project       string         `json:"project"`
		Tasks         []PlanTask     `json:"tasks"`
		OpenQuestions []PlanQuestion `json:"open_questions,omitempty"`
		Assumptions   []string       `json:"assumptions,omitempty"`
	}{
		Project:       p.Project,
		Tasks:         tasksCopy,
		OpenQuestions: p.OpenQuestions,
		Assumptions:   p.Assumptions,
	}

	canonicalJSON, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}

	h := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(h[:]), nil
}

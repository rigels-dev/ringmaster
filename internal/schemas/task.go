package schemas

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var taskIDRegex = regexp.MustCompile(`^T\d{2,3}$`)

type TaskType string

const (
	TaskTypeBackend  TaskType = "backend"
	TaskTypeFrontend TaskType = "frontend"
	TaskTypeTest     TaskType = "test"
	TaskTypeDocs     TaskType = "docs"
	TaskTypeInfra    TaskType = "infra"
)

func (t TaskType) IsValid() bool {
	switch t {
	case TaskTypeBackend, TaskTypeFrontend, TaskTypeTest, TaskTypeDocs, TaskTypeInfra:
		return true
	default:
		return false
	}
}

type TaskSpec struct {
	ID         string   `yaml:"id" json:"id"`
	Title      string   `yaml:"title" json:"title"`
	Type       TaskType `yaml:"type" json:"type"`
	DependsOn  []string `yaml:"depends_on" json:"depends_on"`
	Worker     string   `yaml:"worker" json:"worker"`
	MaxRetries int      `yaml:"max_retries" json:"max_retries"`
	Acceptance []string `yaml:"acceptance" json:"acceptance"`
	// AcceptanceTimeout overrides how long each acceptance command may run, in seconds
	AcceptanceTimeout int    `yaml:"acceptance_timeout,omitempty" json:"acceptance_timeout,omitempty"`
	Description       string `yaml:"-" json:"description"`
	HumanAnswers      string `yaml:"-" json:"human_answers,omitempty"`
	// EditsTests comes from the approved plan only, never from the task file
	EditsTests bool `yaml:"-" json:"edits_tests,omitempty"`
}

func ValidateTaskSpec(spec *TaskSpec) error {
	if !taskIDRegex.MatchString(spec.ID) {
		return fmt.Errorf("invalid task id '%s': must match pattern ^T\\d{2,3}$", spec.ID)
	}
	if !spec.Type.IsValid() {
		return fmt.Errorf("invalid task type '%s': must be backend, frontend, test, docs, or infra", spec.Type)
	}
	if len(spec.Acceptance) == 0 {
		return fmt.Errorf("task '%s' must have at least one acceptance criterion", spec.ID)
	}
	if spec.MaxRetries < 0 || spec.MaxRetries > 5 {
		return fmt.Errorf("task '%s' max_retries must be between 0 and 5, got %d", spec.ID, spec.MaxRetries)
	}
	return nil
}

// ParseTaskMarkdown parses a markdown file containing YAML frontmatter delimited by ---
func ParseTaskMarkdown(content string) (*TaskSpec, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, fmt.Errorf("missing frontmatter delimiter '---'")
	}

	parts := strings.SplitN(trimmed[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("unterminated frontmatter delimiter '---'")
	}

	frontmatter := parts[0]
	body := strings.TrimSpace(parts[1])

	var spec TaskSpec
	// Default values
	spec.MaxRetries = 2
	spec.DependsOn = []string{}

	if err := yaml.Unmarshal([]byte(frontmatter), &spec); err != nil {
		return nil, fmt.Errorf("failed to parse task frontmatter: %w", err)
	}

	// Extract Human Answers section if present
	if idx := strings.Index(body, "## Human Answers"); idx != -1 {
		spec.Description = strings.TrimSpace(body[:idx])
		spec.HumanAnswers = strings.TrimSpace(body[idx+len("## Human Answers"):])
	} else {
		spec.Description = body
	}

	if err := ValidateTaskSpec(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

// ReplaceAcceptance returns a task file's content with the commands under
// 'acceptance:' in its frontmatter replaced by commands. Everything else in
// the file, the body and the user's earlier answers included, is kept as
// it is.
func ReplaceAcceptance(content string, commands []string) (string, error) {
	if len(commands) == 0 {
		return "", fmt.Errorf("a task needs at least one acceptance command")
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("missing frontmatter delimiter '---'")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", fmt.Errorf("unterminated frontmatter delimiter '---'")
	}

	list := []string{"acceptance:"}
	for _, c := range commands {
		// A JSON string is a YAML double-quoted scalar
		var b strings.Builder
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(c); err != nil {
			return "", err
		}
		list = append(list, "  - "+strings.TrimSpace(b.String()))
	}

	var front []string
	replaced := false
	for i := 1; i < end; i++ {
		line := lines[i]
		if !replaced && (line == "acceptance:" || strings.HasPrefix(line, "acceptance:")) {
			front = append(front, list...)
			replaced = true
			// The old list: its items, indented or not, up to the next key
			for i+1 < end && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t") ||
				strings.HasPrefix(lines[i+1], "-") || strings.TrimSpace(lines[i+1]) == "") {
				i++
			}
			continue
		}
		front = append(front, line)
	}
	if !replaced {
		front = append(front, list...)
	}

	out := append([]string{lines[0]}, front...)
	out = append(out, lines[end:]...)
	result := strings.Join(out, "\n")

	// The file must still read back as the same task with the new commands
	spec, err := ParseTaskMarkdown(result)
	if err != nil {
		return "", fmt.Errorf("the task file would no longer parse: %w", err)
	}
	if !slices.Equal(spec.Acceptance, commands) {
		return "", fmt.Errorf("the task file would not hold the new acceptance commands")
	}
	return result, nil
}

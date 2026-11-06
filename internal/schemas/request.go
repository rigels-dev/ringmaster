package schemas

import "fmt"

type RequestType string

const (
	RequestApproval RequestType = "approval" // run a shell command outside the sandbox allow list
	RequestChoice   RequestType = "choice"   // pick one of a few options
	RequestInput    RequestType = "input"    // a value only the user knows
)

// HumanRequest is something a worker needs from the user before it can continue.
type HumanRequest struct {
	ID          string      `yaml:"id" json:"id"`
	Type        RequestType `yaml:"type" json:"type"`
	Question    string      `yaml:"question,omitempty" json:"question,omitempty"`
	Command     string      `yaml:"command,omitempty" json:"command,omitempty"`
	Reason      string      `yaml:"reason,omitempty" json:"reason,omitempty"`
	Options     []string    `yaml:"options,omitempty" json:"options,omitempty"`
	Recommended string      `yaml:"recommended,omitempty" json:"recommended,omitempty"`
	// Secret inputs must be answered with an environment variable name, never the value.
	Secret bool `yaml:"secret,omitempty" json:"secret,omitempty"`
}

func ValidateHumanRequest(r HumanRequest) error {
	if r.ID == "" {
		return fmt.Errorf("request is missing an id")
	}
	switch r.Type {
	case RequestApproval:
		if r.Command == "" {
			return fmt.Errorf("approval request '%s' must name a command", r.ID)
		}
	case RequestChoice:
		if r.Question == "" || len(r.Options) < 2 {
			return fmt.Errorf("choice request '%s' needs a question and at least two options", r.ID)
		}
	case RequestInput:
		if r.Question == "" {
			return fmt.Errorf("input request '%s' needs a question", r.ID)
		}
	default:
		return fmt.Errorf("request '%s' has invalid type '%s': must be approval, choice, or input", r.ID, r.Type)
	}
	return nil
}

// HumanAnswer is the user's reply to a task waiting on them, queued in
// .ringmaster/answers/<task>.json until the scheduler applies it.
type HumanAnswer struct {
	TaskID string `json:"task_id"`
	// Replies maps a request id to the user's answer for it.
	Replies map[string]string `json:"replies,omitempty"`
	// Allow lists approved commands; Scope is "task" (default) or "project".
	Allow []string `json:"allow,omitempty"`
	Scope string   `json:"scope,omitempty"`
	// Text is free-form guidance, e.g. for BLOCKED or NEEDS_REVIEW tasks.
	Text string `json:"text,omitempty"`
}

// CommandApprovals records commands the user approved beyond the sandbox allow list.
type CommandApprovals struct {
	Project []string            `json:"project"`
	Tasks   map[string][]string `json:"tasks"`
}

// For returns the approved commands that apply to taskID.
func (a *CommandApprovals) For(taskID string) []string {
	if a == nil {
		return nil
	}
	return append(append([]string{}, a.Project...), a.Tasks[taskID]...)
}

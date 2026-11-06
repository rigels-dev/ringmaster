package schemas

import "time"

type TaskExecutionStatus string

const (
	TaskStatePending   TaskExecutionStatus = "PENDING"
	TaskStateRunning   TaskExecutionStatus = "RUNNING"
	TaskStateCompleted TaskExecutionStatus = "COMPLETED"
	TaskStateBlocked   TaskExecutionStatus = "BLOCKED"
	TaskStateFailed    TaskExecutionStatus = "FAILED"
	TaskStateReview    TaskExecutionStatus = "NEEDS_REVIEW"
	TaskStatePaused    TaskExecutionStatus = "PAUSED"
	TaskStateAwaiting  TaskExecutionStatus = "AWAITING_INPUT"
)

type TaskState struct {
	ID       string              `json:"id"`
	Status   TaskExecutionStatus `json:"status"`
	Worker   string              `json:"worker"`
	Attempt  int                 `json:"attempt"`
	Usage    Usage               `json:"usage"`
	Error    string              `json:"error,omitempty"`
	Requests []HumanRequest      `json:"requests,omitempty"`
	StartAt  *time.Time          `json:"start_at,omitempty"`
	FinishAt *time.Time          `json:"finish_at,omitempty"`
}

type ProjectState struct {
	Project      string               `json:"project"`
	PlanHash     string               `json:"plan_hash"`
	Status       string               `json:"status"` // "ACTIVE", "PAUSED", "COMPLETED", "FAILED"
	TaskStates   map[string]TaskState `json:"task_states"`
	Planning     Usage                `json:"planning_usage"`
	UsageByAgent map[string]Usage     `json:"usage_by_agent"`
	UsageByTask  map[string]Usage     `json:"usage_by_task"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

func NewProjectState(project, planHash string) *ProjectState {
	return &ProjectState{
		Project:      project,
		PlanHash:     planHash,
		Status:       "ACTIVE",
		TaskStates:   make(map[string]TaskState),
		UsageByAgent: make(map[string]Usage),
		UsageByTask:  make(map[string]Usage),
		UpdatedAt:    time.Now().UTC(),
	}
}

package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

// StateManager provides synchronized thread-safe access to .ringmaster/state.json.
type StateManager struct {
	projectRoot string
	statePath   string
	mu          sync.Mutex
	state       *schemas.ProjectState
}

// NewStateManager loads or initializes the project state file.
func NewStateManager(projectRoot, planHash string) (*StateManager, error) {
	stateDir := filepath.Join(projectRoot, ".ringmaster")
	_ = os.MkdirAll(stateDir, 0755)
	statePath := filepath.Join(stateDir, "state.json")

	sm := &StateManager{
		projectRoot: projectRoot,
		statePath:   statePath,
	}

	loaded, err := storage.ReadJSON[schemas.ProjectState](statePath)
	if err == nil && loaded != nil {
		sm.state = loaded
		// Auto-reset any dangling RUNNING task back to PENDING (from unclean crash/poweroff)
		for id, ts := range sm.state.TaskStates {
			if ts.Status == schemas.TaskStateRunning {
				ts.Status = schemas.TaskStatePending
				ts.StartAt = nil
				sm.state.TaskStates[id] = ts
			}
		}
		if sm.state.PlanHash == "" {
			sm.state.PlanHash = planHash
		}
	} else {
		projectName := filepath.Base(projectRoot)
		sm.state = schemas.NewProjectState(projectName, planHash)
	}

	if err := sm.saveLocked(); err != nil {
		return nil, fmt.Errorf("failed initializing state: %w", err)
	}

	return sm, nil
}

// GetState returns a deep copy snapshot of current project state.
func (sm *StateManager) GetState() schemas.ProjectState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	copyState := *sm.state
	copyState.TaskStates = make(map[string]schemas.TaskState, len(sm.state.TaskStates))
	for k, v := range sm.state.TaskStates {
		copyState.TaskStates[k] = v
	}

	copyState.UsageByAgent = make(map[string]schemas.Usage, len(sm.state.UsageByAgent))
	for k, v := range sm.state.UsageByAgent {
		copyState.UsageByAgent[k] = v
	}

	copyState.UsageByTask = make(map[string]schemas.Usage, len(sm.state.UsageByTask))
	for k, v := range sm.state.UsageByTask {
		copyState.UsageByTask[k] = v
	}

	return copyState
}

// UpdateTaskState updates a single task's execution state and persists atomically.
func (sm *StateManager) UpdateTaskState(taskID string, fn func(*schemas.TaskState)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ts, exists := sm.state.TaskStates[taskID]
	if !exists {
		ts = schemas.TaskState{
			ID:     taskID,
			Status: schemas.TaskStatePending,
		}
	}

	fn(&ts)
	sm.state.TaskStates[taskID] = ts
	sm.state.UpdatedAt = time.Now().UTC()

	return sm.saveLocked()
}

// UpdateProject updates global project state fields and persists atomically.
func (sm *StateManager) UpdateProject(fn func(*schemas.ProjectState)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	fn(sm.state)
	sm.state.UpdatedAt = time.Now().UTC()

	return sm.saveLocked()
}

// TotalTokensConsumed calculates cumulative tokens across planning and task executions.
func (sm *StateManager) TotalTokensConsumed() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	total := sm.state.Planning.TokensTotal()
	for _, ts := range sm.state.TaskStates {
		total += ts.Usage.TokensTotal()
	}
	return total
}

func (sm *StateManager) saveLocked() error {
	return storage.WriteJSONAtomic(sm.statePath, sm.state)
}

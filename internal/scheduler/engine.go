package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ringmaster/internal/config"
	"ringmaster/internal/planner"
	"ringmaster/internal/registry"
	"ringmaster/internal/schemas"
	"ringmaster/internal/verify"
	"ringmaster/internal/workspace"
)

const (
	defaultMaxParallel = 4
)

// Engine orchestrates DAG parallel scheduling, worktree assignment, verification, and merge queue.
type Engine struct {
	projectRoot  string
	cfg          *config.Config
	plan         *schemas.Plan
	stateMgr     *StateManager
	mergeQueue   *workspace.MergeQueue
	reg          *registry.Registry
	briefContent string
	eventsPath   string
	maxParallel  int
}

// NewEngine creates and validates an execution engine for a project.
func NewEngine(
	projectRoot string,
	cfg *config.Config,
	maxParallel int,
) (*Engine, error) {
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}

	// 1. Load plan.json
	planPath := filepath.Join(projectRoot, "plan.json")
	plan, err := planner.ReadPlanFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading %s: %w", planPath, err)
	}

	// 2. Validate cryptographic approval against plan.approved.json
	valid, err := planner.VerifyPlanApproval(projectRoot, plan)
	if err != nil || !valid {
		return nil, fmt.Errorf("plan execution rejected: %w", err)
	}

	planHash, _ := plan.PlanHash()

	// 3. Initialize StateManager
	stateMgr, err := NewStateManager(projectRoot, planHash)
	if err != nil {
		return nil, err
	}

	// 4. Read BRIEF.md
	briefPath := filepath.Join(projectRoot, "BRIEF.md")
	briefBytes, _ := os.ReadFile(briefPath)

	eventsPath := filepath.Join(projectRoot, ".ringmaster", "logs", "events.jsonl")

	return &Engine{
		projectRoot:  projectRoot,
		cfg:          cfg,
		plan:         plan,
		stateMgr:     stateMgr,
		mergeQueue:   workspace.NewMergeQueue(projectRoot),
		reg:          registry.NewRegistry(cfg),
		briefContent: string(briefBytes),
		eventsPath:   eventsPath,
		maxParallel:  maxParallel,
	}, nil
}

// Run executes tasks in the DAG up to completion or until paused/interrupted.
func (e *Engine) Run(ctx context.Context) error {
	sem := make(chan struct{}, e.maxParallel)
	var wg sync.WaitGroup

	taskMap := make(map[string]schemas.PlanTask)
	for _, t := range e.plan.Tasks {
		taskMap[t.ID] = t
	}

	for {
		// 1. Check global context cancellation
		if ctx.Err() != nil {
			_ = e.stateMgr.UpdateProject(func(ps *schemas.ProjectState) {
				ps.Status = "PAUSED"
			})
			return ctx.Err()
		}

		// 2. Check project budget
		consumed := e.stateMgr.TotalTokensConsumed()
		if e.cfg.Budget.MaxTokensPerProject > 0 && consumed >= e.cfg.Budget.MaxTokensPerProject {
			_ = e.stateMgr.UpdateProject(func(ps *schemas.ProjectState) {
				ps.Status = "PAUSED"
			})
			return fmt.Errorf("project paused: total tokens (%d) exceeded project budget (%d)",
				consumed, e.cfg.Budget.MaxTokensPerProject)
		}

		state := e.stateMgr.GetState()

		// 3. Mark skipped tasks if upstream failed or blocked
		e.updateSkippedTasks(&state, taskMap)

		// 4. Find all tasks ready to run
		var runnable []schemas.PlanTask
		allDoneOrTerminated := true

		for _, t := range e.plan.Tasks {
			ts, exists := state.TaskStates[t.ID]
			if !exists {
				ts.Status = schemas.TaskStatePending
			}

			if ts.Status == schemas.TaskStatePending {
				allDoneOrTerminated = false
				if e.dependenciesMet(t, state) {
					runnable = append(runnable, t)
				}
			} else if ts.Status == schemas.TaskStateRunning {
				allDoneOrTerminated = false
			}
		}

		if allDoneOrTerminated {
			// All tasks reached a terminal state (COMPLETED, BLOCKED, FAILED, NEEDS_REVIEW, SKIPPED)
			_ = e.stateMgr.UpdateProject(func(ps *schemas.ProjectState) {
				allCompleted := true
				for _, t := range e.plan.Tasks {
					if ps.TaskStates[t.ID].Status != schemas.TaskStateCompleted {
						allCompleted = false
						break
					}
				}
				if allCompleted {
					ps.Status = "COMPLETED"
				} else {
					ps.Status = "HALTED"
				}
			})
			break
		}

		if len(runnable) == 0 {
			// No tasks immediately runnable (waiting on in-flight tasks or upstream failures)
			hasRunning := false
			for _, ts := range state.TaskStates {
				if ts.Status == schemas.TaskStateRunning {
					hasRunning = true
					break
				}
			}
			if !hasRunning {
				// Deadlock / halted because remaining tasks are blocked
				break
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// 5. Dispatch runnable tasks to worker pool
		for _, task := range runnable {
			// Mark RUNNING
			now := time.Now().UTC()
			_ = e.stateMgr.UpdateTaskState(task.ID, func(ts *schemas.TaskState) {
				ts.Status = schemas.TaskStateRunning
				ts.StartAt = &now
				ts.Worker = task.Worker
			})

			fmt.Printf("⚡ [Scheduler] Dispatching task %s to worker automaton '%s'...\n", task.ID, task.Worker)

			sem <- struct{}{}
			wg.Add(1)

			go func(t schemas.PlanTask) {
				defer func() {
					<-sem
					wg.Done()
				}()
				e.executeTask(ctx, t)
			}(task)
		}
	}

	wg.Wait()
	return nil
}

func (e *Engine) executeTask(ctx context.Context, planTask schemas.PlanTask) {
	// 1. Resolve agent & provider
	workerAlias := planTask.Worker
	provider, agentCfg, err := e.reg.GetProviderForAgent(workerAlias)
	if err != nil {
		e.failTask(planTask.ID, fmt.Sprintf("failed resolving worker agent: %v", err))
		return
	}

	// 2. Load task specification markdown from tasks/Txx-*.md
	taskSpec, err := e.loadTaskSpec(planTask)
	if err != nil {
		e.failTask(planTask.ID, fmt.Sprintf("failed reading task spec file: %v", err))
		return
	}

	// 3. Determine baseRef for worktree branch
	baseRef := "main"
	if len(planTask.DependsOn) > 0 {
		// Use most recent upstream branch if applicable
		baseRef = "rm/" + planTask.DependsOn[len(planTask.DependsOn)-1]
	}

	// 4. Create isolated git worktree
	worktree, err := workspace.CreateWorktree(ctx, e.projectRoot, planTask.ID, baseRef)
	if err != nil {
		e.failTask(planTask.ID, fmt.Sprintf("worktree creation failed: %v", err))
		return
	}

	// 5. Execute task attempt loop
	loopRes, err := verify.RunTaskLoop(
		ctx,
		*taskSpec,
		worktree,
		*agentCfg,
		provider,
		e.cfg.Budget,
		e.cfg.Sandbox,
		e.briefContent,
		e.eventsPath,
	)

	now := time.Now().UTC()
	if err != nil {
		e.failTask(planTask.ID, err.Error())
		return
	}

	// Record usage in state
	_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
		ts.Attempt = loopRes.TotalAttempts
		ts.Usage = loopRes.TotalUsage
		ts.FinishAt = &now
	})

	// 6. Handle verification decision
	switch loopRes.Decision.Type {
	case verify.DecisionDONE:
		fmt.Printf("✓ [Scheduler] Task %s passed acceptance tests! Queuing git merge...\n", planTask.ID)
		// Attempt to commit any uncommitted changes in worktree
		_, _ = workspace.RunGit(ctx, worktree, "add", "-A")
		_, _ = workspace.RunGit(ctx, worktree, "commit", "-m", fmt.Sprintf("Complete task %s", planTask.ID))

		// Enqueue to serialized Merge Queue
		mergeRes, mErr := e.mergeQueue.Enqueue(ctx, planTask.ID)
		if mErr != nil || mergeRes == nil || !mergeRes.Merged {
			// Merge conflict encountered -> set to NEEDS_REVIEW
			fmt.Printf("⚠ [Scheduler] Task %s merge conflict! Needs review.\n", planTask.ID)
			_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
				ts.Status = schemas.TaskStateReview
				ts.Error = fmt.Sprintf("Merge conflict: %s", strings.Join(mergeRes.ConflictedFiles, ", "))
			})
			return
		}

		// Merge succeeded -> remove worktree & mark COMPLETED
		fmt.Printf("★ [Scheduler] Task %s successfully merged to main!\n", planTask.ID)
		_ = workspace.RemoveWorktree(ctx, e.projectRoot, planTask.ID)
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStateCompleted
			ts.Error = ""
		})

	case verify.DecisionBLOCKED:
		fmt.Printf("❌ [Scheduler] Task %s reported BLOCKED: %s\n", planTask.ID, loopRes.Decision.Reason)
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStateBlocked
			ts.Error = loopRes.Decision.Reason
		})

	case verify.DecisionNEED_REVIEW:
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStateReview
			ts.Error = loopRes.Decision.Reason
		})

	case verify.DecisionPAUSED:
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStatePending
			ts.Error = loopRes.Decision.Reason
		})
	}
}

func (e *Engine) dependenciesMet(task schemas.PlanTask, state schemas.ProjectState) bool {
	for _, dep := range task.DependsOn {
		ts, exists := state.TaskStates[dep]
		if !exists || ts.Status != schemas.TaskStateCompleted {
			return false
		}
	}
	return true
}

func (e *Engine) updateSkippedTasks(state *schemas.ProjectState, taskMap map[string]schemas.PlanTask) {
	for _, t := range e.plan.Tasks {
		ts, exists := state.TaskStates[t.ID]
		if exists && (ts.Status == schemas.TaskStateCompleted || ts.Status == schemas.TaskStateRunning) {
			continue
		}

		for _, dep := range t.DependsOn {
			upState := state.TaskStates[dep].Status
			if upState == schemas.TaskStateFailed || upState == schemas.TaskStateBlocked || upState == schemas.TaskStateReview {
				_ = e.stateMgr.UpdateTaskState(t.ID, func(cur *schemas.TaskState) {
					cur.Status = schemas.TaskStateFailed
					cur.Error = fmt.Sprintf("Skipped because upstream dependency '%s' is %s", dep, upState)
				})
				break
			}
		}
	}
}

// GetState returns current project state from StateManager.
func (e *Engine) GetState() schemas.ProjectState {
	return e.stateMgr.GetState()
}

// UpdateTaskState updates a single task state directly (used by CLI or human answer).
func (e *Engine) UpdateTaskState(taskID string, fn func(*schemas.TaskState)) error {
	return e.stateMgr.UpdateTaskState(taskID, fn)
}
func (e *Engine) failTask(taskID, errMsg string) {
	now := time.Now().UTC()
	_ = e.stateMgr.UpdateTaskState(taskID, func(ts *schemas.TaskState) {
		ts.Status = schemas.TaskStateFailed
		ts.Error = errMsg
		ts.FinishAt = &now
	})
}

func (e *Engine) loadTaskSpec(t schemas.PlanTask) (*schemas.TaskSpec, error) {
	// Search in tasks/ for matching Txx-*.md file
	tasksDir := filepath.Join(e.projectRoot, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), t.ID) && strings.HasSuffix(entry.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(tasksDir, entry.Name()))
				if err == nil {
					return schemas.ParseTaskMarkdown(string(content))
				}
			}
		}
	}

	// Fallback to minimal task spec from PlanTask
	return &schemas.TaskSpec{
		ID:         t.ID,
		Title:      t.Title,
		Type:       t.Type,
		DependsOn:  t.DependsOn,
		Worker:     t.Worker,
		MaxRetries: t.MaxRetries,
		Acceptance: t.Acceptance,
	}, nil
}

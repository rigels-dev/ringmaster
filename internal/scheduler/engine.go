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
	"ringmaster/internal/skills"
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
	planPath := filepath.Join(projectRoot, ".ringmaster", "plan.json")
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
	briefPath := filepath.Join(projectRoot, ".ringmaster", "BRIEF.md")
	briefBytes, _ := os.ReadFile(briefPath)
	briefContent := string(briefBytes)
	if decisions := plan.DecisionsMarkdown(); decisions != "" {
		briefContent += "\n\n" + decisions
	}

	eventsPath := filepath.Join(projectRoot, ".ringmaster", "logs", "events.jsonl")

	return &Engine{
		projectRoot:  projectRoot,
		cfg:          cfg,
		plan:         plan,
		stateMgr:     stateMgr,
		mergeQueue:   workspace.NewMergeQueue(projectRoot),
		reg:          registry.NewRegistry(cfg),
		briefContent: briefContent,
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
		// Apply answers the user queued for tasks waiting on them
		e.applyQueuedAnswers()

		// 1. Check global context cancellation
		if ctx.Err() != nil {
			wg.Wait()
			_ = e.stateMgr.UpdateProject(func(ps *schemas.ProjectState) {
				ps.Status = "PAUSED"
			})
			return ctx.Err()
		}

		// 2. Check project budget
		consumed := e.stateMgr.TotalTokensConsumed()
		if e.cfg.Budget.MaxTokensPerProject > 0 && consumed >= e.cfg.Budget.MaxTokensPerProject {
			// Let in-flight tasks record their outcome instead of leaving them RUNNING
			wg.Wait()
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
					ps.Status = haltStatus(ps)
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
				// Halted: remaining tasks wait on paused or failed upstream tasks
				_ = e.stateMgr.UpdateProject(func(ps *schemas.ProjectState) {
					ps.Status = haltStatus(ps)
				})
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

	// 5. Give the worker the enabled skills, as files in its workspace
	if err := skills.Sync(worktree, skills.Enabled(skills.Discover(e.projectRoot), e.cfg.Skills.Disabled)); err != nil {
		e.failTask(planTask.ID, fmt.Sprintf("installing skills failed: %v", err))
		return
	}

	// 6. Execute task attempt loop, with commands the user approved added to the sandbox
	sandbox := e.cfg.Sandbox
	sandbox.Allow = append(append([]string{}, e.cfg.Sandbox.Allow...), LoadApprovals(e.projectRoot).For(planTask.ID)...)
	loopRes, err := verify.RunTaskLoop(
		ctx,
		*taskSpec,
		worktree,
		*agentCfg,
		provider,
		e.cfg.Budget,
		sandbox,
		e.briefContent,
		e.eventsPath,
		// One commit per attempt, tagged with the automaton that made it
		func(attempt int, report *schemas.WorkerReport) {
			subject := fmt.Sprintf("%s: attempt %d", planTask.ID, attempt)
			if report != nil {
				if line := firstLine(report.Summary); line != "" {
					subject = fmt.Sprintf("%s: %s", planTask.ID, line)
				}
			}
			if _, cErr := workspace.CommitAll(ctx, worktree, subject, taskTrailers(planTask.ID, workerAlias, attempt)); cErr != nil {
				fmt.Printf("⚠ [Scheduler] Could not commit %s attempt %d: %v\n", planTask.ID, attempt, cErr)
			}
		},
	)

	// Keep the report and transcripts in the project; the worktree is removed after a merge
	if aErr := archiveTaskArtifacts(e.projectRoot, worktree, planTask.ID); aErr != nil {
		fmt.Printf("⚠ [Scheduler] Could not archive %s report/transcripts: %v\n", planTask.ID, aErr)
	}

	now := time.Now().UTC()
	if err != nil {
		e.failTask(planTask.ID, err.Error())
		return
	}

	// Record usage in state
	_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
		ts.Attempt = loopRes.TotalAttempts
		// Accumulate across runs so the project budget sees every resumed run
		ts.Usage.Add(loopRes.TotalUsage)
		ts.FinishAt = &now
	})

	// 7. Handle verification decision
	switch loopRes.Decision.Type {
	case verify.DecisionDONE:
		fmt.Printf("✓ [Scheduler] Task %s passed acceptance tests! Queuing git merge...\n", planTask.ID)
		// Attempt to commit any uncommitted changes in worktree
		_, _ = workspace.CommitAll(ctx, worktree, fmt.Sprintf("Complete task %s", planTask.ID), taskTrailers(planTask.ID, workerAlias, loopRes.TotalAttempts))

		// Enqueue to serialized Merge Queue
		mergeRes, mErr := e.mergeQueue.Enqueue(ctx, planTask.ID)
		if mErr != nil || mergeRes == nil || !mergeRes.Merged {
			// Not merged (conflict, the user's checkout in the way, or a git error) -> NEEDS_REVIEW
			reason := "merge failed"
			switch {
			case mErr != nil:
				reason = fmt.Sprintf("Merge failed: %v", mErr)
			case mergeRes != nil && mergeRes.Conflict:
				reason = fmt.Sprintf("Merge conflict: %s", strings.Join(mergeRes.ConflictedFiles, ", "))
			case mergeRes != nil && mergeRes.Message != "":
				reason = "Merge not applied: " + mergeRes.Message
			}
			fmt.Printf("⚠ [Scheduler] Task %s was not merged. Needs review: %s\n", planTask.ID, reason)
			_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
				ts.Status = schemas.TaskStateReview
				ts.Error = reason
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
		reason := loopRes.Decision.Reason
		if loopRes.Decision.FailureLogs != "" {
			reason += "\n\n" + loopRes.Decision.FailureLogs
		}
		fmt.Printf("⚠ [Scheduler] Task %s needs review: %s\n", planTask.ID, reason)
		fmt.Printf("   Transcript: %s\n", filepath.Join(worktree, ".ringmaster", "logs"))
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStateReview
			ts.Error = reason
		})

	case verify.DecisionAWAITING_INPUT:
		fmt.Printf("🙋 [Scheduler] Task %s needs your input (%d request(s)):\n", planTask.ID, len(loopRes.Decision.Requests))
		for _, r := range loopRes.Decision.Requests {
			if r.Type == schemas.RequestApproval {
				fmt.Printf("   [%s] approve command `%s`: %s\n", r.ID, r.Command, r.Reason)
			} else {
				fmt.Printf("   [%s] %s: %s\n", r.ID, r.Type, r.Question)
			}
		}
		fmt.Println("   Answer in the Web UI task panel, or with 'ringmaster answer'.")
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStateAwaiting
			ts.Error = loopRes.Decision.Reason
			ts.Requests = loopRes.Decision.Requests
		})

	case verify.DecisionPAUSED:
		// PAUSED is terminal for this run; re-queuing it as PENDING would re-dispatch it at once
		fmt.Printf("⏸ [Scheduler] Task %s paused: %s Run 'ringmaster resume' to continue.\n", planTask.ID, loopRes.Decision.Reason)
		_ = e.stateMgr.UpdateTaskState(planTask.ID, func(ts *schemas.TaskState) {
			ts.Status = schemas.TaskStatePaused
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
				fmt.Printf("⤼ [Scheduler] Task %s skipped: upstream dependency '%s' is %s\n", t.ID, dep, upState)
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
	fmt.Printf("✗ [Scheduler] Task %s failed: %s\n", taskID, errMsg)
	now := time.Now().UTC()
	_ = e.stateMgr.UpdateTaskState(taskID, func(ts *schemas.TaskState) {
		ts.Status = schemas.TaskStateFailed
		ts.Error = errMsg
		ts.FinishAt = &now
	})
}

func (e *Engine) loadTaskSpec(t schemas.PlanTask) (*schemas.TaskSpec, error) {
	// Search in tasks/ for matching Txx-*.md file
	tasksDir := filepath.Join(e.projectRoot, ".ringmaster", "tasks")
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

		AcceptanceTimeout: t.AcceptanceTimeout,
	}, nil
}

// TaskIDs returns the plan's task IDs in plan order.
func (e *Engine) TaskIDs() []string {
	ids := make([]string, 0, len(e.plan.Tasks))
	for _, t := range e.plan.Tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

// taskTrailers identifies a task commit's task, automaton and attempt in git.
func taskTrailers(taskID, automaton string, attempt int) []workspace.Trailer {
	return []workspace.Trailer{
		{Key: "Ringmaster-Task", Value: taskID},
		{Key: "Ringmaster-Automaton", Value: automaton},
		{Key: "Ringmaster-Attempt", Value: fmt.Sprintf("%d", attempt)},
	}
}

// firstLine returns the first line of prose in a markdown summary (headings
// skipped), trimmed to fit a commit subject.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "*- "))
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > 72 {
			line = string(r[:71]) + "…"
		}
		return line
	}
	return ""
}

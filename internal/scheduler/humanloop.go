package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

var answerTaskIDRegex = regexp.MustCompile(`^T\d{2,3}$`)

const skippedPrefix = "Skipped because upstream"

func answersDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".ringmaster", "answers")
}

func approvalsPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".ringmaster", "approvals.json")
}

// QueueAnswer stores a human answer in the project's inbox. A running scheduler applies
// it on its next tick; otherwise the next run or resume applies it before dispatching.
func QueueAnswer(projectRoot string, ans schemas.HumanAnswer) error {
	if !answerTaskIDRegex.MatchString(ans.TaskID) {
		return fmt.Errorf("invalid task id '%s'", ans.TaskID)
	}
	if ans.Scope != "" && ans.Scope != "task" && ans.Scope != "project" {
		return fmt.Errorf("invalid scope '%s': must be task or project", ans.Scope)
	}
	if err := os.MkdirAll(answersDir(projectRoot), 0755); err != nil {
		return err
	}
	file := filepath.Join(answersDir(projectRoot), ans.TaskID+".json")

	// Merge with an answer still waiting in the inbox rather than dropping it
	if prev, err := storage.ReadJSON[schemas.HumanAnswer](file); err == nil && prev != nil {
		for id, reply := range ans.Replies {
			if prev.Replies == nil {
				prev.Replies = map[string]string{}
			}
			prev.Replies[id] = reply
		}
		for _, cmd := range ans.Allow {
			if !slices.Contains(prev.Allow, cmd) {
				prev.Allow = append(prev.Allow, cmd)
			}
		}
		if ans.Scope != "" && len(ans.Allow) > 0 {
			prev.Scope = ans.Scope
		}
		prev.Text = strings.TrimSpace(prev.Text + "\n" + ans.Text)
		ans = *prev
	}
	return storage.WriteJSONAtomic(file, ans)
}

// LoadApprovals reads the commands the user approved beyond the sandbox allow list.
func LoadApprovals(projectRoot string) *schemas.CommandApprovals {
	a, err := storage.ReadJSON[schemas.CommandApprovals](approvalsPath(projectRoot))
	if err != nil || a == nil {
		a = &schemas.CommandApprovals{}
	}
	if a.Tasks == nil {
		a.Tasks = map[string][]string{}
	}
	return a
}

// applyQueuedAnswers applies inbox answers to tasks that are waiting on the user.
func (e *Engine) applyQueuedAnswers() {
	entries, err := os.ReadDir(answersDir(e.projectRoot))
	if err != nil {
		return
	}
	state := e.stateMgr.GetState()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file := filepath.Join(answersDir(e.projectRoot), entry.Name())
		ans, err := storage.ReadJSON[schemas.HumanAnswer](file)
		if err != nil || ans == nil {
			continue
		}
		ts := state.TaskStates[ans.TaskID]
		switch ts.Status {
		case schemas.TaskStateRunning:
			continue // apply once the current attempt has finished
		case schemas.TaskStateCompleted:
			fmt.Printf("🙋 [Scheduler] Ignoring answer for %s: task already completed.\n", ans.TaskID)
		default:
			if err := e.applyAnswer(ts, *ans); err != nil {
				fmt.Printf("✗ [Scheduler] Failed applying answer for %s: %v\n", ans.TaskID, err)
				continue
			}
		}
		_ = os.Remove(file)
	}
}

func (e *Engine) applyAnswer(ts schemas.TaskState, ans schemas.HumanAnswer) error {
	if len(ans.Allow) > 0 {
		approvals := LoadApprovals(e.projectRoot)
		for _, cmd := range ans.Allow {
			cmd = strings.Join(strings.Fields(cmd), " ")
			if cmd == "" {
				continue
			}
			if ans.Scope == "project" {
				if !slices.Contains(approvals.Project, cmd) {
					approvals.Project = append(approvals.Project, cmd)
				}
			} else if !slices.Contains(approvals.Tasks[ans.TaskID], cmd) {
				approvals.Tasks[ans.TaskID] = append(approvals.Tasks[ans.TaskID], cmd)
			}
		}
		if err := storage.WriteJSONAtomic(approvalsPath(e.projectRoot), approvals); err != nil {
			return err
		}
	}

	if err := e.appendHumanAnswers(ans.TaskID, renderHumanAnswer(ts.Requests, ans)); err != nil {
		return err
	}

	_ = e.stateMgr.UpdateTaskState(ans.TaskID, func(cur *schemas.TaskState) {
		cur.Status = schemas.TaskStatePending
		cur.Error = ""
		cur.Requests = nil
	})
	e.resetSkippedTasks()
	e.appendEvent(ans.TaskID, "HUMAN_DECISION", ans)
	fmt.Printf("🙋 [Scheduler] Applied your answer for %s; task re-queued.\n", ans.TaskID)
	return nil
}

// resetSkippedTasks re-queues tasks skipped for a failed upstream; they are skipped
// again if that upstream is still failing.
func (e *Engine) resetSkippedTasks() {
	state := e.stateMgr.GetState()
	for id, ts := range state.TaskStates {
		if ts.Status == schemas.TaskStateFailed && strings.HasPrefix(ts.Error, skippedPrefix) {
			_ = e.stateMgr.UpdateTaskState(id, func(cur *schemas.TaskState) {
				cur.Status = schemas.TaskStatePending
				cur.Error = ""
			})
		}
	}
}

// renderHumanAnswer formats an answer for the task spec's "## Human Answers" section.
func renderHumanAnswer(requests []schemas.HumanRequest, ans schemas.HumanAnswer) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Answer (%s)\n", time.Now().UTC().Format(time.RFC3339)))
	for _, r := range requests {
		reply, ok := ans.Replies[r.ID]
		if !ok {
			continue
		}
		switch r.Type {
		case schemas.RequestApproval:
			sb.WriteString(fmt.Sprintf("- [%s] Command `%s` → %s\n", r.ID, r.Command, reply))
		case schemas.RequestInput:
			if r.Secret {
				sb.WriteString(fmt.Sprintf("- [%s] %s → read it from environment variable `%s`; never hard-code or print its value\n", r.ID, r.Question, reply))
			} else {
				sb.WriteString(fmt.Sprintf("- [%s] %s → %s\n", r.ID, r.Question, reply))
			}
		default:
			sb.WriteString(fmt.Sprintf("- [%s] %s → %s\n", r.ID, r.Question, reply))
		}
	}
	if len(ans.Allow) > 0 {
		scope := "this task"
		if ans.Scope == "project" {
			scope = "the whole project"
		}
		sb.WriteString(fmt.Sprintf("- Approved for %s: `%s`\n", scope, strings.Join(ans.Allow, "`, `")))
	}
	if t := strings.TrimSpace(ans.Text); t != "" {
		sb.WriteString(t + "\n")
	}
	return sb.String()
}

// appendHumanAnswers adds text to the "## Human Answers" section of tasks/<id>-*.md.
func (e *Engine) appendHumanAnswers(taskID, text string) error {
	tasksDir := filepath.Join(e.projectRoot, ".ringmaster", "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), taskID) && strings.HasSuffix(entry.Name(), ".md") {
			path := filepath.Join(tasksDir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := strings.TrimRight(string(content), "\n")
			if !strings.Contains(s, "## Human Answers") {
				s += "\n\n## Human Answers"
			}
			s += "\n\n" + strings.TrimRight(text, "\n") + "\n"
			return os.WriteFile(path, []byte(s), 0644)
		}
	}
	return fmt.Errorf("task file for '%s' not found", taskID)
}

func (e *Engine) appendEvent(taskID, action string, payload any) {
	fd, err := os.OpenFile(e.eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer fd.Close()
	data, _ := json.Marshal(map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"task_id":   taskID,
		"action":    action,
		"payload":   payload,
	})
	_, _ = fd.Write(append(data, '\n'))
}

// haltStatus is the project status when no task can make progress without the user.
func haltStatus(ps *schemas.ProjectState) string {
	status := "HALTED"
	for _, ts := range ps.TaskStates {
		switch ts.Status {
		case schemas.TaskStateAwaiting:
			return "AWAITING_INPUT"
		case schemas.TaskStatePaused:
			status = "PAUSED"
		}
	}
	return status
}

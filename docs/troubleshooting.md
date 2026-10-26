# Ringmaster — Troubleshooting & Operations Guide

Operational guide for resolving task statuses, bottlenecks, and manual human interventions during Ringmaster orchestration.

---

## 1. Task Status: `BLOCKED`

### Cause:
A worker agent requires human input, missing credentials, architecture decisions, or permissions that it cannot resolve autonomously.

### Solution:
1. Check detailed blockers via `status`:
   ```bash
   ringmaster status <project_name>
   ```
2. Provide the unblocking answer:
   ```bash
   ringmaster answer <task_id> --text "Database host is postgresql://user:pass@localhost:5432/db"
   ```
3. Resume execution:
   ```bash
   ringmaster resume <project_name>
   ```

---

## 2. Task Status: `NEEDS_REVIEW`

### Cause:
- A **Merge Conflict** was detected when attempting to merge the task worktree branch (`rm/Txx`) into `main`.
- A worker exhausted its `max_retries` without satisfying acceptance criteria.

### Solution:
1. Inspect the worker report output:
   ```bash
   cat <project_name>/outputs/<task_id>.md
   ```
2. If due to a **Merge Conflict**:
   - The `main` branch remains clean (`git merge --abort` is executed automatically).
   - Resolve conflicts in your repo or adjust task dependencies, then resume.
3. Resume execution:
   ```bash
   ringmaster resume <project_name>
   ```

---

## 3. Project Status: `PAUSED`

### Cause:
Cumulative token consumption reached or exceeded the configured project budget (`budget.max_tokens_per_project` in `ringmaster.toml`).

### Solution:
1. Increase the token budget in `ringmaster.toml`:
   ```toml
   [budget]
   max_tokens_per_project = 3000000
   ```
2. Resume execution:
   ```bash
   ringmaster resume <project_name>
   ```

---

## 4. Plan Hash Mismatch ("Plan has been modified since approval")

### Cause:
The `plan.json` or task specification files were edited after approval was granted.

### Solution:
Re-run the planning command to inspect diffs and grant approval:
```bash
ringmaster plan <project_name>
```
Press `y` to approve the updated plan.


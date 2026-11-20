# Workflow

[← README](../../README.md)

## From brief to merge

1. **Brief.** You describe what to build, in a chat with the master planner or in `.ringmaster/BRIEF.md`. A chat's brief keeps each step's requests together with what you agreed with the master, so a decision made in the discussion ("use Go 1.27, not 1.22") holds in later steps too. When the master plans a later step, it also reads the project's manifests on `main` (`go.mod`, `package.json`, `Dockerfile`, and the like), so it plans for the versions and dependencies the project really uses.
2. **Plan.** The master agent splits the brief into a DAG of tasks, each with a worker, dependencies and acceptance commands. It asks you about decisions it should not guess, and nothing runs until you approve the plan (the approval is tied to the plan's SHA-256 hash).
3. **Run.** Each task gets its own branch, named for the change (`feat/link-store`, `docs/usage`), and a worktree. Workers run in parallel where dependencies allow; a task that depends on another starts from that task's branch.
4. **Verify.** Ringmaster runs the acceptance commands itself instead of trusting the worker's report. Failures go back to the worker for another attempt; problems an attempt cannot fix (a timeout, a command that cannot run, a program that is not installed, such as `docker` in a WSL distro without it) go to you instead. A task whose check needs a program that is missing is stopped before its automaton starts.
5. **Merge.** Passing tasks are merged into `main` one at a time. Every attempt is a commit tagged with the automaton that made it.

## Acceptance commands

Acceptance commands run directly, without a shell: quotes keep an argument together (`python -c "import mypkg"`), but pipes, `&&`, `;`, redirects and substitutions do not work. A command that could never run is sent back to the master to rewrite before the plan reaches you, and a task that still has one is stopped before its automaton starts.

The master is told which commands you allow. If a task's acceptance check still needs one that is not on the list, the task asks you to approve it before its automaton starts, the same way an automaton asks for a command.

Each acceptance command has a time limit (default 300 s), adjustable per project or per task; see [Configuration](configuration.md#project-manifest).

## When a task needs you

When a worker needs you (a decision, a value only you know, or approval for a command outside its allowlist), the task waits until you answer. Every stop says what kind of answer it needs: approve or deny a command, pick one of a few options (and still write your own if none fits), or write an answer. When Ringmaster stops a task itself (checks that keep failing, a used-up budget, a report it could not read), it offers the usual next steps with the recommended one picked, so the common case is one click.

```bash
ringmaster answer T03 --text "Use SQLite"
ringmaster answer T06 --reply r1=approve --allow "docker build -t app ."
ringmaster resume
```

### Changing a task's checks

Only you can change a task's checks. An answer goes to the automaton, which cannot skip or change an acceptance command, so telling it "don't run docker" does not help. When checks fail, cannot run, or run out of time, pick **Change the acceptance commands**: edit the list (one command per line), and the task runs again against only those. From the command line: `ringmaster answer T13 --acceptance "go test ./..."`, repeated for each command to keep.

## The next change

When a plan is finished, plan the next one from Chat. The master is told which tasks are already done and merged, plans only what is new, and numbers its tasks on from the last one (`T06` after `T05`), so finished tasks stay as history and nothing is run twice. Each plan gets the project token budget anew. From the command line, `ringmaster plan --force` generates a new plan although one exists.

## Working in an existing repository

`ringmaster init` adds only `ringmaster.toml` and a `.ringmaster/` directory. Both are kept out of git through `.git/info/exclude`, so they never show up in `git status`, are never pushed, and your `.gitignore` is left alone. It never stages or commits your files. To share the manifest with your team, commit it yourself with `git add -f ringmaster.toml`.

Merge commits are built without checking anything out. If you have `main` checked out with uncommitted changes that overlap a task, the merge waits for you instead of overwriting them.

Project commands work from any subdirectory of a project, the way `go` finds `go.mod`, and refuse to run outside one.

## Web UI

| View | What it shows |
|---|---|
| **Chat** | Brief the master planner, then generate the plan |
| **Tasks** | The DAG, plan approval with open questions, and per-task detail (spec, report, transcripts, answers). When every task is done: what the step did, and a link to Branches |
| **Branches** | Commit graph per task branch: where it forked, what each automaton committed, when it merged. Set the git remote and push `main` and the task branches. Each finished step: release it, download it, or finish it without a release |
| **Providers** | Providers and agents; pick the master and default worker |
| **Logs** | Live and past runs, colored per task, with a lane for each working automaton |
| **Analytics** | Tokens, cost and time per task and per agent |
| **Terminal** | A floating shell in the project directory (toggle with ``Ctrl+` ``), restricted to localhost |

There is a light and a dark theme. The web UI listens on localhost only; see [Configuration](configuration.md#web-ui-port-and-remote-servers) to change the port or reach it on a remote server.

## Telemetry

Worker steps stream to the logs as they happen. Every run is appended to `.ringmaster/logs/run.jsonl`, and reports and transcripts are kept after their worktree is removed. Token usage includes prompt-cache reads and writes, plus cost when the provider reports it. `ringmaster usage` shows token usage per agent and task.

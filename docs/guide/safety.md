# Safety

[← README](../../README.md)

## Guardrails for workers

Workers run commands only from an allowlist (see [presets](configuration.md#commands-presets-and-always-ask)), and their file tools are confined to the task's worktree.

- **Agents on an API.** Ringmaster checks every command and path itself.
- **Agents on a Claude subscription.** The Claude Code CLI enforces the same rules: Bash only for the allowed commands, file access granted in the worktree only, and Claude Code told to refuse reads and writes outside it (`blockReadsOutsideWorkingDirectories`, Claude Code 2.1.314 or later; older versions keep the worktree-only grants but not that refusal).

Worker and acceptance commands see only a built-in set of environment variables (`PATH`, `HOME`, toolchain paths, proxies) plus the ones you name in `pass_env`, so your API keys and other credentials stay out unless you list them. For agents on a Claude subscription, the Claude Code CLI runs the commands itself, so they also see the CLI's own `CLAUDE_*` and `ANTHROPIC_*` variables.

**These are guardrails, not isolation.** The limits apply to the file tools, not to commands, so an allowed command such as `go test` still runs code the worker wrote, with your user's permissions, and can read files outside the worktree. Run Ringmaster inside a container or VM if you need a hard boundary.

## Protected tests

Acceptance commands are only worth something if the worker cannot rewrite what they check. So before each acceptance run, Ringmaster compares the task's workspace with the commit its branch started from and puts back every test file that existed there and was modified or deleted since. Test files the worker added are kept. If the checks then fail, the next attempt is told which files were restored and why; the restore is also recorded in `events.jsonl` and in the task's report.

A file counts as a test by its name or directory: `*_test.go`, `test_*.py`, `*.test.*`, `*.spec.*`, anything under `tests/`, `test/`, `__tests__/`, `spec/` or `testdata/`, and the usual names for Ruby, Rust, Java, C# and PHP. To use your own list, or to turn the protection off:

```toml
[sandbox]
test_files = ["*.feature", "checks/"]   # replaces the built-in list; "name/" is a directory, the rest file-name globs
allow_test_edits = true                 # workers may change existing tests
```

Some tasks have to change existing tests, because the behaviour they pin down is meant to change. The master planner marks such a task with `"edits_tests": true` in `plan.json`; the plan preview and the web UI's task cards point these tasks out, and the flag is part of the approved plan's hash. Tasks of type `test` are always exempt.

The planner cannot always tell. When a task that may not change existing tests stops, and it changed a test (which Ringmaster put back) or asks about one, the stop also offers **Let this task change the existing tests it needs to**. Choose it, and say which test is outdated, for example the case to drop; from then on Ringmaster keeps that task's test changes. Telling the automaton to edit the test is not enough, since protected tests are put back before every check. From the command line: `ringmaster answer T07 --allow-test-edits --text "..."`. An automaton that asks you to "approve" something that is not a command, or a command you approved already, gets a question instead, since approving would change nothing.

This covers workers of every kind, since the restore is done by Ringmaster after the worker has finished. It does not stop a worker from making a test pass in other ways, such as special-casing the test's inputs in the code.

## What a worker sees

Each attempt starts a fresh conversation. The worker's prompt holds:

- the project's whole brief (`.ringmaster/BRIEF.md`);
- its own task: ID, title, description and acceptance commands;
- the answers you gave this task (`## Human Answers` in its task file);
- whether it may change existing tests;
- the enabled skills, by name and description only (it reads a skill's full instructions when one fits);
- the commands it may run, the report it must write, and how to ask you for something;
- on a retry, the output of the checks that failed in the attempt before, and which test changes were undone.

It is not given the chat with the master, the plan, other tasks' specs, or other attempts' transcripts. It reads the code itself, with its tools, from its worktree: a checkout of `main`, or of the task it depends on.

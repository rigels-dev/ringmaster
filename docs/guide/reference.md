# Reference

[← README](../../README.md)

## Project layout

```
my-project/                          your code; the project is the git repository itself
├── ringmaster.toml                  project manifest (name, optional sandbox/budget overrides)
└── .ringmaster/                     all Ringmaster state (excluded from git)
    ├── BRIEF.md                     requirements
    ├── chat_history.json            your conversation with the master planner
    ├── plan.json                    the task DAG, open questions and assumptions
    ├── plan.approved.json           approval, bound to the plan hash
    ├── tasks/Txx-*.md               task specs, including your answers
    ├── state.json                   task status, attempts and token usage
    ├── answers/                     answers queued for waiting tasks
    ├── approvals.json               commands you approved beyond the allowlist
    ├── skills/<name>/SKILL.md       skills for this project's workers (optional)
    ├── run.pid                      present while a run is active
    ├── outputs/Txx.md               worker reports (archived, kept after merge)
    ├── worktrees/Txx/               a task's checkout of its branch while it is active
    │   └── .ringmaster/
    │       ├── skills/              the enabled skills, copied in for the worker
    │       ├── outputs/Txx.md       the report as the worker writes it
    │       └── logs/Txx-attemptN.jsonl   live transcript of each attempt
    └── logs/
        ├── run.jsonl                output of every run, appended
        ├── run-YYYYMMDD-HHMMSS.jsonl    earlier run logs (rotated at 10 MB or cleared in the UI)
        ├── events.jsonl             audit trail: attempts, checks, decisions, answers
        └── Txx-attemptN.jsonl       per-attempt transcripts (archived)
```

Task worktrees are removed once their branch is merged; the task branches stay, so the history can still be traced in the Branches view or with `git log`.

## Branch names

A task's branch is named the way people name them: `<prefix>/<slug>`, with the prefix one of `feat`, `fix`, `docs`, `test`, `refactor`, `chore`. The master proposes the name and you see it on the task card before approving; Ringmaster makes it valid and unique. A task never takes over a branch that already exists, yours or an earlier task's: it gets a numeric suffix (`feat/login-2`). The task ID (`T03`) stays in the UI, in commit trailers and in merge commits, which also carry a `Ringmaster-Branch` trailer. Projects from before v0.7 keep their `rm/Txx` branches.

## Commits

Each attempt is committed on its task branch with trailers:

```
T02: fix data race when copying links

Ringmaster-Task: T02
Ringmaster-Automaton: sonnet-5.5
Ringmaster-Attempt: 2
```

These commits are yours: author and committer are the identity you have set up for git (`user.name` and `user.email`), so a git host shows them under your name alone. Ringmaster's part stays visible in the commit message and the trailers. Use an email your git host knows if you want the commits linked to your account. If git has no identity set, `ringmaster init`, `ringmaster run` and the web UI say so and show the two `git config` commands; until then the commits are made as Ringmaster. Ringmaster never edits your git config.

## Versioning

Releases are named `v{major}.{mid}.{minor}-{codename}`. Codenames are stars in alphabetical order, starting with Antares. The codename moves to the next letter with every major or mid release and stays the same for minor fixes, so a new codename always means an update worth installing. The current version is in [`VERSION`](../../VERSION).

## Requirements

- Linux or macOS
- git 2.38 or newer
- Node.js 22 or newer for the web UI (the CLI does not need it)
- Go 1.27 or newer, only to build from source
- Optional: the Claude Code CLI (`claude`, logged in) for Claude-subscription agents, and whatever your tasks' acceptance commands need (Docker, Python, …)

The install script accepts `RINGMASTER_VERSION` (e.g. `v0.2.0-bellatrix`) to pick a release, and `RINGMASTER_HOME` / `RINGMASTER_BIN_DIR` to change where it installs (default `~/.local/share/ringmaster`, linked into `~/.local/bin`).

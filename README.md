<p align="center">
  <img src="docs/assets/ringmaster-animated.svg" width="220" height="220" alt="Ringmaster emblem" />
</p>

# 🎪 Ringmaster

**A local control plane for teams of AI coding agents.** Ringmaster turns a project brief into a plan of tasks, hands each task to an AI worker in its own git worktree, checks the result with real commands, and merges what passes into `main`, while you watch, answer questions and approve from a terminal or a web UI.

Current release: **v0.1.0-antares**. Ringmaster runs on Linux and macOS.

---

## How it works

1. **Brief.** You describe what to build, in a chat with the master planner or in `.ringmaster/BRIEF.md`.
2. **Plan.** The master agent splits the brief into a DAG of tasks, each with a worker, dependencies and acceptance commands. It asks you about decisions it should not guess, and nothing runs until you approve the plan (the approval is tied to the plan's SHA-256 hash).
3. **Run.** Each task gets a branch `rm/Txx` and a worktree. Workers run in parallel where dependencies allow; a task that depends on another starts from that task's branch.
4. **Verify.** Ringmaster runs the acceptance commands itself instead of trusting the worker's report. Failures go back to the worker for another attempt; problems an attempt cannot fix (a timeout, a command that cannot run) go to you instead.
5. **Merge.** Passing tasks are merged into `main` one at a time. Every attempt is a commit tagged with the automaton that made it.

When a worker needs you (a decision, a value only you know, or approval for a command outside its sandbox), the task waits until you answer.

## Features

- **Works in your existing repository.** `ringmaster init` adds only `ringmaster.toml` and a `.ringmaster/` directory, kept out of git through `.git/info/exclude`. It never stages or commits your files.
- **Merges that leave your checkout alone.** Merge commits are built without checking anything out. If you have `main` checked out with uncommitted changes that overlap a task, the merge waits for you instead of overwriting them.
- **Traceable history.** Each attempt is committed on its task branch with trailers:
  ```
  T02: fix data race when copying links

  Ringmaster-Task: T02
  Ringmaster-Automaton: sonnet-5.5
  Ringmaster-Attempt: 2
  ```
- **Any provider.** OpenAI-compatible APIs (OpenAI, OpenRouter, Ollama, vLLM, DeepSeek and others), the Anthropic API, and a local Claude subscription through the Claude Code CLI.
- **One fleet for all projects.** Providers and agents are configured once, globally; a project only assigns agents to its tasks.
- **Sandboxed workers.** Commands run from an allowlist with a scrubbed environment and path-traversal checks. Each acceptance command has a time limit (default 300 s), adjustable per project or per task.
- **Live and lasting telemetry.** Worker steps stream to the logs as they happen. Every run is appended to `.ringmaster/logs/run.jsonl`, and reports and transcripts are kept after their worktree is removed. Token usage includes prompt-cache reads and writes, plus cost when the provider reports it.

### Web UI

| View | What it shows |
|---|---|
| **Chat** | Brief the master planner, then generate the plan |
| **Tasks** | The DAG, plan approval with open questions, and per-task detail (spec, report, transcripts, answers) |
| **Branches** | Commit graph per task branch: where it forked, what each automaton committed, when it merged |
| **Providers** | Providers and agents; pick the master and default worker |
| **Logs** | Live and past runs, colored per task, with a lane for each working automaton |
| **Analytics** | Tokens, cost and time per task and per agent |
| **Terminal** | A floating shell in the project directory (`Ctrl+\``), restricted to localhost |

There is a light and a dark theme.

---

## Requirements

- Linux or macOS
- Go 1.27 or newer
- Node.js 22 or newer, to build the web UI
- git 2.38 or newer
- Optional: the Claude Code CLI (`claude`, logged in) for Claude-subscription agents, and whatever your tasks' acceptance commands need (Docker, Python, …)

## Install

```bash
git clone <this repository> ringmaster
cd ringmaster
make build web        # bin/ringmaster and the web UI
ln -s "$PWD/bin/ringmaster" ~/.local/bin/ringmaster
ringmaster --version  # ringmaster version v0.1.0-antares
```

Keep the binary in the repository's `bin/`: Ringmaster finds its global config and web UI relative to it.

## Configure your fleet

The fleet lives in `ringmaster.toml` at the root of the Ringmaster install. Start from the example, or use the interactive editor:

```bash
cp ringmaster.example.toml ringmaster.toml
ringmaster config        # TUI: add providers, pick models, set master (m) and default worker (w)
ringmaster agents list   # check what is registered
```

```toml
[providers.claude_sub]
type = "claude_subscription"          # uses your local Claude Code login

[providers.openrouter]
type = "openai"
base_url = "https://openrouter.ai/api/v1"
api_key_env = "OPENROUTER_API_KEY"    # the name of the variable, not the key

[agents.planner]
provider = "claude_sub"
model = "claude-opus-5-5"
role = "master"

[agents.haiku]
provider = "claude_sub"
model = "claude-haiku-5-5"
role = "worker"
tool_calling = true

[routing]
default_master = "planner"
default_worker = "haiku"
```

## Quickstart

```bash
# A new project…
ringmaster init url-shortener && cd url-shortener
# …or an existing repository (run at its root)
cd my-repo && ringmaster init

ringmaster               # launcher: open the web UI (http://localhost:12053) or the TUI
```

From the command line instead:

```bash
ringmaster plan          # generate the plan, review it, press y to approve
ringmaster run -p 4      # run up to 4 tasks in parallel
ringmaster status --watch
ringmaster answer T03 --text "Use SQLite"          # reply to a waiting task
ringmaster answer T06 --reply r1=approve --allow "docker build -t app ."
ringmaster resume
ringmaster usage         # token usage per agent and task
```

Project commands work from any subdirectory of a project, the way `go` finds `go.mod`, and refuse to run outside one.

**Using a remote server?** The web UI listens on localhost only. Forward it over SSH:

```bash
ssh -N -L 12053:localhost:12053 user@your-server
```

## Project layout

```
my-project/                        your code; the project is the git repository itself
├── ringmaster.toml                project manifest
└── .ringmaster/                   all Ringmaster state (excluded from git)
    ├── BRIEF.md                   requirements
    ├── plan.json                  the task DAG
    ├── plan.approved.json         approval, bound to the plan hash
    ├── tasks/Txx-*.md             task specs, including your answers
    ├── state.json                 task status and token usage
    ├── outputs/Txx.md             worker reports (kept after merge)
    ├── worktrees/Txx/             a task's working copy while it is active
    └── logs/
        ├── run.jsonl              output of every run
        ├── events.jsonl           audit trail
        └── Txx-attemptN.jsonl     per-attempt transcripts
```

The manifest can tune how this project's tasks run:

```toml
[sandbox]
allow = ["go test", "go vet", "go build"]   # commands workers and acceptance checks may run
acceptance_timeout = 600                     # seconds per acceptance command

[budget]
max_tokens_per_task = 300000
```

A task in the plan can also set its own `acceptance_timeout`.

## Versioning

Releases are named `v{major}.{mid}.{minor}-{codename}`. Codenames are stars in alphabetical order, starting with Antares. The codename moves to the next letter with every major or mid release and stays the same for minor fixes, so a new codename always means an update worth installing. The current version is in [`VERSION`](VERSION).

## Development

```bash
make test     # go vet and the test suite
make build    # binary, stamped with the version from VERSION
make web      # web UI (Nuxt)
make tag      # tag the current commit with the version from VERSION
```

## License

MIT; see [LICENSE](LICENSE).

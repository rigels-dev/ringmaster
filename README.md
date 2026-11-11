<p align="center">
  <img src="docs/assets/ringmaster-animated.svg" width="220" height="220" alt="Ringmaster emblem" />
</p>

# 🎪 Ringmaster

**A local control plane for teams of AI coding agents.** Ringmaster turns a project brief into a plan of tasks, hands each task to an AI worker in its own git worktree, checks the result with real commands, and merges what passes into `main`, while you watch, answer questions and approve from a terminal or a web UI.

Current release: **v0.3.2-capella**. Ringmaster runs on Linux and macOS.

---

## How it works

1. **Brief.** You describe what to build, in a chat with the master planner or in `.ringmaster/BRIEF.md`.
2. **Plan.** The master agent splits the brief into a DAG of tasks, each with a worker, dependencies and acceptance commands. It asks you about decisions it should not guess, and nothing runs until you approve the plan (the approval is tied to the plan's SHA-256 hash).
3. **Run.** Each task gets a branch `rm/Txx` and a worktree. Workers run in parallel where dependencies allow; a task that depends on another starts from that task's branch.
4. **Verify.** Ringmaster runs the acceptance commands itself instead of trusting the worker's report. Failures go back to the worker for another attempt; problems an attempt cannot fix (a timeout, a command that cannot run) go to you instead.
5. **Merge.** Passing tasks are merged into `main` one at a time. Every attempt is a commit tagged with the automaton that made it.

When a worker needs you (a decision, a value only you know, or approval for a command outside its allowlist), the task waits until you answer.

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
- **Guardrails for workers.** Workers run commands only from an allowlist, and their file tools are confined to the task's worktree. Worker and acceptance commands see only a built-in set of environment variables (`PATH`, `HOME`, toolchain paths, proxies) plus the ones you name in `pass_env`, so your API keys and other credentials stay out unless you list them. For agents on a Claude subscription, the Claude Code CLI runs the commands itself, so they also see the CLI's own `CLAUDE_*` and `ANTHROPIC_*` variables. These are guardrails, not isolation: an allowed command such as `go test` still runs code the worker wrote, with your user's permissions. Run Ringmaster inside a container or VM if you need a hard boundary. Each acceptance command has a time limit (default 300 s), adjustable per project or per task.
- **Agent skills.** Drop a skill folder (a `SKILL.md` plus whatever it refers to) into Ringmaster and every worker can use it, whichever model it runs on. Turn skills on and off from the TUI or the command line.
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
| **Terminal** | A floating shell in the project directory (toggle with ``Ctrl+` ``), restricted to localhost |

There is a light and a dark theme.

---

## Requirements

- Linux or macOS
- Go 1.27 or newer
- Node.js 22 or newer, to build the web UI
- git 2.38 or newer
- Optional: the Claude Code CLI (`claude`, logged in) for Claude-subscription agents, and whatever your tasks' acceptance commands need (Docker, Python, …)

## Install

**Install script** (Linux and macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/rigels-dev/ringmaster/main/install.sh | sh
```

It downloads the latest release for your platform, checks its SHA-256, installs it in `~/.local/share/ringmaster` and links `ringmaster` into `~/.local/bin`. Run it again to upgrade. Set `RINGMASTER_VERSION` (e.g. `v0.2.0-bellatrix`) to pick a release.

**Homebrew** (macOS and Linux):

```bash
brew install rigels-dev/tap/ringmaster
```

**From source:**

```bash
git clone https://github.com/rigels-dev/ringmaster.git
cd ringmaster
make build web        # bin/ringmaster and the web UI
ln -s "$PWD/bin/ringmaster" ~/.local/bin/ringmaster
```

Check the install with `ringmaster --version`. The web UI needs Node.js 22 or newer at run time; the CLI does not.

## Configure your fleet

The fleet lives in `~/.config/ringmaster/ringmaster.toml` (or `$XDG_CONFIG_HOME/ringmaster/`), outside the install, so upgrades keep it. Set it up with the interactive editor, or start from the example:

```bash
ringmaster config        # TUI: add providers, pick models, set master (m) and default worker (w)
ringmaster agents list   # check what is registered
```

A config left next to an older install (`<install>/ringmaster.toml`) is copied to the new location the first time you run a newer version.

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
ringmaster skills        # installed agent skills; enable or disable them
```

Project commands work from any subdirectory of a project, the way `go` finds `go.mod`, and refuse to run outside one.

**Using a remote server?** The web UI listens on localhost only. Forward it over SSH:

```bash
ssh -N -L 12053:localhost:12053 user@your-server
```

**Port taken, or two projects open at once?** The web UI listens on port 12053 by default. Pick another for one run with `ringmaster --port 4000` or `RINGMASTER_PORT=4000`, or keep it in the project's `ringmaster.toml` (or in the fleet config, for every project):

```toml
[web]
port = 4000
```

The most specific setting wins: the flag, then the environment variable, then the project, then the fleet config.

## Skills

A skill is a folder holding a `SKILL.md`: instructions for one kind of work (your code style, how to write a migration, a review checklist), in the format agent tools already share. Install one by copying or symlinking its folder:

```
~/.config/ringmaster/skills/<name>/SKILL.md      for every project
<project>/.ringmaster/skills/<name>/SKILL.md     for one project (wins over a global skill of the same name)
```

```bash
ringmaster skills                    # what is installed, and whether it is on
ringmaster skills disable go-style   # workers no longer get it (enable turns it back on)
```

The TUI (`ringmaster config`) has the same switches on its Skills tab. An installed skill is on until you disable it. A project can also turn skills off for itself in its `ringmaster.toml`:

```toml
[skills]
disabled = ["deploy"]
```

When a task starts, the enabled skills are copied into its workspace under `.ringmaster/skills/`, which git ignores, so they are never committed or merged. The worker's prompt lists each skill by name and description (taken from the `description` in the `SKILL.md` frontmatter), and the worker reads the full instructions only for the skills that fit its task. Three limits to know:

- A script shipped with a skill runs only if your command allowlist permits it; skills do not widen what a worker may execute.
- Symlinks inside a skill folder and files over 1 MB are not copied.
- Skills reach workers only. The master planner does not see them yet.

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
    ├── worktrees/Txx/               a task's checkout of branch rm/Txx while it is active
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

Task worktrees are removed once their branch is merged; the `rm/Txx` branches stay, so the history can still be traced in the Branches view or with `git log`.

The manifest can tune how this project's tasks run:

```toml
[sandbox]
allow = ["go test", "go vet", "go build"]   # commands workers and acceptance checks may run
acceptance_timeout = 600                     # seconds per acceptance command
pass_env = ["DATABASE_URL", "AWS_*"]         # extra environment variables those commands may see

[budget]
max_tokens_per_task = 300000
```

A project's `allow` replaces the fleet's list, while its `pass_env` adds to it; a trailing `*` matches a prefix. A task in the plan can also set its own `acceptance_timeout`.

## Versioning

Releases are named `v{major}.{mid}.{minor}-{codename}`. Codenames are stars in alphabetical order, starting with Antares. The codename moves to the next letter with every major or mid release and stays the same for minor fixes, so a new codename always means an update worth installing. The current version is in [`VERSION`](VERSION).

## Development

```bash
make test     # go vet and the test suite
make build    # binary, stamped with the version from VERSION
make dist     # release archives for linux/darwin × amd64/arm64, with checksums
make web      # web UI (Nuxt)
make tag      # tag the current commit with the version from VERSION
```

## License

MIT; see [LICENSE](LICENSE).

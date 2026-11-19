<p align="center">
  <img src="docs/assets/ringmaster-animated.svg" width="220" height="220" alt="Ringmaster emblem" />
</p>

# 🎪 Ringmaster

**A local control plane for teams of AI coding agents.** Ringmaster turns a project brief into a plan of tasks, hands each task to an AI worker in its own git worktree, checks the result with real commands, and merges what passes into `main`, while you watch, answer questions and approve from a terminal or a web UI.

Current release: **v0.13.2-mira**. Ringmaster runs on Linux and macOS.

---

## How it works

1. **Brief.** You describe what to build, in a chat with the master planner or in `.ringmaster/BRIEF.md`. A chat's brief keeps each step's requests together with what you agreed with the master, so a decision made in the discussion ("use Go 1.27, not 1.22") holds in later steps too. When the master plans a later step, it also reads the project's manifests on `main` (`go.mod`, `package.json`, `Dockerfile`, and the like), so it plans for the versions and dependencies the project really uses.
2. **Plan.** The master agent splits the brief into a DAG of tasks, each with a worker, dependencies and acceptance commands. It asks you about decisions it should not guess, and nothing runs until you approve the plan (the approval is tied to the plan's SHA-256 hash). Acceptance commands run directly, without a shell: quotes keep an argument together (`python -c "import mypkg"`), but pipes, `&&`, `;`, redirects and substitutions do not work. A command that could never run is sent back to the master to rewrite before the plan reaches you, and a task that still has one is stopped before its automaton starts. The master is told which commands you allow. If a task's acceptance check still needs one that is not on the list, the task asks you to approve it before its automaton starts, the same way an automaton asks for a command.
3. **Run.** Each task gets its own branch, named for the change (`feat/link-store`, `docs/usage`), and a worktree. Workers run in parallel where dependencies allow; a task that depends on another starts from that task's branch.
4. **Verify.** Ringmaster runs the acceptance commands itself instead of trusting the worker's report. Failures go back to the worker for another attempt; problems an attempt cannot fix (a timeout, a command that cannot run, a program that is not installed, such as `docker` in a WSL distro without it) go to you instead. A task whose check needs a program that is missing is stopped before its automaton starts.
5. **Merge.** Passing tasks are merged into `main` one at a time. Every attempt is a commit tagged with the automaton that made it.

When a worker needs you (a decision, a value only you know, or approval for a command outside its allowlist), the task waits until you answer. Every stop says what kind of answer it needs: approve or deny a command, pick one of a few options (and still write your own if none fits), or write an answer. When Ringmaster stops a task itself (checks that keep failing, a used-up budget, a report it could not read), it offers the usual next steps with the recommended one picked, so the common case is one click.

Only you can change a task's checks. An answer goes to the automaton, which cannot skip or change an acceptance command, so telling it "don't run docker" does not help. When checks fail, cannot run, or run out of time, pick **Change the acceptance commands**: edit the list (one command per line), and the task runs again against only those. From the command line: `ringmaster answer T13 --acceptance "go test ./..."`, repeated for each command to keep.

## Features

- **Works in your existing repository.** `ringmaster init` adds only `ringmaster.toml` and a `.ringmaster/` directory. Both are kept out of git through `.git/info/exclude`, so they never show up in `git status`, are never pushed, and your `.gitignore` is left alone. It never stages or commits your files. To share the manifest with your team, commit it yourself with `git add -f ringmaster.toml`.
- **Merges that leave your checkout alone.** Merge commits are built without checking anything out. If you have `main` checked out with uncommitted changes that overlap a task, the merge waits for you instead of overwriting them.
- **Traceable history.** Each attempt is committed on its task branch with trailers:
  ```
  T02: fix data race when copying links

  Ringmaster-Task: T02
  Ringmaster-Automaton: sonnet-5.5
  Ringmaster-Attempt: 2
  ```
  These commits are yours: author and committer are the identity you have set up for git (`user.name` and `user.email`), so a git host shows them under your name alone. Ringmaster's part stays visible in the commit message and the trailers. Use an email your git host knows if you want the commits linked to your account. If git has no identity set, `ringmaster init`, `ringmaster run` and the web UI say so and show the two `git config` commands; until then the commits are made as Ringmaster. Ringmaster never edits your git config.
- **Any provider.** OpenAI-compatible APIs (OpenAI, OpenRouter, Ollama, vLLM, DeepSeek and others), the Anthropic API, and a local Claude subscription through the Claude Code CLI.
- **One fleet for all projects.** Providers and agents are configured once, globally; a project only assigns agents to its tasks.
- **Guardrails for workers.** Workers run commands only from an allowlist, and their file tools are confined to the task's worktree. Worker and acceptance commands see only a built-in set of environment variables (`PATH`, `HOME`, toolchain paths, proxies) plus the ones you name in `pass_env`, so your API keys and other credentials stay out unless you list them. For agents on a Claude subscription, the Claude Code CLI runs the commands itself, so they also see the CLI's own `CLAUDE_*` and `ANTHROPIC_*` variables. These are guardrails, not isolation: an allowed command such as `go test` still runs code the worker wrote, with your user's permissions. Run Ringmaster inside a container or VM if you need a hard boundary. Each acceptance command has a time limit (default 300 s), adjustable per project or per task.
- **Tests a worker cannot weaken.** A worker may add tests, but its changes to test files that already existed are undone before the acceptance commands run, so a check cannot be passed by editing it. A task that really must change existing tests says so in the plan, where you see it before approving. If a task stops over a test that turned out to be outdated, Ringmaster offers to let that task change existing tests.
- **Agent skills.** Drop a skill folder (a `SKILL.md` plus whatever it refers to) into Ringmaster and every worker can use it, whichever model it runs on. Turn skills on and off from the TUI or the command line.
- **Live and lasting telemetry.** Worker steps stream to the logs as they happen. Every run is appended to `.ringmaster/logs/run.jsonl`, and reports and transcripts are kept after their worktree is removed. Token usage includes prompt-cache reads and writes, plus cost when the provider reports it.

### Web UI

| View | What it shows |
|---|---|
| **Chat** | Brief the master planner, then generate the plan |
| **Tasks** | The DAG, plan approval with open questions, and per-task detail (spec, report, transcripts, answers). When every task is done: what the step did, and a link to Branches |
| **Branches** | Commit graph per task branch: where it forked, what each automaton committed, when it merged. Set the git remote and push `main` and the task branches. Each finished step: release it, download it, or finish it without a release |
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

ringmaster               # launcher: open the web UI (http://localhost:2053) or the TUI
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
ringmaster remote set https://github.com/you/project.git   # once: where to publish
ringmaster push          # push main and the task branches there
ringmaster steps         # finished plans; release one with: ringmaster release <version> --step <n>
```

Project commands work from any subdirectory of a project, the way `go` finds `go.mod`, and refuse to run outside one.

**Publishing the result.** Tasks are merged into your local `main`; nothing leaves your machine until you push. Give the project a remote once, then push from the command line or from the Branches view of the web UI:

```bash
ringmaster remote set git@github.com:you/project.git
ringmaster remote        # the remote, and which branches still need pushing
ringmaster push          # main, every task branch (merged or not) and the release tags
ringmaster push --main-only
ringmaster prune         # delete the task branches already merged into main
ringmaster prune --remote  # and delete them on origin too
```

Only `main`, the task branches and the steps' release tags are pushed, each named explicitly from Ringmaster's own record; your other branches and tags are left alone, even ones with similar names. A release tagged without pushing (**Tag only**, or `ringmaster release` without `--push`) shows as *not pushed* in Branches and goes with the next push. A push is never forced: if the remote has commits this project does not, that branch is rejected and reported. Pushing uses your own git login (an SSH key or a credential helper) and never prompts, so a missing login fails at once. Ringmaster refuses a URL with credentials in it, since it would be stored in `.git/config`.

A task branch merged into `main` does not need pushing: its commits go with `main`. So once `main` (and any unfinished task branch) is on the remote, the Branches view says everything is pushed. Merged task branches can then be deleted with **Delete merged branches…** or `ringmaster prune`. The work stays in `main`'s history; only the branch names go. `main`, unfinished task branches, branches checked out in a worktree and your own branches are never deleted. With `--remote` (or the checkbox in the web UI) they are deleted on origin too, unless someone has moved the branch there since this project last saw it.

**When every task is done.** A finished plan becomes a *step* of the project: Step 1, Step 2, and so on, each ending at the commit on `main` where its last task was merged. The Tasks view says the step is done and links to Branches, where each step can be named, released, downloaded, or finished without a release. From the command line:

```bash
ringmaster steps                          # every step, and its release
ringmaster steps name 1 "First version"
ringmaster release                        # the last release, and suggested next versions
ringmaster release v0.2.0 --step 2        # tag where step 2 ended
ringmaster release v0.2.0 --step 2 --push # and push main and that tag to origin
ringmaster release notes --step 2 --ai     # draft the release notes, with a summary by AI
ringmaster release v0.2.0 --step 2 --notes-file notes.md --push --github-release
ringmaster release page v0.1.0            # a GitHub release page for a tag pushed earlier
ringmaster steps close 2                  # or: finished without a release
ringmaster archive v0.2.0                 # .ringmaster/archives/<project>-v0.2.0.zip
```

Steps are kept in `.ringmaster/steps.json`, so they are never committed or pushed. Their releases are read from git tags, so a tag you make by hand on a step's last commit counts too. Without `--step`, `ringmaster release` tags the tip of `main`.

A release is an annotated git tag on a commit of `main`, made in your name. The version is yours to choose, in any scheme. Ringmaster only suggests: from `v0.1.0` it offers `v0.1.1` (fixes only), `v0.2.0` (new features) and `v1.0.0` (breaking changes), and marks the one that fits what was built; with no tag yet it offers `v0.1.0`. A tag that is not a version number gets no suggestion. A version that exists is never moved or replaced, and a release is refused while tasks of the current plan are unfinished (`--force` overrides). An archive holds only the files git tracks at that tag, so Ringmaster's state and untracked files stay out; `--format tar.gz` is also available.

**Release notes.** The Release form starts with notes drafted from the step's tasks, one line each under *Features*, *Fixes*, *Documentation* or *Tests* (from the kind its branch names), and a place for a summary. Write the summary yourself, or press **Write summary with AI**: the planner model reads the tasks' specs and the files they changed and writes a sentence or two. That is one model call on your provider's quota, made only when you press it; the answers you gave the automatons are not sent. The notes go into the tag's message and are kept in `.ringmaster/releases/`. When the remote is on GitHub and the [GitHub CLI](https://cli.github.com) (`gh`) is installed and logged in, **Tag and push** also creates the release's page on GitHub with those notes. A release pushed without a page gets a **Create GitHub release page…** button in Branches (or `ringmaster release page <version>`). If the page cannot be made, the tag and the push still stand, and the reason is shown.

**Using a remote server?** The web UI listens on localhost only. Forward it over SSH:

```bash
ssh -N -L 2053:localhost:2053 user@your-server
```

**Port taken, or two projects open at once?** The web UI listens on port 2053 by default. Pick another for one run with `ringmaster --port 4000` or `RINGMASTER_PORT=4000`, or keep it in the project's `ringmaster.toml` (or in the fleet config, for every project):

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

The TUI (`ringmaster config`) has the same switches on its Skills tab, and the web UI on its Skills page. An installed skill is on until you disable it. A project can also turn skills off for itself in its `ringmaster.toml`:

```toml
[skills]
disabled = ["deploy"]
```

When a task starts, the enabled skills are copied into its workspace under `.ringmaster/skills/`, which git ignores, so they are never committed or merged. The worker's prompt lists each skill by name and description (taken from the `description` in the `SKILL.md` frontmatter), and the worker reads the full instructions only for the skills that fit its task. The master planner gets the same list of names and descriptions, so it can point a task at the skill that applies. Two limits to know:

- A script shipped with a skill runs only if your command allowlist permits it; skills do not widen what a worker may execute.
- Symlinks inside a skill folder and files over 1 MB are not copied.

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

**Branch names.** A task's branch is named the way people name them: `<prefix>/<slug>`, with the prefix one of `feat`, `fix`, `docs`, `test`, `refactor`, `chore`. The master proposes the name and you see it on the task card before approving; Ringmaster makes it valid and unique. A task never takes over a branch that already exists, yours or an earlier task's: it gets a numeric suffix (`feat/login-2`). The task ID (`T03`) stays in the UI, in commit trailers and in merge commits, which also carry a `Ringmaster-Branch` trailer. Projects from before v0.7 keep their `rm/Txx` branches.

**The next change.** When a plan is finished, plan the next one from Chat. The master is told which tasks are already done and merged, plans only what is new, and numbers its tasks on from the last one (`T06` after `T05`), so finished tasks stay as history and nothing is run twice. Each plan gets the project token budget anew. From the command line, `ringmaster plan --force` generates a new plan although one exists.

The manifest can tune how this project's tasks run:

```toml
[sandbox]
presets = ["go", "docker"]                   # command packages, usually added by approving a plan
allow = ["make test"]                        # single commands on top of the presets
always_ask = ["make deploy"]                 # stop and ask even though allowed
acceptance_timeout = 600                     # seconds per acceptance command
pass_env = ["DATABASE_URL", "AWS_*"]         # extra environment variables those commands may see

[budget]
max_tokens_per_task = 300000
```

Budgets apply even when you set none: 300,000 tokens per task and 2,000,000 per project by default. The project budget is for the plan being run; a later plan starts from zero. A token read from the prompt cache counts as a tenth of a token, which is what providers charge for it; agents that re-read their conversation from the cache on every step (a Claude subscription agent does) would otherwise hit the limit on small tasks. A task that reaches its budget before finishing is paused, and resuming gives it another full budget. An attempt that finishes its work is always verified, even if its last step went over.

A project's `allow` replaces the fleet's list, while its `pass_env` adds to it; a trailing `*` matches a prefix. A task in the plan can also set its own `acceptance_timeout`.

### Commands: presets and always-ask

A *preset* is a package of commands for one language or tool: what automatons may run without asking, and what still needs your approval although an allowed command covers it. Ringmaster has presets for `go`, `python`, `node`, `rust` and `docker`:

| Preset | Allowed | Always asked |
|---|---|---|
| `go` | `go build`, `go test`, `go vet`, `go mod`, `gofmt` | `go clean` |
| `python` | `python`, `python3`, `pytest`, `ruff`, `mypy` | `pip uninstall` |
| `node` | `node`, `npm`, `npx` | `npm publish` and the other registry commands |
| `rust` | `cargo build`, `cargo test`, `cargo check`, `cargo clippy`, `cargo fmt` | `cargo publish`, `cargo yank`, `cargo login` |
| `docker` | `docker build`, `docker compose` | `docker compose down`, `docker push`, `docker system prune`, removals |

The master names the presets a plan needs in its `stack` (`["go", "docker"]`). The approval banner shows what each one allows, and approving the plan adds them to the project's `[sandbox] presets`; Ringmaster only ever adds to that line, and leaves the rest of the file as it was. A preset of the same name under `[presets.<name>]` in the fleet config replaces the built-in one, and new names can be defined there too:

```toml
[presets.go]
allow = ["go build", "go test", "go vet", "go mod", "gofmt", "go generate"]
always_ask = ["go clean"]
```

Some commands always stop the task to ask, in every project, even where an allowed entry would cover them: `git push`, `git reset --hard`, `git clean`, `rm`, `sudo`, `curl`, `ssh` and similar ones that leave the machine, discard work or delete files. A command you approve while a task runs is allowed as approved, for that task or the whole project. `ringmaster presets` lists the presets and what the project's automatons may run now.

A project that names no presets keeps the old built-in list (`pytest`, `npm`, `go test`, …). Once it has presets, they replace that list, so a Go project does not also get `npm`.

### Protected tests

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

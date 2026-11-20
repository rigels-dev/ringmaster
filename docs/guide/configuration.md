# Configuration

[← README](../../README.md)

## The fleet

Providers and agents are configured once, globally; a project only assigns agents to its tasks. The fleet lives in `~/.config/ringmaster/ringmaster.toml` (or `$XDG_CONFIG_HOME/ringmaster/`), outside the install, so upgrades keep it. Set it up with the interactive editor, or start from [`ringmaster.example.toml`](../../ringmaster.example.toml):

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

## Adding a provider

Add providers and agents from the **Providers** view of the web UI, from the TUI (`ringmaster config`), or by editing the fleet config. In the web UI:

1. **Add or edit a provider.** Give it a short name, pick its type, and fill in the base URL and the API key. **Test Connection** checks both before you save.
2. **Add an agent.** Pick the provider; its models are loaded from the provider's `/models` endpoint, so you choose from a list instead of typing an ID. Set the role (*Master* to plan, *Automaton* to do tasks), and press **Ping** to check that the model answers.
3. **Pick the master and the default automaton** with **Set Master** and **Set Automaton**. Each task in a plan can still be given any automaton.

Most providers speak the OpenAI API, so they use the type *OpenAI-compatible*:

| Provider | Type | Base URL |
|---|---|---|
| OpenAI | OpenAI-compatible | `https://api.openai.com/v1` |
| Google Gemini (AI Studio) | OpenAI-compatible | `https://generativelanguage.googleapis.com/v1beta/openai` |
| OpenRouter | OpenAI-compatible | `https://openrouter.ai/api/v1` |
| DeepSeek | OpenAI-compatible | `https://api.deepseek.com/v1` |
| Ollama (local) | OpenAI-compatible | `http://localhost:11434/v1` |
| vLLM (local) | OpenAI-compatible | `http://localhost:8000/v1` |
| Anthropic API | Claude API | `https://api.anthropic.com/v1` (the default) |
| Claude subscription | Claude Subscription | none: uses your local Claude Code login |

Any other service with an OpenAI-compatible `/chat/completions` endpoint works the same way. A trailing `/` on the base URL does not matter.

A few provider notes:

- **Google Gemini.** Use a key from [Google AI Studio](https://aistudio.google.com) (it starts with `AIza`); a Vertex AI key does not work on this endpoint. Gemini lists its models with a `models/` prefix (`models/gemini-...`), which is how they appear in the list. The free tier allows few requests per minute, so run fewer tasks in parallel (`ringmaster run -p 1`) if you see HTTP 429.
- **Automatons need tool calling.** A worker reads and edits files through tools, so pick models that support function calling and keep **Tool calling** on. The master does not need it.

### API keys

The API key field takes the **name of an environment variable** (`GEMINI_API_KEY`), and Ringmaster reads the key from it when it calls the provider. Export the variable in the shell that starts `ringmaster`, and restart it after you set a new one:

```bash
export GEMINI_API_KEY="AIza..."
```

The field also accepts the key itself: a value that starts with `sk-`, or that names no environment variable that is set, is used as the key. That is convenient for a quick test, but the key is then stored in plain text in the fleet config, and is sent to the web UI, which hides it behind dots but can reveal it. Prefer a variable name: the key then stays out of the file and out of the UI.

Supported provider types, in short: OpenAI-compatible APIs, the Anthropic API, and a local Claude subscription through the Claude Code CLI (`claude`, logged in).

## Project manifest

The project's `ringmaster.toml` can tune how its tasks run:

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

A project's `allow` replaces the fleet's list, while its `pass_env` adds to it; a trailing `*` matches a prefix. A task in the plan can also set its own `acceptance_timeout`.

### Budgets

Budgets apply even when you set none: 300,000 tokens per task and 2,000,000 per project by default. The project budget is for the plan being run; a later plan starts from zero. A token read from the prompt cache counts as a tenth of a token, which is what providers charge for it; agents that re-read their conversation from the cache on every step (a Claude subscription agent does) would otherwise hit the limit on small tasks. A task that reaches its budget before finishing is paused, and resuming gives it another full budget. An attempt that finishes its work is always verified, even if its last step went over.

## Commands: presets and always-ask

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

## Web UI port and remote servers

The web UI listens on port 2053 by default. Pick another for one run with `ringmaster --port 4000` or `RINGMASTER_PORT=4000`, or keep it in the project's `ringmaster.toml` (or in the fleet config, for every project):

```toml
[web]
port = 4000
```

The most specific setting wins: the flag, then the environment variable, then the project, then the fleet config.

The web UI listens on localhost only. On a remote server, forward it over SSH:

```bash
ssh -N -L 2053:localhost:2053 user@your-server
```

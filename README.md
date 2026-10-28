<p align="center">
  <img src="docs/assets/ringmaster-animated.svg" width="260" height="260" alt="Ringmaster Animated Kinematics Emblem" />
</p>

# 🎪 Ringmaster — Local AI Agent Orchestration Control Plane

> **Cheap-by-default, Provider-Agnostic, Multi-Agent Software Engineering Control Plane**

Ringmaster acts as an orchestrator (akin to a *Kubernetes Control Plane*) for autonomous AI software engineering teams. It decomposes software requirements into Directed Acyclic Graph (DAG) tasks, dispatches them to parallel workers operating inside isolated `git worktrees`, independently verifies acceptance criteria, and serializes clean merges into the `main` branch.

---

## 🌟 Key Features

- **Provider-Agnostic:** Seamless integration with OpenAI-compatible APIs (Ollama, vLLM, OpenRouter, 9router, DeepSeek), official Anthropic API, and local Claude Subscriptions (`~/.claude/` credentials via Claude CLI).
- **Steampunk / Circus Terminal & Web UI:** Interactive 2-step setup with live provider search, visual role indicators (Master vs Worker), and real-time model ping test.
- **Cryptographic Approval Gate:** Master AI designs the DAG workflow and min-max token estimates. Execution requires approval backed by canonical SHA-256 plan hashes.
- **Strict Sandbox Security:** Path traversal blocker, command allowlist verification, and environment scrubbing (zero API key leakage).
- **Autonomous Worker Runtime:** Equipped with 5 native tools (`read_file`, `write_file`, `list_dir`, `search`, `run_cmd`) with execution timeouts and buffer truncation.
- **Independent Verification & Retry:** Master autonomously executes test commands (does not rely on blind worker reports). Automatic retries feed test failures back to the worker prompt.
- **Serialized Git Merge Queue:** Each worker operates in an isolated worktree branch (`rm/Txx`). Verified tasks merge cleanly into `main` with automatic conflict aborts.
- **Human-in-the-Loop:** Interactive `answer` command and web UI actions to unblock tasks requiring human decisions.

---

## 🚀 Quickstart Guide

### 1. Build & Install
```bash
go build -o bin/ringmaster ./cmd/ringmaster
```

### 2. Configure Providers & Models (Interactive)
Run the root command to choose between the Web UI Dashboard or Terminal TUI:
```bash
ringmaster
```
- Or run the TUI directly: `ringmaster config`
- Register providers (OpenAI, Anthropic, Claude CLI, Ollama, OpenRouter, etc.).
- Highlight an agent and press `p` to test live chat ping latency.
- Designate your Master Planner (`m`) and Default Worker (`w`).

### 3. Initialize a New Project
```bash
ringmaster init my-project
```
This creates `my-project/` with a starter `BRIEF.md` specification and a clean git repository on branch `main`.

To initialize inside an existing codebase:
```bash
cd my-existing-repo
ringmaster init
```

### 4. Planning & Approval Gate
Describe your application requirements in `BRIEF.md`, then ask the Master Planner to architect the DAG:
```bash
ringmaster plan my-project
```
Review the DAG task preview table and token range estimates. Press `y` to approve!

### 5. Run Parallel Orchestration
```bash
ringmaster run my-project --parallel 4
```

### 6. Monitor Status & Token Analytics
```bash
# Live visual pipeline monitoring
ringmaster status my-project --watch

# In-depth token breakdown and budget tracking
ringmaster usage my-project
```

---

## 📂 Project Directory Structure

```
<project_root>/
├── BRIEF.md                 # Project requirements and scope
├── plan.json                # Master AI DAG plan definition
├── plan.approved.json       # User-approved plan hash (canonical SHA-256)
├── tasks/                   # Detailed task specifications (T01-*.md, T02-*.md)
├── outputs/                 # Worker verification reports (T01.md, T02.md)
├── repo/                    # Main git repository (branch 'main')
├── .worktrees/              # Isolated git worktrees per task (created dynamically)
└── .ringmaster/
    ├── state.json           # Execution state & token accumulation (atomic)
    └── logs/
        ├── events.jsonl     # Append-only audit trail
        └── Txx-attemptN.jsonl # Attempt dialogue transcripts & tool execution logs
```

---

## 📜 License
Ringmaster is open-source software licensed under the [MIT License](LICENSE).


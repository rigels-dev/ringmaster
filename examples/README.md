# Ringmaster Examples

A collection of ready-to-use project templates and specifications for Ringmaster orchestration.

---

## 1. `todo-app` (Go CLI Todo Application)
A lightweight starter project for testing end-to-end multi-agent orchestration:
- **Specification:** `examples/todo-app/BRIEF.md`
- **Generated DAG Tasks:**
  - `T01`: Core Todo struct and storage functions (`todo.go`)
  - `T02`: Unit tests and edge-case testing (`todo_test.go`)
  - `T03`: CLI interactive runner (`main.go`)

### How to Run:

#### Scenario A: Initialize into a new project
```bash
ringmaster init todo-demo
cp examples/todo-app/BRIEF.md todo-demo/BRIEF.md
ringmaster plan todo-demo
ringmaster run todo-demo
```

#### Scenario B: Blank init in current working directory
```bash
mkdir -p /tmp/my-blank-todo && cd /tmp/my-blank-todo
ringmaster init
# Automatically initializes current folder as a Ringmaster project!
```


# Skills

[← README](../../README.md)

A skill is a folder holding a `SKILL.md`: instructions for one kind of work (your code style, how to write a migration, a review checklist), in the format agent tools already share. Every worker can use it, whichever model it runs on. Install one by copying or symlinking its folder:

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

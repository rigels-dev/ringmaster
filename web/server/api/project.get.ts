import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

export default defineEventHandler((event) => {
  // 1. Resolve project directory in priority:
  // a. Environment variable RINGMASTER_PROJECT_DIR
  // b. Global active project file (~/.ringmaster/active_project)
  // c. Process CWD if contains ringmaster project markers
  // d. Fallback to /home/ubuntu/project/ringmaster/examples/todo-app
  let projectDir = process.env.RINGMASTER_PROJECT_DIR

  if (!projectDir || projectDir.includes('.openclaw')) {
    const activeFile = path.join(os.homedir(), '.ringmaster', 'active_project')
    if (fs.existsSync(activeFile)) {
      try {
        const stored = fs.readFileSync(activeFile, 'utf-8').trim()
        if (stored && fs.existsSync(stored)) {
          projectDir = stored
        }
      } catch {}
    }
  }

  const cwd = process.cwd()
  if (!projectDir) {
    if (fs.existsSync(path.join(cwd, 'ringmaster.toml')) || fs.existsSync(path.join(cwd, 'BRIEF.md'))) {
      projectDir = cwd
    } else {
      const exampleDir = '/home/ubuntu/project/ringmaster/examples/todo-app'
      projectDir = fs.existsSync(exampleDir) ? exampleDir : cwd
    }
  }

  const projectName = path.basename(projectDir)

  // 1. Read BRIEF.md
  let brief = ''
  const briefPath = path.join(projectDir, 'BRIEF.md')
  if (fs.existsSync(briefPath)) {
    try {
      brief = fs.readFileSync(briefPath, 'utf-8')
    } catch {}
  }

  // 2. Read plan.json / plan.approved.json
  let plan = null
  const planPath = path.join(projectDir, 'plan.json')
  if (fs.existsSync(planPath)) {
    try {
      plan = JSON.parse(fs.readFileSync(planPath, 'utf-8'))
    } catch {}
  }

  // 3. Read state.json
  let state = null
  const statePath = path.join(projectDir, '.ringmaster', 'state.json')
  if (fs.existsSync(statePath)) {
    try {
      state = JSON.parse(fs.readFileSync(statePath, 'utf-8'))
    } catch {}
  }

  // 4. Read ringmaster.toml for active master & workers
  let master = 'none'
  let workers: string[] = []
  const tomlPath = path.join(projectDir, 'ringmaster.toml')
  const rootToml = '/home/ubuntu/project/ringmaster/ringmaster.toml'
  const targetToml = fs.existsSync(tomlPath) ? tomlPath : rootToml

  if (fs.existsSync(targetToml)) {
    try {
      const content = fs.readFileSync(targetToml, 'utf-8')
      const masterMatch = content.match(/default_master\s*=\s*['"]([^'"]+)['"]/)
      if (masterMatch) master = masterMatch[1]
      
      const agentMatches = content.matchAll(/\[agents\.([a-zA-Z0-9_\-]+)\]/g)
      for (const m of agentMatches) {
        if (m[1] !== master) workers.push(m[1])
      }
    } catch {}
  }

  const isInitialized = fs.existsSync(briefPath) || fs.existsSync(tomlPath)
  const isBlank = !plan || !plan.tasks || plan.tasks.length === 0

  return {
    name: projectName,
    path: projectDir,
    isInitialized,
    isBlank,
    brief,
    plan,
    state,
    master,
    workers
  }
})

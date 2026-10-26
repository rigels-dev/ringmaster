import fs from 'node:fs'
import path from 'node:path'

export default defineEventHandler((event) => {
  // Check RINGMASTER_PROJECT_DIR from environment or fallback to current directory
  const projectDir = process.env.RINGMASTER_PROJECT_DIR || process.cwd()
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

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

  // 4. Resolve Master Planner & Workers configuration:
  // Global fleet config is in /home/ubuntu/project/ringmaster/ringmaster.toml
  // Local project manifest is in <projectDir>/ringmaster.toml
  let master = 'none'
  let workers: string[] = []

  const rootToml = '/home/ubuntu/project/ringmaster/ringmaster.toml'
  const projectToml = path.join(projectDir, 'ringmaster.toml')

  // Helper to parse master & workers from toml content
  const parseFleet = (filePath: string) => {
    if (!fs.existsSync(filePath)) return
    try {
      const content = fs.readFileSync(filePath, 'utf-8')
      const masterMatch = content.match(/default_master\s*=\s*['"]([^'"]+)['"]/)
      if (masterMatch && masterMatch[1] && masterMatch[1].trim() !== '') {
        const potentialMaster = masterMatch[1].trim()
        // Check if this potentialMaster actually has role = 'master' in [agents]
        const agentBlockRegex = new RegExp(`\\[agents\\.${potentialMaster.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}\\]([\\s\\S]*?)(?=\\n\\[|$)`)
        const agentBlock = content.match(agentBlockRegex)
        if (agentBlock && agentBlock[1].includes("role = 'master'")) {
          master = potentialMaster
        } else {
          // If default_master has role = 'worker', search for any other agent with role = 'master'
          const allAgents = content.matchAll(/\[agents\.([^\]]+)\]([\s\S]*?)(?=\n\[|$)/g)
          let foundMaster = false
          for (const a of allAgents) {
            if (a[2].includes("role = 'master'")) {
              master = a[1].trim()
              foundMaster = true
              break
            }
          }
          if (!foundMaster) {
            master = 'none'
          }
        }
      } else {
        master = 'none'
      }
      
      const agentMatches = content.matchAll(/\[agents\.([^\]]+)\]/g)
      for (const m of agentMatches) {
        const aName = m[1].trim()
        if (aName !== master && !workers.includes(aName)) {
          workers.push(aName)
        }
      }
    } catch {}
  }

  // Read root fleet first (where providers & agents are configured)
  parseFleet(rootToml)
  // If local project toml has specific overrides, apply them ONLY if it defines default_master or agents
  if (projectToml !== rootToml && fs.existsSync(projectToml)) {
    try {
      const localContent = fs.readFileSync(projectToml, 'utf-8')
      if (localContent.includes('default_master') || localContent.includes('[agents')) {
        parseFleet(projectToml)
      }
    } catch {}
  }

  const isInitialized = fs.existsSync(briefPath) || fs.existsSync(projectToml)
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

import fs from 'node:fs'
import path from 'node:path'
import { rootTomlPath, resolveProjectDir } from '../utils/ringmaster'

export default defineEventHandler((event) => {
  const projectDir = resolveProjectDir()
  const projectName = path.basename(projectDir)

  // 1. Read BRIEF.md
  let brief = ''
  const briefPath = path.join(projectDir, '.ringmaster', 'BRIEF.md')
  if (fs.existsSync(briefPath)) {
    try {
      brief = fs.readFileSync(briefPath, 'utf-8')
    } catch {}
  }

  // 2. Read plan.json / plan.approved.json
  let plan = null
  const planPath = path.join(projectDir, '.ringmaster', 'plan.json')
  if (fs.existsSync(planPath)) {
    try {
      plan = JSON.parse(fs.readFileSync(planPath, 'utf-8'))
    } catch {}
  }

  // plan.json carries no descriptions; take them from the tasks/Txx-*.md bodies
  const tasksDir = path.join(projectDir, '.ringmaster', 'tasks')
  if (plan?.tasks && fs.existsSync(tasksDir)) {
    const specFiles = fs.readdirSync(tasksDir).filter((f) => f.endsWith('.md'))
    for (const t of plan.tasks) {
      const file = specFiles.find((f) => f.startsWith(t.id))
      if (!t.description && file) {
        try {
          const content = fs.readFileSync(path.join(tasksDir, file), 'utf-8').trim()
          const body = content.startsWith('---') ? content.slice(3).split('---').slice(1).join('---') : content
          t.description = body.split('## Human Answers')[0].trim()
        } catch {}
      }
    }
  }

  // 3. Read state.json
  let state = null
  const statePath = path.join(projectDir, '.ringmaster', 'state.json')
  if (fs.existsSync(statePath)) {
    try {
      state = JSON.parse(fs.readFileSync(statePath, 'utf-8'))
    } catch {}
  }

  // 4. Resolve Master Planner & Workers from the global fleet config
  // (<ringmaster root>/ringmaster.toml). The project's own ringmaster.toml is
  // its manifest and never defines agents.
  let master = 'none'
  let workers: string[] = []

  const rootToml = rootTomlPath()
  const projectToml = path.join(projectDir, 'ringmaster.toml')

  // Helper to parse master & workers from toml content
  const parseFleet = (filePath: string) => {
    if (!fs.existsSync(filePath)) return
    try {
      const content = fs.readFileSync(filePath, 'utf-8')
      const masterMatch = content.match(/default_master\s*=\s*['"]([^'"]+)['"]/)
      if (masterMatch && masterMatch[1] && masterMatch[1].trim() !== '') {
        const potentialMaster = masterMatch[1].trim().replace(/^['"]|['"]$/g, '')
        // Check if this potentialMaster actually has role = 'master' in [agents]
        const agentBlockRegex = new RegExp(`\\[agents\\.(?:'|")?${potentialMaster.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}(?:'|")?\\]([\\s\\S]*?)(?=\\n\\[|$)`)
        const agentBlock = content.match(agentBlockRegex)
        if (agentBlock && agentBlock[1].includes("role = 'master'")) {
          master = potentialMaster
        } else {
          // If default_master has role = 'worker', search for any other agent with role = 'master'
          const allAgents = content.matchAll(/\[agents\.(?:'|")?([^'\]]+)(?:'|")?\]([\s\S]*?)(?=\n\[|$)/g)
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
        const aName = m[1].trim().replace(/^['"]|['"]$/g, '')
        if (aName !== master && !workers.includes(aName)) {
          workers.push(aName)
        }
      }
    } catch {}
  }

  parseFleet(rootToml)

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

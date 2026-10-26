import fs from 'node:fs'
import path from 'node:path'

export default defineEventHandler(() => {
  const projectDir = process.env.RINGMASTER_PROJECT_DIR || process.cwd()
  const localToml = path.join(projectDir, 'ringmaster.toml')
  const rootToml = '/home/ubuntu/project/ringmaster/ringmaster.toml'
  const targetToml = fs.existsSync(localToml) ? localToml : rootToml

  const providers: any[] = []
  const agents: any[] = []
  let defaultMaster = ''
  let defaultWorker = ''

  if (fs.existsSync(targetToml)) {
    try {
      const content = fs.readFileSync(targetToml, 'utf-8')
      
      // Robust Regex for provider & agent names allowing dots, slashes, hyphens, underscores
      const provRegex = /\[providers\.([^\]]+)\]([\s\S]*?)(?=\n\[|$)/g
      let match: RegExpExecArray | null
      while ((match = provRegex.exec(content)) !== null) {
        const id = match[1].trim()
        const block = match[2]
        const typeMatch = block.match(/type\s*=\s*['"]([^'"]+)['"]/)
        const urlMatch = block.match(/base_url\s*=\s*['"]([^'"]+)['"]/)
        const envMatch = block.match(/api_key_env\s*=\s*['"]([^'"]+)['"]/)
        providers.push({
          id,
          type: typeMatch ? typeMatch[1] : 'openai',
          base_url: urlMatch ? urlMatch[1] : '',
          api_key_env: envMatch ? envMatch[1] : ''
        })
      }

      const agentRegex = /\[agents\.([^\]]+)\]([\s\S]*?)(?=\n\[|$)/g
      while ((match = agentRegex.exec(content)) !== null) {
        const id = match[1].trim()
        const block = match[2]
        const provMatch = block.match(/provider\s*=\s*['"]([^'"]+)['"]/)
        const modelMatch = block.match(/model\s*=\s*['"]([^'"]+)['"]/)
        const roleMatch = block.match(/role\s*=\s*['"]([^'"]+)['"]/)
        const tcMatch = block.match(/tool_calling\s*=\s*(true|false)/)
        agents.push({
          id,
          provider: provMatch ? provMatch[1] : '',
          model: modelMatch ? modelMatch[1] : '',
          role: roleMatch ? roleMatch[1] : 'worker',
          tool_calling: tcMatch ? tcMatch[1] === 'true' : true
        })
      }

      const dmMatch = content.match(/default_master\s*=\s*['"]([^'"]+)['"]/)
      if (dmMatch && dmMatch[1].trim() !== '') {
        const potentialDm = dmMatch[1].trim()
        const targetAgent = agents.find(a => a.id === potentialDm)
        if (targetAgent && targetAgent.role === 'master') {
          defaultMaster = potentialDm
        } else {
          const masterAgent = agents.find(a => a.role === 'master')
          defaultMaster = masterAgent ? masterAgent.id : ''
        }
      } else {
        const masterAgent = agents.find(a => a.role === 'master')
        defaultMaster = masterAgent ? masterAgent.id : ''
      }
      const dwMatch = content.match(/default_worker\s*=\s*['"]([^'"]+)['"]/)
      if (dwMatch) defaultWorker = dwMatch[1]

    } catch (err: any) {
      return { ok: false, error: err.message }
    }
  }

  return {
    ok: true,
    configPath: targetToml,
    providers,
    agents,
    defaultMaster,
    defaultWorker
  }
})

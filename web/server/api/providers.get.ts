import { spawn } from 'node:child_process'
import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir } from '../utils/ringmaster'

export default defineEventHandler(async () => {
  const projectDir = resolveProjectDir()
  const localToml = path.join(projectDir, 'ringmaster.toml')
  const rootToml = rootTomlPath()
  const targetToml = fs.existsSync(localToml) ? localToml : rootToml

  // Call Go binary directly: ringmaster config-api
  return new Promise((resolve) => {
    const ringmasterBin = ringmasterBinPath()
    const child = spawn(ringmasterBin, ['config-api'])

    let stdout = ''
    let stderr = ''

    child.stdout.on('data', (d) => { stdout += d.toString() })
    child.stderr.on('data', (d) => { stderr += d.toString() })

    child.on('close', (code) => {
      try {
        const json = JSON.parse(stdout)
        if (json.ok && json.data && json.data.config) {
          const cfg = json.data.config
          const providers: any[] = []
          const rawProviders = cfg.providers || cfg.Providers || {}
          for (const [id, p] of Object.entries(rawProviders)) {
            const prov = p as any
            providers.push({
              id,
              type: prov.type || prov.Type || 'openai',
              base_url: prov.base_url || prov.BaseURL || '',
              api_key_env: prov.api_key_env || prov.APIKeyEnv || ''
            })
          }

          const agents: any[] = []
          const rawAgents = cfg.agents || cfg.Agents || {}
          for (const [id, a] of Object.entries(rawAgents)) {
            const ag = a as any
            agents.push({
              id,
              provider: ag.provider || ag.Provider || '',
              model: ag.model || ag.Model || '',
              role: ag.role || ag.Role || 'worker',
              tool_calling: typeof ag.tool_calling === 'boolean' ? ag.tool_calling : (typeof ag.ToolCalling === 'boolean' ? ag.ToolCalling : true)
            })
          }

          const routing = cfg.routing || cfg.Routing || {}
          resolve({
            ok: true,
            configPath: json.data.config_path,
            providers,
            agents,
            defaultMaster: routing.default_master || routing.DefaultMaster || '',
            defaultWorker: routing.default_worker || routing.DefaultWorker || ''
          })
        } else {
          resolve({ ok: false, error: json.error || 'Failed to read config' })
        }
      } catch (err: any) {
        resolve({
          ok: false,
          error: stderr || stdout || err.message
        })
      }
    })

    const payload = JSON.stringify({
      action: 'get',
      config_path: targetToml
    })
    child.stdin.write(payload)
    child.stdin.end()
  })
})

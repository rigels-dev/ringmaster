import { spawn } from 'node:child_process'
import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir } from '../utils/ringmaster'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const rootToml = rootTomlPath()
  const projectDir = resolveProjectDir()
  const localToml = path.join(projectDir, 'ringmaster.toml')

  // Providers are ALWAYS stored globally in rootToml!
  // Agents can be stored in local project if action is agent-specific, else fallback to rootToml
  let targetToml = rootToml
  if (body.action === 'add_agent' || body.action === 'set_role' || body.action === 'delete_agent') {
    if (fs.existsSync(localToml)) {
      targetToml = localToml
    }
  }

  // Call Go binary directly: ringmaster config-api
  // This ensures 100% Go TOML parser, validation, and zero regex fragility
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
        resolve(json)
      } catch (err: any) {
        resolve({
          ok: false,
          error: stderr || stdout || err.message
        })
      }
    })

    // Pipe JSON request payload to stdin of Go binary
    const payload = JSON.stringify({
      config_path: targetToml,
      ...body
    })
    child.stdin.write(payload)
    child.stdin.end()
  })
})

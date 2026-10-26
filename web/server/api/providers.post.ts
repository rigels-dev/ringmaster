import { spawn } from 'node:child_process'
import path from 'node:path'
import fs from 'node:fs'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const projectDir = process.env.RINGMASTER_PROJECT_DIR || process.cwd()
  const localToml = path.join(projectDir, 'ringmaster.toml')
  const rootToml = '/home/ubuntu/project/ringmaster/ringmaster.toml'
  const targetToml = fs.existsSync(localToml) ? localToml : rootToml

  // Call Go binary directly: ringmaster config-api
  // This ensures 100% Go TOML parser, validation, and zero regex fragility
  return new Promise((resolve) => {
    const ringmasterBin = '/home/ubuntu/project/ringmaster/bin/ringmaster'
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

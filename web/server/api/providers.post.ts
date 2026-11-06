import { spawn } from 'node:child_process'
import { ringmasterBinPath, rootTomlPath } from '../utils/ringmaster'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  // Providers and agents both live in the global fleet config. A project's
  // ringmaster.toml is its manifest: once it holds any agent, config loading
  // stops merging the global fleet and the project loses every provider.
  const targetToml = rootTomlPath()

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

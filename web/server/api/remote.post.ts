import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// { action: 'set', url }            point origin at a repository
// { action: 'push', mainOnly? }     push main and the task branches to it
//
// Both go through the CLI, which validates the URL and never forces a push.
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  const bin = ringmasterBinPath()
  const cwd = resolveProjectDir()

  if (body.action === 'set') {
    const url = typeof body.url === 'string' ? body.url.trim() : ''
    if (!url) return { ok: false, error: 'Enter the repository URL' }
    try {
      // "--" keeps a URL that starts with a dash from being read as a flag
      await runCommand(bin, ['remote', 'set', '--config', rootTomlPath(), '--', url], { cwd, timeout: 15000 })
      return { ok: true, url }
    } catch (err: any) {
      return { ok: false, error: cliError(err, 'Failed to set the remote') }
    }
  }

  if (body.action === 'push') {
    const args = ['push', '--json', '--config', rootTomlPath()]
    if (body.mainOnly === true) args.push('--main-only')
    try {
      const { stdout } = await runCommand(bin, args, { cwd, timeout: 200000 })
      return JSON.parse(stdout)
    } catch (err: any) {
      // A failed push still prints its report on stdout (per-branch results and the reason)
      try {
        const report = JSON.parse(String(err.stdout || ''))
        if (report && typeof report === 'object') return report
      } catch {}
      return { ok: false, results: [], error: cliError(err, 'The push failed') }
    }
  }

  return { ok: false, error: "Unknown action; use 'set' or 'push'" }
})

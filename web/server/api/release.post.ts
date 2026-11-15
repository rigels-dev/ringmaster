import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// { version, push? }  tag main as a release, and optionally push main and
// that tag. The CLI validates the version and never moves an existing tag.
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  const version = typeof body.version === 'string' ? body.version.trim() : ''
  if (!version) return { ok: false, error: 'Enter a version' }

  const args = ['release', '--json', '--config', rootTomlPath()]
  if (body.push === true) args.push('--push')
  // "--" keeps a version that starts with a dash from being read as a flag
  args.push('--', version)

  try {
    const { stdout } = await runCommand(ringmasterBinPath(), args, { cwd: resolveProjectDir(), timeout: 200000 })
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    // A failure still prints its report: a push can fail after the tag was made
    try {
      const report = JSON.parse(String(err.stdout || ''))
      if (report && typeof report === 'object') return { ok: false, ...report }
    } catch {}
    return { ok: false, error: cliError(err, 'The release failed') }
  }
})

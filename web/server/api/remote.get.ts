import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// The project's git remote and the push state of main and the task branches
// (ringmaster remote --json). Read from local refs; nothing is contacted.
export default defineEventHandler(async () => {
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['remote', '--json', '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    const status = JSON.parse(stdout)
    return { ok: true, name: status.name, url: status.url || '', branches: status.branches || [], author: status.author || null }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to read the remote') }
  }
})

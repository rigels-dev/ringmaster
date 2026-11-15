import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// The last release, whether main is already released, the tasks still
// unfinished, and suggested next versions (ringmaster release --json).
export default defineEventHandler(async () => {
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['release', '--json', '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to read the release information') }
  }
})

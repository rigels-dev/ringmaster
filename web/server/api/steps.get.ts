import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// The project's finished steps, oldest first, with their releases
// (ringmaster steps --json). Listing also records a plan that has just finished.
export default defineEventHandler(async () => {
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['steps', '--json', '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    return { ok: true, steps: JSON.parse(stdout).steps || [] }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to read the steps') }
  }
})

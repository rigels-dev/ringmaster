import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// The command presets, and what the project's automatons may run now
// (ringmaster presets --json)
export default defineEventHandler(async () => {
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['presets', '--json', '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to read the presets') }
  }
})

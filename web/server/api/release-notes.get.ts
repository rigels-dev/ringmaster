import { ringmasterBinPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'
import { notesArgs } from '../utils/releaseNotes'

// ?step=<n>&version=<v>  a draft of the release notes, from the release's
// tasks (ringmaster release notes --json). No model is called.
export default defineEventHandler(async (event) => {
  try {
    const { stdout } = await runCommand(ringmasterBinPath(), notesArgs(getQuery(event), false), {
      cwd: resolveProjectDir(), timeout: 15000
    })
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'The release notes could not be drafted') }
  }
})

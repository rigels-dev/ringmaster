import { ringmasterBinPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'
import { notesArgs } from '../utils/releaseNotes'

// { step, version }  have the master agent write the summary of the release
// notes (ringmaster release notes --ai --json). One model call, on the
// user's quota, made only when they ask for it.
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  try {
    const { stdout } = await runCommand(ringmasterBinPath(), notesArgs(body, true), {
      cwd: resolveProjectDir(), timeout: 300000
    })
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'The summary could not be written') }
  }
})

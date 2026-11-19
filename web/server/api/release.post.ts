import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'
import { notesOf } from '../utils/releaseNotes'

// { version, push?, step?, notes?, githubRelease? }  tag main (or a finished
// step) as a release with notes, optionally push main and that tag, and then
// create its GitHub release page. The CLI validates the version and never
// moves an existing tag.
// { action: 'page', version, notes? }  create the GitHub release page of a
// release that is already pushed.
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  const version = typeof body.version === 'string' ? body.version.trim() : ''
  if (!version) return { ok: false, error: 'Enter a version' }
  const { notes, error } = notesOf(body)
  if (error) return { ok: false, error }

  let args: string[]
  if (body.action === 'page') {
    args = ['release', 'page', '--json', '--config', rootTomlPath()]
  } else {
    args = ['release', '--json', '--config', rootTomlPath()]
    if (body.push === true) args.push('--push')
    if (body.push === true && body.githubRelease === true) args.push('--github-release')
    // A finished step is released where it ended
    const step = Number(body.step)
    if (Number.isInteger(step) && step > 0) args.push('--step', String(step))
  }
  // The notes go through stdin, so no file is left behind
  if (notes !== undefined) args.push('--notes-file', '-')
  // "--" keeps a version that starts with a dash from being read as a flag
  args.push('--', version)

  try {
    const { stdout } = await runCommand(ringmasterBinPath(), args, {
      cwd: resolveProjectDir(), timeout: 200000, input: notes ?? ''
    })
    return { ok: true, ...JSON.parse(stdout) }
  } catch (err: any) {
    // A failure still prints its report: a push can fail after the tag was made
    try {
      const report = JSON.parse(String(err.stdout || ''))
      if (report && typeof report === 'object' && !Array.isArray(report)) return { ok: false, ...report }
    } catch {}
    return { ok: false, error: cliError(err, body.action === 'page' ? 'The release page could not be made' : 'The release failed') }
  }
})

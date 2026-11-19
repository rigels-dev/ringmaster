import { rootTomlPath } from './ringmaster'

// Release notes as the web UI sends them: text, at most MAX_NOTES characters
// (the CLI refuses more). Anything else counts as no notes, so the CLI drafts them.
export const MAX_NOTES = 20000

export function notesOf(body: any): { notes?: string; error?: string } {
  if (typeof body?.notes !== 'string') return {}
  if (body.notes.length > MAX_NOTES) return { error: `The release notes are too long (${MAX_NOTES} characters at most)` }
  return { notes: body.notes }
}

// The arguments of `ringmaster release notes` for a step and version given by the web UI
export function notesArgs(input: any, ai: boolean): string[] {
  const args = ['release', 'notes', '--json', '--config', rootTomlPath()]
  const step = Number(input?.step)
  if (Number.isInteger(step) && step > 0) args.push('--step', String(step))
  const version = typeof input?.version === 'string' ? input.version.trim() : ''
  // "=" keeps a version that starts with a dash a value, not a flag
  if (version && version.length <= 100) args.push(`--version=${version}`)
  if (ai) args.push('--ai')
  return args
}

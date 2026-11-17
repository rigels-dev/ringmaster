import fs from 'node:fs'
import path from 'node:path'

// Shared with the UI, which groups the chat by step the same way
export { isGeneratePlanMessage } from '../../utils/chatSteps.js'

// What earlier plans already built, for the Master in chat: the tasks that
// are done and merged, read from the project's state. Empty for a new project.
export function finishedWorkNote(projectDir: string): string {
  let state: any = null
  let plan: any = null
  try {
    state = JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'state.json'), 'utf-8'))
  } catch {
    return ''
  }
  try {
    plan = JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'plan.json'), 'utf-8'))
  } catch {}

  const titles: Record<string, string> = {}
  for (const t of plan?.tasks || []) titles[t.id] = t.title

  const done = Object.entries<any>(state?.task_states || {})
    .filter(([, ts]) => ts?.status === 'COMPLETED')
    .map(([id, ts]) => ({ id, title: ts.title || titles[id] || '' }))
    .sort((a, b) => a.id.localeCompare(b.id, undefined, { numeric: true }))
  if (!done.length) return ''
  const line = (t: { id: string; title: string }) => `- ${t.id}${t.title ? ': ' + t.title : ''}`

  // Grouped by the step each task was part of, when the steps are recorded
  let steps: any[] = []
  try {
    steps = JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'steps.json'), 'utf-8'))?.steps || []
  } catch {}
  const lines: string[] = []
  const placed = new Set<string>()
  for (const st of steps) {
    const own = done.filter((t) => st.tasks?.includes(t.id))
    if (!own.length) continue
    lines.push(`Step ${st.number}${st.name ? ' (' + st.name + ')' : ''}:`)
    for (const t of own) {
      lines.push('  ' + line(t))
      placed.add(t.id)
    }
  }
  const rest = done.filter((t) => !placed.has(t.id))
  if (rest.length) {
    // Without recorded steps the list stays flat
    const indent = lines.length ? '  ' : ''
    if (lines.length) lines.push('Also done:')
    for (const t of rest) lines.push(indent + line(t))
  }

  return [
    'PROJECT STATUS: this project already has finished work. These tasks are done and merged into main:',
    ...lines,
    'The user is now discussing a further change. Treat what is listed as already built: do not propose building it again, and plan only what is new or different.'
  ].join('\n')
}

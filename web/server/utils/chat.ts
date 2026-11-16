import fs from 'node:fs'
import path from 'node:path'

// The message the UI adds to the chat when "Generate plan" is clicked. It is
// a button press, not a requirement, so it is left out of the brief and of
// the conversation the Master sees. The older wording is still recognised.
export function isGeneratePlanMessage(text: unknown): boolean {
  const t = String(text ?? '')
  return t.includes('⚡ Generate DAG Plan') || t.trim() === 'Generate the plan from this conversation.'
}

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

  const lines = done.map((t) => `- ${t.id}${t.title ? ': ' + t.title : ''}`)
  return [
    'PROJECT STATUS: this project already has finished work. These tasks are done and merged into main:',
    ...lines,
    'The user is now discussing a further change. Treat what is listed as already built: do not propose building it again, and plan only what is new or different.'
  ].join('\n')
}

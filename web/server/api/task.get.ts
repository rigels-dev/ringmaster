import fs from 'node:fs'
import path from 'node:path'
import { resolveProjectDir } from '../utils/ringmaster'

// Everything a human needs to trace one task: spec, state, audit events,
// the worker report and the per-attempt transcripts from its worktree.
export default defineEventHandler((event) => {
  const id = String(getQuery(event).id || '')
  if (!/^T\d{2,3}$/.test(id)) {
    return { ok: false, error: `Invalid task id '${id}'` }
  }

  const projectDir = resolveProjectDir()
  const worktree = path.join(projectDir, '.ringmaster', 'worktrees', id)

  const readText = (file: string) => {
    try {
      return fs.readFileSync(file, 'utf-8')
    } catch {
      return ''
    }
  }
  const readJsonl = (file: string) =>
    readText(file)
      .split('\n')
      .filter(Boolean)
      .map((line) => {
        try {
          return JSON.parse(line)
        } catch {
          return { raw: line }
        }
      })

  // Task spec markdown (tasks/Txx-*.md)
  let specFile = ''
  let spec = ''
  const tasksDir = path.join(projectDir, '.ringmaster', 'tasks')
  if (fs.existsSync(tasksDir)) {
    const match = fs.readdirSync(tasksDir).find((f) => f.startsWith(id) && f.endsWith('.md'))
    if (match) {
      specFile = path.join(tasksDir, match)
      spec = readText(specFile)
    }
  }

  let state = null
  try {
    state = JSON.parse(readText(path.join(projectDir, '.ringmaster', 'state.json')))?.task_states?.[id] || null
  } catch {}

  const events = readJsonl(path.join(projectDir, '.ringmaster', 'logs', 'events.jsonl'))
    .filter((e: any) => e.task_id === id)

  // Live files in the worktree first; the project's archive once it is merged and removed
  const archivedReport = path.join(projectDir, '.ringmaster', 'outputs', `${id}.md`)
  const worktreeReport = path.join(worktree, '.ringmaster', 'outputs', `${id}.md`)
  const reportFile = fs.existsSync(worktreeReport) ? worktreeReport : archivedReport
  const report = readText(reportFile)

  // Transcripts: the live worktree's copy wins over the project's archive
  const attempts: Array<{ attempt: number; file: string; entries: any[] }> = []
  const logDirs = [path.join(worktree, '.ringmaster', 'logs'), path.join(projectDir, '.ringmaster', 'logs')]
  for (const logsDir of logDirs) {
    if (!fs.existsSync(logsDir)) continue
    for (const f of fs.readdirSync(logsDir)) {
      const m = f.match(new RegExp(`^${id}-attempt(\\d+)\\.jsonl$`))
      if (m && !attempts.some((a) => a.attempt === Number(m[1]))) {
        const file = path.join(logsDir, f)
        attempts.push({ attempt: Number(m[1]), file, entries: readJsonl(file) })
      }
    }
  }
  attempts.sort((a, b) => a.attempt - b.attempt)

  return {
    ok: true,
    id,
    branch: `rm/${id}`,
    worktree: fs.existsSync(worktree) ? worktree : '',
    specFile,
    spec,
    state,
    events,
    reportFile: report ? reportFile : '',
    report,
    attempts
  }
})

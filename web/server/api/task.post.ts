import fs from 'node:fs'
import path from 'node:path'
import { resolveProjectDir } from '../utils/ringmaster'

// Queue the user's answer for a task waiting on them. The scheduler applies
// .ringmaster/answers/<id>.json on its next tick, or on the next run; when no
// run is active we start one so the task resumes right away.
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  const id = String(body.id || '')
  if (!/^T\d{2,3}$/.test(id)) {
    return { ok: false, error: `Invalid task id '${id}'` }
  }

  const replies: Record<string, string> = {}
  for (const [k, v] of Object.entries(body.replies || {})) {
    if (typeof v === 'string' && v.trim()) replies[k] = v.trim()
  }
  const allow = (Array.isArray(body.allow) ? body.allow : [])
    .map((c: unknown) => String(c).trim())
    .filter(Boolean)
  const scope = body.scope === 'project' ? 'project' : 'task'
  const text = typeof body.text === 'string' ? body.text.trim() : ''
  // New acceptance commands, one per entry. Ringmaster checks them again
  // when it applies the answer, and keeps the task waiting if one cannot run.
  const acceptance = [...new Set((Array.isArray(body.acceptance) ? body.acceptance : [])
    .map((c: unknown) => String(c).trim())
    .filter(Boolean))] as string[]
  if (Array.isArray(body.acceptance) && !acceptance.length) {
    return { ok: false, error: 'A task needs at least one acceptance command.' }
  }
  if (acceptance.some((c) => c.length > 500 || /[\r\n]/.test(c))) {
    return { ok: false, error: 'Each acceptance command is one line, at most 500 characters.' }
  }

  if (!text && !allow.length && !acceptance.length && !Object.keys(replies).length) {
    return { ok: false, error: 'Nothing to answer: add a reply, an approval or a note.' }
  }

  const projectDir = resolveProjectDir()
  const answersDir = path.join(projectDir, '.ringmaster', 'answers')
  fs.mkdirSync(answersDir, { recursive: true })
  const answer: Record<string, any> = { task_id: id, replies, allow, scope, text }
  if (acceptance.length) answer.acceptance = acceptance

  // Merge with an answer still waiting in the inbox rather than dropping it
  const inboxFile = path.join(answersDir, `${id}.json`)
  if (fs.existsSync(inboxFile)) {
    try {
      const prev = JSON.parse(fs.readFileSync(inboxFile, 'utf-8'))
      answer.replies = { ...(prev.replies || {}), ...replies }
      answer.allow = [...new Set([...(prev.allow || []), ...allow])]
      if (!allow.length && prev.scope) answer.scope = prev.scope
      // The same note sent twice (a double click) is kept once
      answer.text = prev.text === text ? text : [prev.text, text].filter(Boolean).join('\n')
      if (!acceptance.length && prev.acceptance?.length) answer.acceptance = prev.acceptance
    } catch {}
  }
  const tmp = path.join(answersDir, `.${id}.json.tmp`)
  fs.writeFileSync(tmp, JSON.stringify(answer, null, 2), 'utf-8')
  fs.renameSync(tmp, inboxFile)

  // Resume orchestration unless one is already running (it will pick the answer up)
  let resumed = false
  let resumeError = ''
  if (body.resume !== false) {
    try {
      const res: any = await $fetch('/api/run', { method: 'POST', body: { action: 'start' } })
      resumed = Boolean(res?.ok)
      if (!res?.ok && !String(res?.error || '').includes('already running')) resumeError = res?.error || ''
    } catch (err: any) {
      resumeError = err.message
    }
  }

  return { ok: true, queued: true, resumed, resumeError }
})

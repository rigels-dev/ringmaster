import fs from 'node:fs'
import path from 'node:path'
import { resolveProjectDir } from '../utils/ringmaster'

export default defineEventHandler((event) => {
  const projectDir = resolveProjectDir()
  const query = getQuery(event)
  const selectedTaskId = (query.task as string) || ''

  // 1. Discover all task logs across worktrees and .ringmaster/logs
  const items: Array<{
    taskId: string
    attempt: number
    filePath: string
    size: number
    modifiedAt: string
  }> = []

  // Check worktrees
  const worktreesDir = path.join(projectDir, '.worktrees')
  if (fs.existsSync(worktreesDir)) {
    try {
      const entries = fs.readdirSync(worktreesDir)
      for (const tId of entries) {
        const logFolder = path.join(worktreesDir, tId, '.ringmaster', 'logs')
        if (fs.existsSync(logFolder)) {
          const files = fs.readdirSync(logFolder)
          for (const f of files) {
            if (f.endsWith('.jsonl')) {
              const fullPath = path.join(logFolder, f)
              const stat = fs.statSync(fullPath)
              const attemptMatch = f.match(/attempt(\d+)/)
              const attempt = attemptMatch ? parseInt(attemptMatch[1], 10) : 1
              items.push({
                taskId: tId,
                attempt,
                filePath: fullPath,
                size: stat.size,
                modifiedAt: stat.mtime.toISOString()
              })
            }
          }
        }
      }
    } catch {}
  }

  // Check central logs
  const centralLogs = path.join(projectDir, '.ringmaster', 'logs')
  if (fs.existsSync(centralLogs)) {
    try {
      const files = fs.readdirSync(centralLogs)
      for (const f of files) {
        if (f.endsWith('.jsonl') && f !== 'events.jsonl') {
          const fullPath = path.join(centralLogs, f)
          const stat = fs.statSync(fullPath)
          const taskMatch = f.match(/(T\d+)/)
          const tId = taskMatch ? taskMatch[1] : 'SYSTEM'
          const attemptMatch = f.match(/attempt(\d+)/)
          const attempt = attemptMatch ? parseInt(attemptMatch[1], 10) : 1
          items.push({
            taskId: tId,
            attempt,
            filePath: fullPath,
            size: stat.size,
            modifiedAt: stat.mtime.toISOString()
          })
        }
      }
    } catch {}
  }

  // Read events.jsonl audit trail
  const eventsFile = path.join(projectDir, '.ringmaster', 'logs', 'events.jsonl')
  const events: any[] = []
  if (fs.existsSync(eventsFile)) {
    try {
      const content = fs.readFileSync(eventsFile, 'utf-8')
      const lines = content.split('\n').filter(Boolean)
      for (const line of lines.slice(-100)) {
        try {
          events.push(JSON.parse(line))
        } catch {}
      }
    } catch {}
  }

  // If a specific task is queried, parse its transcript entries
  let activeTranscript: any[] = []
  const targetItem = selectedTaskId
    ? items.find(i => i.taskId === selectedTaskId)
    : (items.length > 0 ? items[items.length - 1] : null)

  if (targetItem && fs.existsSync(targetItem.filePath)) {
    try {
      const raw = fs.readFileSync(targetItem.filePath, 'utf-8')
      const lines = raw.split('\n').filter(Boolean)
      for (const l of lines) {
        try {
          activeTranscript.push(JSON.parse(l))
        } catch {}
      }
    } catch {}
  }

  return {
    ok: true,
    availableTasks: Array.from(new Set(items.map(i => i.taskId))),
    items,
    selectedTask: targetItem ? targetItem.taskId : '',
    transcript: activeTranscript,
    events: events.reverse()
  }
})

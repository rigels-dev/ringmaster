import { spawn } from 'node:child_process'
import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir, childEnv } from '../utils/ringmaster'

interface RunProcessState {
  process: any
  pid: number | null
  isRunning: boolean
  startedAt: string | null
}

// The run started from this server, if any. Its output is not kept here:
// `ringmaster run` persists every line to .ringmaster/logs/run.jsonl, which
// also covers runs started from the CLI and survives server restarts.
const globalState: RunProcessState = {
  process: null,
  pid: null,
  isRunning: false,
  startedAt: null
}

// How much of the run log the Logs page receives (the newest part)
const MAX_LOG_LINES = 3000
const MAX_TAIL_BYTES = 4 * 1024 * 1024

function runLogPath(projectDir: string) {
  return path.join(projectDir, '.ringmaster', 'logs', 'run.jsonl')
}

// Messages from the UI itself go into the same log, so they sort with the run output
function appendRunLog(projectDir: string, line: string) {
  try {
    fs.mkdirSync(path.dirname(runLogPath(projectDir)), { recursive: true })
    fs.appendFileSync(runLogPath(projectDir), JSON.stringify({ ts: new Date().toISOString(), line }) + '\n')
  } catch {}
}

// Newest MAX_LOG_LINES entries of run.jsonl, reading only the file's tail
function readRunLog(projectDir: string): { logs: string[]; logTimes: string[] } {
  const logs: string[] = []
  const logTimes: string[] = []
  let text = ''
  try {
    const file = runLogPath(projectDir)
    const size = fs.statSync(file).size
    const start = Math.max(0, size - MAX_TAIL_BYTES)
    const fd = fs.openSync(file, 'r')
    try {
      const buf = Buffer.alloc(size - start)
      fs.readSync(fd, buf, 0, buf.length, start)
      text = buf.toString('utf-8')
    } finally {
      fs.closeSync(fd)
    }
    // A tail read may start mid-line; drop the partial first line
    if (start > 0) text = text.slice(text.indexOf('\n') + 1)
  } catch {
    return { logs, logTimes }
  }

  const lines = text.split('\n').filter(Boolean).slice(-MAX_LOG_LINES)
  for (const raw of lines) {
    try {
      const entry = JSON.parse(raw)
      logs.push(entry.stream === 'err' ? `[stderr] ${entry.line}` : entry.line)
      logTimes.push(entry.ts || '')
    } catch {}
  }
  return { logs, logTimes }
}

// PID of a `ringmaster run` started outside this server (CLI, or before a
// server restart), read from <project>/.ringmaster/run.pid. Null if not alive.
function externalRunPid(projectDir: string): number | null {
  try {
    const pid = parseInt(fs.readFileSync(path.join(projectDir, '.ringmaster', 'run.pid'), 'utf-8').trim(), 10)
    if (!pid || pid === globalState.pid) return null
    process.kill(pid, 0)
    // Guard against PID reuse after an unclean exit left a stale file
    const cmdline = path.join('/proc', String(pid), 'cmdline')
    if (fs.existsSync(cmdline) && !fs.readFileSync(cmdline, 'utf-8').includes('ringmaster')) return null
    return pid
  } catch {
    return null
  }
}

export default defineEventHandler(async (event) => {
  const projectDir = resolveProjectDir()
  const ringmasterBin = ringmasterBinPath()
  const rootConfig = rootTomlPath()
  const stateFile = path.join(projectDir, '.ringmaster', 'state.json')
  const approvedFile = path.join(projectDir, '.ringmaster', 'plan.approved.json')

  // GET: poll current run status & the persisted run log
  if (event.node.req.method === 'GET') {
    let projectState = null
    if (fs.existsSync(stateFile)) {
      try {
        projectState = JSON.parse(fs.readFileSync(stateFile, 'utf-8'))
      } catch {}
    }

    const extPid = globalState.isRunning ? null : externalRunPid(projectDir)
    const { logs, logTimes } = readRunLog(projectDir)

    return {
      ok: true,
      isRunning: globalState.isRunning || extPid !== null,
      external: extPid !== null,
      pid: extPid ?? globalState.pid,
      startedAt: globalState.startedAt,
      logs,
      logTimes,
      projectState
    }
  }

  // POST: start or stop orchestration
  if (event.node.req.method === 'POST') {
    const body = await readBody(event) || {}
    const action = body.action || 'start'

    // 1. STOP ACTION
    if (action === 'stop') {
      if (globalState.process && globalState.isRunning) {
        try {
          globalState.process.kill('SIGTERM')
        } catch {}
        globalState.isRunning = false
        appendRunLog(projectDir, '[Ringmaster UI] Received user stop request. Orchestration halted gracefully.')
        return { ok: true, message: 'Execution stopping gracefully.' }
      }
      const extPid = externalRunPid(projectDir)
      if (extPid !== null) {
        try {
          process.kill(extPid, 'SIGTERM')
        } catch {}
        appendRunLog(projectDir, `[Ringmaster UI] Sent stop request to external orchestration (PID ${extPid}).`)
        return { ok: true, message: 'Execution stopping gracefully.' }
      }
      return { ok: true, message: 'No active execution running.' }
    }

    // CLEAR LOGS ACTION: archive the log rather than deleting it
    if (action === 'clear_logs') {
      if (globalState.isRunning || externalRunPid(projectDir) !== null) {
        return { ok: false, error: 'Logs can be cleared once the run has exited.' }
      }
      const file = runLogPath(projectDir)
      if (fs.existsSync(file)) {
        const stamp = new Date().toISOString().replace(/[-:]/g, '').replace('T', '-').slice(0, 15)
        fs.renameSync(file, path.join(path.dirname(file), `run-${stamp}.jsonl`))
      }
      return { ok: true }
    }

    // 2. START ACTION
    if (action === 'start') {
      const extPid = externalRunPid(projectDir)
      if (globalState.isRunning || extPid !== null) {
        return { ok: false, error: 'Orchestration is already running.', pid: extPid ?? globalState.pid }
      }

      if (!fs.existsSync(approvedFile)) {
        return { ok: false, error: 'Plan is not approved yet. Please review and approve plan first.' }
      }

      const parallel = body.parallel || 4
      appendRunLog(projectDir, `[Ringmaster UI] Launching orchestration in '${projectDir}' (parallel=${parallel})...`)
      globalState.startedAt = new Date().toISOString()
      globalState.isRunning = true

      try {
        const child = spawn(ringmasterBin, ['run', '--config', rootConfig, '-p', String(parallel)], {
          cwd: projectDir,
          env: childEnv()
        })

        globalState.process = child
        globalState.pid = child.pid || null

        // The run writes its own log file; drain the pipes, keeping only the
        // last stderr line for errors printed after the run log is closed
        let lastErr = ''
        child.stdout.on('data', () => {})
        child.stderr.on('data', (data) => {
          const lines = data.toString().split('\n').map((l: string) => l.trim()).filter(Boolean)
          if (lines.length) lastErr = lines[lines.length - 1]
        })

        child.on('close', (code) => {
          globalState.isRunning = false
          globalState.pid = null
          if (code !== 0 && lastErr) appendRunLog(projectDir, `[Ringmaster UI] ${lastErr}`)
          appendRunLog(projectDir, `[Ringmaster UI] Orchestration process exited with code ${code}.`)
        })

        child.on('error', (err) => {
          globalState.isRunning = false
          globalState.pid = null
          appendRunLog(projectDir, `[Ringmaster UI] Process error: ${err.message}`)
        })

        return {
          ok: true,
          message: 'Orchestration started successfully.',
          pid: globalState.pid,
          startedAt: globalState.startedAt
        }
      } catch (err: any) {
        globalState.isRunning = false
        return { ok: false, error: err.message || 'Failed to start ringmaster run process.' }
      }
    }

    return { ok: false, error: `Unknown action '${action}'` }
  }
})

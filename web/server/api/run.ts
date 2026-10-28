import { spawn } from 'node:child_process'
import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir, childEnv } from '../utils/ringmaster'

interface RunProcessState {
  process: any
  pid: number | null
  isRunning: boolean
  startedAt: string | null
  logs: string[]
}

// In-memory reference for the running background orchestration
const globalState: RunProcessState = {
  process: null,
  pid: null,
  isRunning: false,
  startedAt: null,
  logs: []
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
  const approvedFile = path.join(projectDir, 'plan.approved.json')

  // GET: poll current run status & recent output logs
  if (event.node.req.method === 'GET') {
    let projectState = null
    if (fs.existsSync(stateFile)) {
      try {
        projectState = JSON.parse(fs.readFileSync(stateFile, 'utf-8'))
      } catch {}
    }

    const extPid = globalState.isRunning ? null : externalRunPid(projectDir)

    return {
      ok: true,
      isRunning: globalState.isRunning || extPid !== null,
      external: extPid !== null,
      pid: extPid ?? globalState.pid,
      startedAt: globalState.startedAt,
      logs: globalState.logs,
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
        globalState.logs.push('[Ringmaster UI] Received user stop request. Orchestration halted gracefully.')
        return { ok: true, message: 'Execution stopping gracefully.' }
      }
      const extPid = externalRunPid(projectDir)
      if (extPid !== null) {
        try {
          process.kill(extPid, 'SIGTERM')
        } catch {}
        globalState.logs.push(`[Ringmaster UI] Sent stop request to external orchestration (PID ${extPid}).`)
        return { ok: true, message: 'Execution stopping gracefully.' }
      }
      return { ok: true, message: 'No active execution running.' }
    }

    // CLEAR LOGS ACTION
    if (action === 'clear_logs') {
      if (!globalState.isRunning) {
        globalState.logs = []
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

      // Check if project state is blocked, or fresh start
      const parallel = body.parallel || 4
      globalState.logs = [`[Ringmaster UI] Launching orchestration in '${projectDir}' (parallel=${parallel})...`]
      globalState.startedAt = new Date().toISOString()
      globalState.isRunning = true

      try {
        const child = spawn(ringmasterBin, ['run', '--config', rootConfig, '-p', String(parallel)], {
          cwd: projectDir,
          env: childEnv()
        })

        globalState.process = child
        globalState.pid = child.pid || null

        child.stdout.on('data', (data) => {
          const lines = data.toString().split('\n').filter(Boolean)
          for (const line of lines) {
            globalState.logs.push(line)
            if (globalState.logs.length > 200) globalState.logs.shift()
          }
        })

        child.stderr.on('data', (data) => {
          const lines = data.toString().split('\n').filter(Boolean)
          for (const line of lines) {
            globalState.logs.push(`[stderr] ${line}`)
            if (globalState.logs.length > 200) globalState.logs.shift()
          }
        })

        child.on('close', (code) => {
          globalState.isRunning = false
          globalState.pid = null
          globalState.logs.push(`[Ringmaster UI] Orchestration process exited with code ${code}.`)
        })

        child.on('error', (err) => {
          globalState.isRunning = false
          globalState.pid = null
          globalState.logs.push(`[Ringmaster UI] Process error: ${err.message}`)
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

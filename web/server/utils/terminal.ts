import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { StringDecoder } from 'node:string_decoder'
import path from 'node:path'
import { ringmasterBinPath, childEnv } from './ringmaster'

// Web UI terminal sessions. Each session is a `ringmaster pty-bridge` child
// (a shell on a pseudo-terminal); sessions outlive the browser socket so a
// minimized panel or a page refresh reattaches and replays recent output.

const REPLAY_LIMIT = 256 * 1024

export interface TerminalPeer {
  send(data: string): void
}

interface TerminalSession {
  id: string
  cwd: string
  title: string
  proc: ChildProcessWithoutNullStreams
  replay: string
  peers: Set<TerminalPeer>
  exited: boolean
  exitCode: number | null
}

const sessions = new Map<string, TerminalSession>()
let counter = 0

function broadcast(s: TerminalSession, msg: object) {
  const data = JSON.stringify(msg)
  for (const peer of s.peers) {
    try {
      peer.send(data)
    } catch {}
  }
}

export function createSession(cwd: string, cols: number, rows: number): TerminalSession {
  counter++
  const id = `t${Date.now().toString(36)}${counter}`
  const proc = spawn(
    ringmasterBinPath(),
    ['pty-bridge', '--cwd', cwd, '--cols', String(cols || 80), '--rows', String(rows || 24)],
    { cwd, env: childEnv() }
  )
  const session: TerminalSession = {
    id,
    cwd,
    title: path.basename(cwd) || cwd,
    proc,
    replay: '',
    peers: new Set(),
    exited: false,
    exitCode: null
  }

  // Decode as a stream so multi-byte characters split across chunks survive
  const decoder = new StringDecoder('utf8')
  const onOutput = (chunk: Buffer) => {
    const text = decoder.write(chunk)
    if (!text) return
    session.replay = (session.replay + text).slice(-REPLAY_LIMIT)
    broadcast(session, { t: 'o', d: text })
  }
  proc.stdout.on('data', onOutput)
  proc.stderr.on('data', onOutput)

  const finish = (code: number | null) => {
    if (session.exited) return
    session.exited = true
    session.exitCode = code
    broadcast(session, { t: 'x', code })
  }
  proc.on('close', finish)
  proc.on('error', (err) => {
    onOutput(Buffer.from(`\r\n[terminal] failed to start shell: ${err.message}\r\n`))
    finish(-1)
  })

  sessions.set(id, session)
  return session
}

export function getTerminalSession(id: string) {
  return sessions.get(id)
}

export function listSessions() {
  return [...sessions.values()].map((s) => ({ id: s.id, title: s.title, cwd: s.cwd, exited: s.exited }))
}

export function writeInput(s: TerminalSession, data: string) {
  if (!s.exited) s.proc.stdin.write(JSON.stringify({ t: 'i', d: data }) + '\n')
}

export function resize(s: TerminalSession, cols: number, rows: number) {
  if (!s.exited && cols > 0 && rows > 0) s.proc.stdin.write(JSON.stringify({ t: 'r', c: cols, r: rows }) + '\n')
}

export function killSession(id: string) {
  const s = sessions.get(id)
  if (!s) return
  sessions.delete(id)
  // Closing stdin makes the bridge kill its shell; SIGTERM covers a stuck bridge
  try {
    s.proc.stdin.end()
  } catch {}
  setTimeout(() => {
    if (!s.exited) s.proc.kill('SIGTERM')
  }, 1000)
}

// Sessions may only start inside the project (its root or a task worktree)
export function resolveTerminalCwd(projectDir: string, requested?: string): string {
  const root = path.resolve(projectDir)
  if (!requested) return root
  const target = path.resolve(root, requested)
  return target === root || target.startsWith(root + path.sep) ? target : root
}

import fs from 'node:fs'
import { spawn } from 'node:child_process'
import path from 'node:path'
import os from 'node:os'

// Ringmaster installation root. Set by the Go launcher via RINGMASTER_ROOT;
// otherwise walk up from the working directory until cmd/ringmaster is found.
export function ringmasterRoot(): string {
  if (process.env.RINGMASTER_ROOT) return process.env.RINGMASTER_ROOT

  let dir = process.cwd()
  while (true) {
    if (fs.existsSync(path.join(dir, 'cmd', 'ringmaster'))) return dir
    const parent = path.dirname(dir)
    if (parent === dir) return process.cwd()
    dir = parent
  }
}

export function ringmasterBinPath(): string {
  return process.env.RINGMASTER_BIN || path.join(ringmasterRoot(), 'bin', 'ringmaster')
}

// Global fleet config (providers & agents). The launcher passes the path it
// resolved; otherwise follow the same rule as the CLI: the user config dir,
// else a legacy config next to the install.
export function rootTomlPath(): string {
  if (process.env.RINGMASTER_CONFIG) return process.env.RINGMASTER_CONFIG
  const configDir = process.env.RINGMASTER_CONFIG_DIR
    || (process.env.XDG_CONFIG_HOME ? path.join(process.env.XDG_CONFIG_HOME, 'ringmaster') : path.join(os.homedir(), '.config', 'ringmaster'))
  const user = path.join(configDir, 'ringmaster.toml')
  const legacy = path.join(ringmasterRoot(), 'ringmaster.toml')
  if (!fs.existsSync(user) && fs.existsSync(legacy)) return legacy
  return user
}

// Project directory: set by the Go launcher, which only starts inside a
// project. Falls back to the process CWD when the server is run by hand.
export function resolveProjectDir(): string {
  return process.env.RINGMASTER_PROJECT_DIR || process.cwd()
}

// Run a command without a shell (args are passed verbatim), optionally piping
// `input` to stdin. Rejects with stdout/stderr attached on non-zero exit or timeout.
export function runCommand(
  cmd: string,
  args: string[],
  opts: { cwd?: string; input?: string; timeout?: number } = {}
): Promise<{ stdout: string; stderr: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { cwd: opts.cwd, env: childEnv() })
    let stdout = ''
    let stderr = ''
    let timedOut = false

    const timer = opts.timeout
      ? setTimeout(() => { timedOut = true; child.kill('SIGKILL') }, opts.timeout)
      : null

    child.stdout.on('data', (d) => { stdout += d.toString() })
    child.stderr.on('data', (d) => { stderr += d.toString() })
    child.on('error', (err: any) => {
      if (timer) clearTimeout(timer)
      reject(Object.assign(err, { stdout, stderr }))
    })
    child.on('close', (code) => {
      if (timer) clearTimeout(timer)
      if (code === 0 && !timedOut) return resolve({ stdout, stderr })
      const reason = timedOut ? `timed out after ${opts.timeout}ms` : `exited with code ${code}`
      reject(Object.assign(new Error(`${path.basename(cmd)} ${reason}: ${stderr.trim()}`), { stdout, stderr }))
    })

    child.stdin.end(opts.input ?? '')
  })
}

// Environment for child processes: make sure node (for the claude CLI),
// ~/.local/bin and the Go toolchain are reachable.
export function childEnv(): NodeJS.ProcessEnv {
  const extra = [
    path.dirname(process.execPath),
    path.join(os.homedir(), '.local', 'bin'),
    '/usr/local/go/bin'
  ]
  return {
    ...process.env,
    PATH: [process.env.PATH, ...extra].filter(Boolean).join(path.delimiter)
  }
}

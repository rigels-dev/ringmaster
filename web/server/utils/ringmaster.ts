import fs from 'node:fs'
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

// Global fleet config (providers & agents)
export function rootTomlPath(): string {
  return path.join(ringmasterRoot(), 'ringmaster.toml')
}

// Resolve project directory in priority:
// a. Environment variable RINGMASTER_PROJECT_DIR
// b. Global active project file (~/.ringmaster/active_project)
// c. Process CWD
export function resolveProjectDir(): string {
  let projectDir = process.env.RINGMASTER_PROJECT_DIR

  if (!projectDir || projectDir.includes('.openclaw')) {
    const activeFile = path.join(os.homedir(), '.ringmaster', 'active_project')
    if (fs.existsSync(activeFile)) {
      try {
        const stored = fs.readFileSync(activeFile, 'utf-8').trim()
        if (stored && fs.existsSync(stored)) {
          projectDir = stored
        }
      } catch {}
    }
  }

  return projectDir || process.cwd()
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

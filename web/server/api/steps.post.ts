import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// { action: 'name', step, name }   name a step ('' for "Step <n>")
// { action: 'close', step }        finished without a release
// { action: 'reopen', step }       offer a release again
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  const step = Number(body.step)
  if (!Number.isInteger(step) || step < 1) return { ok: false, error: 'Unknown step' }

  let args: string[]
  if (body.action === 'name') {
    const name = typeof body.name === 'string' ? body.name : ''
    args = ['steps', 'name', '--config', rootTomlPath(), '--', String(step), ...(name.trim() ? [name] : [])]
  } else if (body.action === 'close' || body.action === 'reopen') {
    args = ['steps', body.action, '--config', rootTomlPath(), '--', String(step)]
  } else {
    return { ok: false, error: "Unknown action; use 'name', 'close' or 'reopen'" }
  }

  try {
    await runCommand(ringmasterBinPath(), args, { cwd: resolveProjectDir(), timeout: 15000 })
    return { ok: true }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to update the step') }
  }
})

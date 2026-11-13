import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { isSkillName } from '../utils/skills'
import { cliError } from '../utils/cli'

// Turn a skill on or off in the fleet config (ringmaster skills enable|disable).
export default defineEventHandler(async (event) => {
  const body = (await readBody(event)) || {}
  if (!isSkillName(body.name)) {
    return { ok: false, error: 'Invalid skill name' }
  }
  if (typeof body.enabled !== 'boolean') {
    return { ok: false, error: "'enabled' must be true or false" }
  }

  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['skills', body.enabled ? 'enable' : 'disable', body.name, '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    return { ok: true, message: stdout.trim().replace(/^✓\s*/, '') }
  } catch (err: any) {
    return { ok: false, error: cliError(err, 'Failed to change the skill') }
  }
})

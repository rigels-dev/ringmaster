import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'

// The installed agent skills and whether workers get them, as the Go binary
// sees them (ringmaster skills --json).
export default defineEventHandler(async () => {
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['skills', '--json', '--config', rootTomlPath()],
      { cwd: resolveProjectDir(), timeout: 15000 }
    )
    const report = JSON.parse(stdout)
    return {
      ok: true,
      skills: Array.isArray(report.skills) ? report.skills : [],
      globalDir: report.global_dir || '',
      projectDir: report.project_dir || ''
    }
  } catch (err: any) {
    return { ok: false, error: (err.stderr || err.message || 'Failed to list skills').trim(), skills: [] }
  }
})

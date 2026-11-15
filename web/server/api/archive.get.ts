import fs from 'node:fs'
import path from 'node:path'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { cliError } from '../utils/cli'

// Download the project's committed files as an archive.
// ?ref=<tag or branch, default main>&format=zip|tar.gz
export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const ref = typeof query.ref === 'string' && query.ref ? query.ref : 'main'
  const format = query.format === 'tar.gz' ? 'tar.gz' : 'zip'

  let file = ''
  try {
    const { stdout } = await runCommand(
      ringmasterBinPath(),
      ['archive', '--json', '--format', format, '--config', rootTomlPath(), '--', ref],
      { cwd: resolveProjectDir(), timeout: 120000 }
    )
    file = JSON.parse(stdout).path
  } catch (err: any) {
    throw createError({ statusCode: 400, statusMessage: 'Bad Request', message: cliError(err, 'Failed to build the archive') })
  }

  setHeader(event, 'Content-Type', format === 'zip' ? 'application/zip' : 'application/gzip')
  setHeader(event, 'Content-Disposition', `attachment; filename="${path.basename(file).replace(/"/g, '')}"`)
  setHeader(event, 'Content-Length', fs.statSync(file).size)
  return sendStream(event, fs.createReadStream(file))
})

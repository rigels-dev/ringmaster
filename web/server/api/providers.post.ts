import fs from 'node:fs'
import path from 'node:path'

export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const projectDir = process.env.RINGMASTER_PROJECT_DIR || process.cwd()
  const localToml = path.join(projectDir, 'ringmaster.toml')
  const rootToml = '/home/ubuntu/project/ringmaster/ringmaster.toml'
  const targetToml = fs.existsSync(localToml) ? localToml : rootToml

  if (!fs.existsSync(targetToml)) {
    return { ok: false, error: 'Config file not found' }
  }

  try {
    let content = fs.readFileSync(targetToml, 'utf-8')
    const { action } = body

    if (action === 'add_provider') {
      const { id, type, base_url, api_key_env } = body
      if (!id) return { ok: false, error: 'Provider ID is required' }

      // Check if provider block already exists
      const blockRegex = new RegExp(`\\[providers\\.${id}\\][\\s\\S]*?(?=\\n\\[|$)`)
      const newBlock = `[providers.${id}]\ntype = '${type || 'openai'}'\nbase_url = '${base_url || ''}'\napi_key_env = '${api_key_env || ''}'\n`

      if (blockRegex.test(content)) {
        content = content.replace(blockRegex, newBlock.trim())
      } else {
        // Append before [agents] or at end of providers
        if (content.includes('[agents]')) {
          content = content.replace('[agents]', `${newBlock}\n[agents]`)
        } else {
          content += `\n${newBlock}`
        }
      }
      fs.writeFileSync(targetToml, content, 'utf-8')
      return { ok: true, message: `Provider '${id}' saved successfully` }
    }

    if (action === 'delete_provider') {
      const { id } = body
      const blockRegex = new RegExp(`\\[providers\\.${id}\\][\\s\\S]*?(?=\\n\\[|$)`, 'g')
      content = content.replace(blockRegex, '')
      fs.writeFileSync(targetToml, content, 'utf-8')
      return { ok: true, message: `Provider '${id}' deleted` }
    }

    if (action === 'add_agent') {
      const { id, provider, model, role, tool_calling } = body
      if (!id || !model) return { ok: false, error: 'Agent alias and model name are required' }

      const tc = tool_calling ? 'true' : 'false'
      const newBlock = `[agents.${id}]\nprovider = '${provider || ''}'\nmodel = '${model}'\nrole = '${role || 'worker'}'\ntool_calling = ${tc}\n`

      const blockRegex = new RegExp(`\\[agents\\.${id}\\][\\s\\S]*?(?=\\n\\[|$)`)
      if (blockRegex.test(content)) {
        content = content.replace(blockRegex, newBlock.trim())
      } else {
        if (content.includes('[routing]')) {
          content = content.replace('[routing]', `${newBlock}\n[routing]`)
        } else {
          content += `\n${newBlock}`
        }
      }

      if (role === 'master') {
        content = content.replace(/default_master\s*=\s*['"][^'"]*['"]/, `default_master = '${id}'`)
      }

      fs.writeFileSync(targetToml, content, 'utf-8')
      return { ok: true, message: `Agent '${id}' configured successfully` }
    }

    if (action === 'set_role') {
      const { id, role } = body
      const blockRegex = new RegExp(`(\\[agents\\.${id}\\][\\s\\S]*?)(role\\s*=\\s*['"][^'"]*['"])([\\s\\S]*?)(?=\\n\\[|$)`)
      if (blockRegex.test(content)) {
        content = content.replace(blockRegex, (match, p1, p2, p3) => {
          const newRole = `role = '${role}'`
          const newTc = role === 'master' ? 'tool_calling = false' : 'tool_calling = true'
          let updated = p1 + newRole + p3
          updated = updated.replace(/tool_calling\s*=\s*(true|false)/, newTc)
          return updated
        })
      }

      if (role === 'master') {
        content = content.replace(/default_master\s*=\s*['"][^'"]*['"]/, `default_master = '${id}'`)
      } else if (role === 'worker') {
        content = content.replace(/default_worker\s*=\s*['"][^'"]*['"]/, `default_worker = '${id}'`)
      }

      fs.writeFileSync(targetToml, content, 'utf-8')
      return { ok: true, message: `Agent '${id}' role updated to ${role}` }
    }

    if (action === 'delete_agent') {
      const { id } = body
      const blockRegex = new RegExp(`\\[agents\\.${id}\\][\\s\\S]*?(?=\\n\\[|$)`, 'g')
      content = content.replace(blockRegex, '')
      fs.writeFileSync(targetToml, content, 'utf-8')
      return { ok: true, message: `Agent '${id}' deleted` }
    }

    return { ok: false, error: 'Invalid action' }
  } catch (err: any) {
    return { ok: false, error: err.message }
  }
})

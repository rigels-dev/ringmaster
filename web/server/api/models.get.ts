import fs from 'node:fs'
import { rootTomlPath } from '../utils/ringmaster'

function escapeRegex(str: string) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const providerId = query.provider as string

  if (!providerId) {
    return { ok: false, error: 'provider query param is required' }
  }

  // Providers live in the global fleet config; a project's ringmaster.toml is
  // only its manifest (name, brief, plan) and never defines providers
  const targetToml = rootTomlPath()

  if (!fs.existsSync(targetToml)) {
    return { ok: false, error: 'Config file not found' }
  }

  let content = ''
  try {
    content = fs.readFileSync(targetToml, 'utf-8')
  } catch (err: any) {
    return { ok: false, error: err.message }
  }

  // Extract provider block
  const blockRegex = new RegExp(`\\[providers\\.${escapeRegex(providerId)}\\]([\\s\\S]*?)(?=\\n\\[|$)`)
  const match = content.match(blockRegex)
  if (!match) {
    return { ok: false, error: `Provider '${providerId}' not found in config` }
  }

  const block = match[1]
  const typeMatch = block.match(/type\s*=\s*['"]([^'"]+)['"]/)
  const urlMatch = block.match(/base_url\s*=\s*['"]([^'"]+)['"]/)
  const envMatch = block.match(/api_key_env\s*=\s*['"]([^'"]+)['"]/)

  const pType = typeMatch ? typeMatch[1] : 'openai'
  const baseURL = urlMatch ? urlMatch[1] : ''
  const apiEnv = envMatch ? envMatch[1] : ''

  // 1. Claude Subscription (Local Session) or Anthropic
  if (pType === 'claude_subscription' || pType === 'claude_sub' || pType === 'anthropic') {
    return {
      ok: true,
      provider: providerId,
      type: pType,
      models: [
        { id: 'claude-opus-5-5', name: 'Claude Opus 5.5 (deep architecture & planning)' },
        { id: 'claude-sonnet-5-5', name: 'Claude Sonnet 5.5 (balanced)' },
        { id: 'claude-haiku-5-5', name: 'Claude Haiku 5.5 (fast worker)' },
        { id: 'claude-fable-5-1', name: 'Claude Fable 5.1' }
      ]
    }
  }

  // 3. OpenAI / 9router / Ollama / OpenRouter Live Fetch
  let apiKey = ''
  if (apiEnv) {
    if (apiEnv.startsWith('sk-') || apiEnv.length > 20) {
      apiKey = apiEnv
    } else {
      apiKey = process.env[apiEnv] || apiEnv
    }
  }

  let endpoint = baseURL || 'https://api.openai.com/v1'
  const targetUrl = endpoint.replace(/\/+$/, '') + '/models'

  try {
    const headers: Record<string, string> = {
      'User-Agent': 'Ringmaster/0.4'
    }
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`
    }

    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 8000)

    const resp = await fetch(targetUrl, {
      headers,
      signal: controller.signal
    })
    clearTimeout(timeout)

    if (!resp.ok) {
      return { ok: false, error: `Failed to fetch models from ${targetUrl} (HTTP ${resp.status})` }
    }

    const json = await resp.json()
    const rawList = json.data || json.models || []
    const models = rawList.map((m: any) => ({
      id: typeof m === 'string' ? m : m.id,
      name: typeof m === 'string' ? m : (m.id || m.name)
    }))

    return {
      ok: true,
      provider: providerId,
      type: pType,
      models
    }
  } catch (err: any) {
    return {
      ok: false,
      error: err.name === 'AbortError' ? 'Timeout fetching models list' : err.message
    }
  }
})

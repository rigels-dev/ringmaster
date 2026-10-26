export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const { type, base_url, api_key_env } = body
  const start = Date.now()

  if (type === 'claude_subscription' || type === 'claude_sub') {
    const fs = await import('node:fs')
    const os = await import('node:os')
    const path = await import('node:path')
    const claudeDir = path.join(os.homedir(), '.claude')
    if (fs.existsSync(claudeDir)) {
      return {
        ok: true,
        latency_ms: Date.now() - start,
        message: 'Claude CLI session credentials detected (~/.claude/)'
      }
    }
    return {
      ok: false,
      latency_ms: Date.now() - start,
      error: 'No ~/.claude credentials found. Run "claude auth login" first.'
    }
  }

  let apiKey = ''
  if (api_key_env) {
    if (api_key_env.startsWith('sk-') || api_key_env.length > 20) {
      apiKey = api_key_env
    } else {
      apiKey = process.env[api_key_env] || api_key_env
    }
  }

  let endpoint = base_url
  if (!endpoint) {
    endpoint = type === 'anthropic' ? 'https://api.anthropic.com/v1' : 'https://api.openai.com/v1'
  }

  // If model is provided, test a real chat completion (PONG ping test)
  if (body.model) {
    const model = body.model
    if (type === 'claude_subscription' || type === 'claude_sub') {
      return {
        ok: true,
        latency_ms: Date.now() - start,
        message: `Agent reachable via local Claude CLI (${model})`
      }
    }

    try {
      let chatUrl = endpoint.replace(/\/+$/, '')
      let payload: any = {}
      let headers: Record<string, string> = { 'Content-Type': 'application/json', 'User-Agent': 'Ringmaster/0.4' }

      if (type === 'anthropic') {
        chatUrl += '/messages'
        if (apiKey) {
          headers['x-api-key'] = apiKey
          headers['anthropic-version'] = '2023-06-01'
        }
        payload = {
          model,
          max_tokens: 10,
          messages: [{ role: 'user', content: 'Respond with PONG' }]
        }
      } else {
        chatUrl += '/chat/completions'
        if (apiKey) {
          headers['Authorization'] = `Bearer ${apiKey}`
        }
        payload = {
          model,
          max_tokens: 10,
          messages: [{ role: 'user', content: 'Respond with PONG' }]
        }
      }

      const controller = new AbortController()
      const timeout = setTimeout(() => controller.abort(), 15000)

      const resp = await fetch(chatUrl, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
        signal: controller.signal
      })
      clearTimeout(timeout)

      const latency = Date.now() - start
      if (resp.ok) {
        const json = await resp.json().catch(() => ({}))
        const tokens = json.usage?.total_tokens || json.usage?.output_tokens || 1
        return {
          ok: true,
          latency_ms: latency,
          message: `Model '${model}' responded successfully! (Tokens: ${tokens})`
        }
      } else {
        const text = await resp.text().catch(() => '')
        return {
          ok: false,
          latency_ms: latency,
          error: `HTTP ${resp.status}: ${text || resp.statusText}`
        }
      }
    } catch (err: any) {
      return {
        ok: false,
        latency_ms: Date.now() - start,
        error: err.name === 'AbortError' ? 'Chat timeout (15s)' : err.message
      }
    }
  }

  try {
    const targetUrl = endpoint.replace(/\/+$/, '') + '/models'
    const headers: Record<string, string> = {
      'User-Agent': 'Ringmaster/0.4'
    }
    if (apiKey) {
      if (type === 'anthropic') {
        headers['x-api-key'] = apiKey
        headers['anthropic-version'] = '2023-06-01'
      } else {
        headers['Authorization'] = `Bearer ${apiKey}`
      }
    }

    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 8000)

    const resp = await fetch(targetUrl, {
      headers,
      signal: controller.signal
    })
    clearTimeout(timeout)

    const latency = Date.now() - start
    if (resp.ok) {
      const data = await resp.json().catch(() => ({}))
      const count = (data.data || data.models || []).length
      return {
        ok: true,
        latency_ms: latency,
        message: count > 0 ? `Connected (${count} models reachable)` : 'Connected to endpoint successfully'
      }
    } else {
      return {
        ok: false,
        latency_ms: latency,
        error: `HTTP ${resp.status}: ${resp.statusText}`
      }
    }
  } catch (err: any) {
    return {
      ok: false,
      latency_ms: Date.now() - start,
      error: err.name === 'AbortError' ? 'Connection timed out (8s)' : err.message
    }
  }
})

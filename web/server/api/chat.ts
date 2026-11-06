import path from 'node:path'
import fs from 'node:fs'
import { rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'

export default defineEventHandler(async (event) => {
  const projectDir = resolveProjectDir()
  const dotRingmaster = path.join(projectDir, '.ringmaster')
  const chatFile = path.join(dotRingmaster, 'chat_history.json')
  const briefPath = path.join(projectDir, '.ringmaster', 'BRIEF.md')
  // Master agent and its provider come from the global fleet config only
  const rootToml = rootTomlPath()

  if (event.node.req.method === 'GET') {
    if (fs.existsSync(chatFile)) {
      try {
        const data = JSON.parse(fs.readFileSync(chatFile, 'utf-8'))
        return { ok: true, messages: data }
      } catch (err: any) {
        return { ok: false, error: err.message, messages: [] }
      }
    }
    return { ok: true, messages: [] }
  }

  if (event.node.req.method === 'POST') {
    try {
      const body = await readBody(event)
      if (!fs.existsSync(dotRingmaster)) {
        fs.mkdirSync(dotRingmaster, { recursive: true })
      }

      // If action is asking Master AI in interactive discussion
      if (body.action === 'discuss') {
        const userPrompt = body.prompt || ''
        const history = Array.isArray(body.history) ? body.history : []

        // Resolve active Master Agent and its Provider configuration dynamically
        let masterAlias = ''
        let masterModel = ''
        let masterProvider = ''

        // Helper to inspect TOML config
        const inspectConfig = (tomlFile: string) => {
          if (!fs.existsSync(tomlFile)) return
          try {
            const content = fs.readFileSync(tomlFile, 'utf-8')
            const mMatch = content.match(/default_master\s*=\s*['"]([^'"]+)['"]/)
            if (mMatch) {
              masterAlias = mMatch[1].trim().replace(/^['"]|['"]$/g, '')
            }
            if (masterAlias) {
              const aBlockRegex = new RegExp(`\\[agents\\.(?:'|")?${masterAlias.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}(?:'|")?\\]([\\s\\S]*?)(?=\\n\\[|$)`)
              const aBlock = content.match(aBlockRegex)
              if (aBlock) {
                const modelMatch = aBlock[1].match(/model\s*=\s*['"]([^'"]+)['"]/)
                const provMatch = aBlock[1].match(/provider\s*=\s*['"]([^'"]+)['"]/)
                if (modelMatch) masterModel = modelMatch[1].trim().replace(/^['"]|['"]$/g, '')
                if (provMatch) masterProvider = provMatch[1].trim().replace(/^['"]|['"]$/g, '')
              }
            }
          } catch {}
        }

        inspectConfig(rootToml)

        // Fallbacks if not set
        if (!masterModel) {
          masterModel = 'claude-opus-5-5'
          masterProvider = 'claude-sub'
        }

        // Read provider specs
        let provType = 'openai'
        let provBaseURL = ''
        let provAPIKey = ''

        const inspectProvider = (tomlFile: string, pName: string) => {
          if (!fs.existsSync(tomlFile) || !pName) return
          try {
            const content = fs.readFileSync(tomlFile, 'utf-8')
            const pBlockRegex = new RegExp(`\\[providers\\.(?:'|")?${pName.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}(?:'|")?\\]([\\s\\S]*?)(?=\\n\\[|$)`)
            const pBlock = content.match(pBlockRegex)
            if (pBlock) {
              const tMatch = pBlock[1].match(/type\s*=\s*['"]([^'"]+)['"]/)
              const bMatch = pBlock[1].match(/base_url\s*=\s*['"]([^'"]+)['"]/)
              const kMatch = pBlock[1].match(/api_key_env\s*=\s*['"]([^'"]+)['"]/)
              if (tMatch) provType = tMatch[1].trim().replace(/^['"]|['"]$/g, '')
              if (bMatch) provBaseURL = bMatch[1].trim().replace(/^['"]|['"]$/g, '')
              if (kMatch) provAPIKey = kMatch[1].trim().replace(/^['"]|['"]$/g, '')
            }
          } catch {}
        }

        inspectProvider(rootToml, masterProvider)

        // Resolve API key environment variable or literal
        let apiKey = ''
        if (provAPIKey) {
          if (provAPIKey.startsWith('sk-') || provAPIKey.length > 20) {
            apiKey = provAPIKey
          } else {
            apiKey = process.env[provAPIKey] || provAPIKey
          }
        }

        const systemPrompt = `You are the Master AI Architect and Project Planner of Ringmaster (Control Plane).
You are currently engaged in a live planning discussion with the project lead/developer.
Your goals:
1. Discuss requirements, clarify architecture, verify tech stack, and structure the implementation.
2. Outline recommended task breakdown, testing strategy, and automaton assignment.
3. Keep answers concise, authoritative, structured, and helpful.
   TERMINOLOGY: the execution agents that implement tasks are called "automatons" (e.g. "Automaton A", "dikerjakan oleh 2 automaton"). Never call them "workers" in your replies, in any language.
4. READINESS DECISION (CRITICAL):
   - If the project scope, requirements, or architecture are still vague, ask clarifying questions. DO NOT append the readiness signal.
   - ONLY when you have evaluated that you have sufficient, actionable information (clear tech stack, clear endpoints/logic, and a viable task breakdown), you MUST append this exact signal at the very end of your response:
   [RINGMASTER_CAN_GENERATE: TRUE]
Format with clean markdown.`

        // Prepare conversation context
        const convMessages: Array<{ role: string; content: string }> = [
          { role: 'system', content: systemPrompt }
        ]

        for (const m of history.slice(-8)) {
          if (m.text && !m.text.includes('⚡ Generate DAG Plan')) {
            convMessages.push({
              role: m.role === 'user' ? 'user' : 'assistant',
              content: m.text
            })
          }
        }
        convMessages.push({ role: 'user', content: userPrompt })

        let replyText = ''

        // 1. If provider is Claude Subscription CLI
        if (provType === 'claude_subscription' || provType === 'claude_sub' || provType === 'claude-sub') {
          let conversationContext = ''
          for (const m of convMessages.slice(1)) {
            conversationContext += `${m.role === 'user' ? 'User' : 'Ringmaster Master'}: ${m.content}\n\n`
          }
          conversationContext += `Ringmaster Master:`

          const fullPrompt = `${systemPrompt}\n\n${conversationContext}`
          const args = ['-p']
          if (masterModel) args.push('--model', masterModel)

          // Prompt goes over stdin, so no quoting or temp file is needed
          const { stdout } = await runCommand('claude', args, { input: fullPrompt, timeout: 90000 })
          replyText = stdout.trim()
        } 
        // 2. If provider is Anthropic direct API
        else if (provType === 'anthropic') {
          const ep = (provBaseURL || 'https://api.anthropic.com/v1').replace(/\/+$/, '') + '/messages'
          const headers: Record<string, string> = {
            'Content-Type': 'application/json',
            'anthropic-version': '2023-06-01'
          }
          if (apiKey) headers['x-api-key'] = apiKey

          const formattedMessages = convMessages.filter(m => m.role !== 'system').map(m => ({
            role: m.role,
            content: m.content
          }))

          const res = await $fetch<any>(ep, {
            method: 'POST',
            headers,
            body: {
              model: masterModel,
              max_tokens: 2048,
              system: systemPrompt,
              messages: formattedMessages
            }
          })
          replyText = res?.content?.[0]?.text || ''
        }
        // 3. If provider is OpenAI-compatible (9router, sumopod, ollama, openai, vllm, deepseek)
        else {
          const ep = (provBaseURL || 'https://api.openai.com/v1').replace(/\/+$/, '') + '/chat/completions'
          const headers: Record<string, string> = {
            'Content-Type': 'application/json'
          }
          if (apiKey) headers['Authorization'] = `Bearer ${apiKey}`

          const res = await $fetch<any>(ep, {
            method: 'POST',
            headers,
            body: {
              model: masterModel,
              messages: convMessages
            }
          })
          replyText = res?.choices?.[0]?.message?.content || ''
        }

        // Detect [RINGMASTER_CAN_GENERATE: TRUE] tag
        let canGenerate = false
        if (replyText.includes('[RINGMASTER_CAN_GENERATE: TRUE]')) {
          canGenerate = true
          replyText = replyText.replace(/\[RINGMASTER_CAN_GENERATE:\s*TRUE\]/g, '').trim()
        }

        return {
          ok: true,
          reply: replyText,
          canGenerate,
          masterUsed: `${masterAlias || 'default'} (${masterModel})`,
          updatedBrief: fs.existsSync(briefPath) ? fs.readFileSync(briefPath, 'utf-8') : ''
        }
      }

      // Save messages array
      if (Array.isArray(body.messages)) {
        fs.writeFileSync(chatFile, JSON.stringify(body.messages, null, 2), 'utf-8')
      }

      // If user message is updating project brief or instruction
      if (body.updateBrief && typeof body.briefContent === 'string') {
        fs.writeFileSync(briefPath, body.briefContent, 'utf-8')
      }

      return { ok: true, saved: true }
    } catch (err: any) {
      return { ok: false, error: err.message }
    }
  }
})

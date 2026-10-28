import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'

export default defineEventHandler(async (event) => {
  const projectDir = resolveProjectDir()
  const ringmasterBin = ringmasterBinPath()
  const briefPath = path.join(projectDir, 'BRIEF.md')
  const planFile = path.join(projectDir, 'plan.json')
  const approvedFile = path.join(projectDir, 'plan.approved.json')
  const dotRingmaster = path.join(projectDir, '.ringmaster')
  const chatFile = path.join(dotRingmaster, 'chat_history.json')

  // GET: return plan.json and approval state
  if (event.node.req.method === 'GET') {
    let plan = null
    let isApproved = false
    let approvedHash = ''

    if (fs.existsSync(planFile)) {
      try {
        plan = JSON.parse(fs.readFileSync(planFile, 'utf-8'))
      } catch {}
    }

    if (fs.existsSync(approvedFile)) {
      try {
        const appData = JSON.parse(fs.readFileSync(approvedFile, 'utf-8'))
        isApproved = true
        approvedHash = appData.plan_hash || ''
      } catch {}
    }

    return {
      ok: true,
      plan,
      isApproved,
      approvedHash
    }
  }

  // POST: actions (generate, approve, update_worker, reject)
  if (event.node.req.method === 'POST') {
    try {
      const body = await readBody(event) || {}
      const action = body.action || 'generate'

      // 1. ACTION: APPROVE PLAN
      if (action === 'approve') {
        const rootConfig = rootTomlPath()
        const { stdout } = await runCommand(ringmasterBin, ['plan', '--approve', '--config', rootConfig], {
          cwd: projectDir,
          timeout: 30000
        })

        let approvedHash = ''
        if (fs.existsSync(approvedFile)) {
          try {
            const appData = JSON.parse(fs.readFileSync(approvedFile, 'utf-8'))
            approvedHash = appData.plan_hash || ''
          } catch {}
        }

        return {
          ok: true,
          message: 'Plan successfully approved and cryptographically hashed!',
          isApproved: true,
          approvedHash,
          output: stdout
        }
      }

      // 2. ACTION: UPDATE TASK WORKER
      if (action === 'update_worker') {
        const { taskId, worker } = body
        if (!taskId || !worker) {
          return { ok: false, error: 'taskId and worker are required' }
        }

        if (!fs.existsSync(planFile)) {
          return { ok: false, error: 'plan.json not found' }
        }

        const plan = JSON.parse(fs.readFileSync(planFile, 'utf-8'))
        let updated = false
        for (const t of plan.tasks || []) {
          if (t.id === taskId) {
            t.worker = worker
            updated = true
            break
          }
        }

        if (updated) {
          fs.writeFileSync(planFile, JSON.stringify(plan, null, 2), 'utf-8')
          // Revoke approval when plan is edited
          if (fs.existsSync(approvedFile)) {
            try { fs.unlinkSync(approvedFile) } catch {}
          }
          return { ok: true, plan, isApproved: false }
        }
        return { ok: false, error: `Task ${taskId} not found` }
      }

      // 3. ACTION: REJECT / DISCARD PLAN
      if (action === 'reject') {
        if (fs.existsSync(planFile)) {
          try { fs.unlinkSync(planFile) } catch {}
        }
        if (fs.existsSync(approvedFile)) {
          try { fs.unlinkSync(approvedFile) } catch {}
        }
        return { ok: true, message: 'Plan discarded' }
      }

      // 4. ACTION: GENERATE (HEADLESS)
      // Synthesize BRIEF.md comprehensively from the entire user chat history
      if (fs.existsSync(chatFile)) {
        try {
          const history = JSON.parse(fs.readFileSync(chatFile, 'utf-8'))
          const userMsgs = history
            .filter((m: any) => m.role === 'user' && !m.text.includes('⚡ Generate DAG Plan'))
            .map((m: any) => m.text.trim())
            .filter(Boolean)

          if (userMsgs.length > 0) {
            // Join all user requirements chronologically so no prompt instruction is lost
            const consolidatedBrief = `# Project Requirements & Specifications\n\n` + 
              userMsgs.map((msg: string, idx: number) => `### Instruction / Requirement #${idx + 1}\n${msg}`).join('\n\n')
            fs.writeFileSync(briefPath, consolidatedBrief, 'utf-8')
          }
        } catch {}
      }

      // If force re-plan, remove old plan.json and approval
      if (body.force) {
        if (fs.existsSync(planFile)) {
          try { fs.unlinkSync(planFile) } catch {}
        }
        if (fs.existsSync(approvedFile)) {
          try { fs.unlinkSync(approvedFile) } catch {}
        }
      }

      // Execute ringmaster plan --headless in projectDir with explicit config fallback
      const rootConfig = rootTomlPath()
      const { stdout } = await runCommand(ringmasterBin, ['plan', '--headless', '--config', rootConfig], {
        cwd: projectDir,
        timeout: 120000
      })

      let plan = null
      if (fs.existsSync(planFile)) {
        try {
          plan = JSON.parse(fs.readFileSync(planFile, 'utf-8'))
        } catch {}
      }

      return {
        ok: true,
        message: 'Plan generated successfully',
        output: stdout,
        plan,
        isApproved: fs.existsSync(approvedFile)
      }
    } catch (err: any) {
      return {
        ok: false,
        error: err.message || 'Failed to generate plan',
        output: err.stdout || err.stderr || ''
      }
    }
  }
})

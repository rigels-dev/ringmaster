import path from 'node:path'
import fs from 'node:fs'
import { ringmasterBinPath, rootTomlPath, resolveProjectDir, runCommand } from '../utils/ringmaster'
import { briefByStep } from '../../utils/chatSteps.js'

export default defineEventHandler(async (event) => {
  const projectDir = resolveProjectDir()
  const ringmasterBin = ringmasterBinPath()
  const briefPath = path.join(projectDir, '.ringmaster', 'BRIEF.md')
  const planFile = path.join(projectDir, '.ringmaster', 'plan.json')
  const approvedFile = path.join(projectDir, '.ringmaster', 'plan.approved.json')
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

      // ACTION: ANSWER AN OPEN QUESTION FROM THE MASTER
      if (action === 'answer_question') {
        const { questionId } = body
        const answer = typeof body.answer === 'string' ? body.answer.trim() : ''
        if (!questionId || !fs.existsSync(planFile)) {
          return { ok: false, error: 'questionId and an existing plan.json are required' }
        }
        const plan = JSON.parse(fs.readFileSync(planFile, 'utf-8'))
        const q = (plan.open_questions || []).find((x: any) => x.id === questionId)
        if (!q) {
          return { ok: false, error: `Question ${questionId} not found` }
        }
        if (answer) q.answer = answer
        else delete q.answer
        fs.writeFileSync(planFile, JSON.stringify(plan, null, 2), 'utf-8')
        // Answers are part of the plan hash, so any change needs a fresh approval
        if (fs.existsSync(approvedFile)) {
          try { fs.unlinkSync(approvedFile) } catch {}
        }
        return { ok: true, plan, isApproved: false }
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
      // BRIEF.md is written from everything the user asked in the chat,
      // grouped by step, so the planner has the whole history
      if (fs.existsSync(chatFile)) {
        try {
          // Listing the steps records a plan that has just finished, so the count below is current
          await runCommand(ringmasterBin, ['steps', '--json', '--config', rootTomlPath()], { cwd: projectDir, timeout: 15000 }).catch(() => {})
          const history = JSON.parse(fs.readFileSync(chatFile, 'utf-8'))
          const brief = briefByStep(history, finishedStepCount(projectDir), currentPlanFinished(projectDir))
          if (brief) {
            fs.mkdirSync(path.dirname(briefPath), { recursive: true })
            fs.writeFileSync(briefPath, brief, 'utf-8')
          }
        } catch {}
      }

      // A new plan needs a new approval. The old plan.json is left for the
      // planner, which reads it to name the tasks already done, and replaces it.
      if (body.force && fs.existsSync(approvedFile)) {
        try { fs.unlinkSync(approvedFile) } catch {}
      }

      // Execute ringmaster plan --headless in projectDir with explicit config fallback
      const rootConfig = rootTomlPath()
      const planArgs = ['plan', '--headless', '--config', rootConfig]
      if (body.force) planArgs.push('--force')
      const { stdout } = await runCommand(ringmasterBin, planArgs, {
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

// How many steps the project has finished, as recorded in steps.json
function finishedStepCount(projectDir: string): number {
  try {
    return JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'steps.json'), 'utf-8'))?.steps?.length || 0
  } catch {
    return 0
  }
}

// Whether every task of the current plan is done
function currentPlanFinished(projectDir: string): boolean {
  try {
    const plan = JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'plan.json'), 'utf-8'))
    const state = JSON.parse(fs.readFileSync(path.join(projectDir, '.ringmaster', 'state.json'), 'utf-8'))
    const tasks = plan?.tasks || []
    return tasks.length > 0 && tasks.every((t: any) => state?.task_states?.[t.id]?.status === 'COMPLETED')
  } catch {
    return false
  }
}

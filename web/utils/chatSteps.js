// How the chat relates to the project's steps. Kept apart from the UI state
// so it can be tested on its own.

// The step of every chat message. A message records the step it was written
// in. Messages saved before steps were recorded have none: they belong to
// step 1, except those written after the last plan once that plan has
// finished, which are about the next step.
export function stepsOfMessages(messages, finishedSteps, planFinished) {
  const lastPlan = messages.map(m => Boolean(m.planReady)).lastIndexOf(true)
  return messages.map((m, i) => {
    if (m.step) return m.step
    if (finishedSteps > 0 && planFinished && lastPlan >= 0 && i > lastPlan) return finishedSteps + 1
    return 1
  })
}

// Whether the saved conversation leaves the Master ready to plan. Only what
// was said since the last plan counts: that plan used up the readiness of
// everything before it.
export function readyToPlan(messages) {
  const lastPlan = messages.map(m => Boolean(m.planReady)).lastIndexOf(true)
  return messages.slice(lastPlan + 1).some(m => m.role === 'master' && (m.ready || (
    // Conversations saved before readiness was recorded
    !m.step && /Generate Plan|Rencana Proyek|Pembagian Task|Rencana Eksekusi|Task Breakdown|Rencana ini sudah actionable/.test(m.text || '')
  )))
}

// The message the UI adds to the chat when "Generate plan" is clicked. It is
// a button press, not a requirement, so it is left out of the brief and of
// the conversation the Master sees. The older wording is still recognised.
export function isGeneratePlanMessage(text) {
  const t = String(text ?? '')
  return t.includes('⚡ Generate DAG Plan') || t.trim() === 'Generate the plan from this conversation.'
}

// What the user asked for in each step: their messages, grouped by step,
// without the "Generate plan" button presses. Returns [{ step, requests }],
// in step order.
export function requestsByStep(messages, finishedSteps, planFinished) {
  const stepOf = stepsOfMessages(messages, finishedSteps, planFinished)
  const groups = new Map()
  messages.forEach((m, i) => {
    if (m.role !== 'user' || isGeneratePlanMessage(m.text)) return
    const text = String(m.text || '').trim()
    if (!text) return
    if (!groups.has(stepOf[i])) groups.set(stepOf[i], [])
    groups.get(stepOf[i]).push(text)
  })
  return [...groups.entries()].sort((a, b) => a[0] - b[0]).map(([step, requests]) => ({ step, requests }))
}

// The readiness signal the Master appends; it is for the UI, not for a reader
const READY_SIGNAL = /\[RINGMASTER_CAN_GENERATE:\s*TRUE\]/gi

// What the user and the Master agreed in each step: the Master's last answer
// that was ready to plan, before that step's plan was made. It holds the
// decisions the discussion settled (a version, a library, a default), which
// the user's own messages may only hint at ("use the latest Go").
// Returns a Map of step -> text.
export function agreedByStep(messages, finishedSteps, planFinished) {
  const stepOf = stepsOfMessages(messages, finishedSteps, planFinished)
  const agreed = new Map()
  messages.forEach((m, i) => {
    if (m.role !== 'master' || !m.ready) return
    const text = String(m.text || '').replace(READY_SIGNAL, '').trim()
    if (text) agreed.set(stepOf[i], text)
  })
  return agreed
}

// BRIEF.md: everything the user asked for, grouped by step, with what was
// agreed with the Master in that step, so the planner sees the whole history
// and the decisions made along the way.
export function briefByStep(messages, finishedSteps, planFinished) {
  const groups = requestsByStep(messages, finishedSteps, planFinished)
  if (!groups.length) return ''
  const agreed = agreedByStep(messages, finishedSteps, planFinished)
  const sections = groups.map(({ step, requests }) => {
    let section = `## Step ${step}\n\n` + requests.map((r, i) => `### Request ${i + 1}\n${r}`).join('\n\n')
    if (agreed.has(step)) section += `\n\n### Agreed with the Master\n${agreed.get(step)}`
    return section
  })
  return '# Project Requirements & Specifications\n\n' +
    'Later requests, and what was agreed with the Master, take precedence over earlier requests of the same step. ' +
    'Decisions of earlier steps still hold in later steps unless a later step changes them.\n\n' +
    `${sections.join('\n\n')}\n`
}

// The conversation the Master is given in chat: what was agreed in each
// earlier step, then the current step's own messages. Earlier steps' raw
// messages would push the current discussion out of a short window, while
// their decisions must not be lost. prompt is the message being answered;
// it is left out when the history already ends with it.
export function conversationForMaster(messages, finishedSteps, planFinished, prompt, max = 20) {
  const stepOf = stepsOfMessages(messages, finishedSteps, planFinished)
  const current = stepOf.length ? stepOf[stepOf.length - 1] : 1
  const agreed = agreedByStep(messages, finishedSteps, planFinished)
  const out = []
  for (const [step, text] of [...agreed.entries()].sort((a, b) => a[0] - b[0])) {
    if (step >= current) continue
    out.push({ role: 'user', content: `What did we agree for Step ${step}?` })
    out.push({ role: 'assistant', content: `Agreed for Step ${step}, and still in force unless we change it:\n\n${text}` })
  }
  const own = messages
    .map((m, i) => ({ m, step: stepOf[i] }))
    .filter(({ m, step }) => step === current && m.text && !isGeneratePlanMessage(m.text))
    .map(({ m }) => ({ role: m.role === 'user' ? 'user' : 'assistant', content: String(m.text) }))
  const last = own[own.length - 1]
  if (last && last.role === 'user' && last.content.trim() === String(prompt ?? '').trim()) own.pop()
  return out.concat(own.slice(-max))
}

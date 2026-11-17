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

// BRIEF.md: everything the user asked for, grouped by step, so the planner
// sees the whole history and a reader sees what each step was about.
export function briefByStep(messages, finishedSteps, planFinished) {
  const groups = requestsByStep(messages, finishedSteps, planFinished)
  if (!groups.length) return ''
  const sections = groups.map(({ step, requests }) =>
    `## Step ${step}\n\n` + requests.map((r, i) => `### Request ${i + 1}\n${r}`).join('\n\n'))
  return `# Project Requirements & Specifications\n\n${sections.join('\n\n')}\n`
}

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

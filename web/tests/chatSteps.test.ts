import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { stepsOfMessages, readyToPlan } from '../utils/chatSteps.js'

const PRECEDENCE = 'Later requests, and what was agreed with the Master, take precedence over earlier requests of the same step. ' +
  'Decisions of earlier steps still hold in later steps unless a later step changes them.\n\n'

const user = (text: string, extra = {}) => ({ role: 'user', text, ...extra })
const master = (text: string, extra = {}) => ({ role: 'master', text, ...extra })

test('messages keep the step they were written in', () => {
  const msgs = [user('a', { step: 1 }), master('b', { step: 1 }), user('c', { step: 2 })]
  assert.deepEqual(stepsOfMessages(msgs, 1, true), [1, 1, 2])
})

test('older messages: after a finished plan they belong to the next step', () => {
  const msgs = [user('build it'), master('plan', { planReady: true }), user('now add a UI')]
  assert.deepEqual(stepsOfMessages(msgs, 1, true), [1, 1, 2])
  // The plan is not finished yet: everything is still step 1
  assert.deepEqual(stepsOfMessages(msgs, 0, false), [1, 1, 1])
  // New messages carry their step; older ones follow the rule
  assert.deepEqual(stepsOfMessages([...msgs, master('ok', { step: 2 })], 1, true), [1, 1, 2, 2])
})

test('the Master is ready only if it said so after the last plan', () => {
  assert.equal(readyToPlan([master('enough detail', { ready: true, step: 1 })]), true)
  assert.equal(readyToPlan([master('more questions', { ready: false, step: 1 })]), false)

  // Ready, then a plan was made: used up
  assert.equal(readyToPlan([
    master('enough', { ready: true, step: 1 }),
    master('The plan is ready', { planReady: true, step: 1 })
  ]), false)

  // A new discussion after the plan, and the Master is ready again
  assert.equal(readyToPlan([
    master('enough', { ready: true, step: 1 }),
    master('The plan is ready', { planReady: true, step: 1 }),
    user('add a UI', { step: 2 }),
    master('which framework?', { ready: false, step: 2 }),
    master('got it', { ready: true, step: 2 })
  ]), true)
})

test('older conversations: readiness is read from the wording, only since the last plan', () => {
  assert.equal(readyToPlan([master('Here is the Task Breakdown')]), true)
  assert.equal(readyToPlan([
    master('Here is the Task Breakdown'),
    master('The plan is ready', { planReady: true }),
    user('buatkan UI sederhana')
  ]), false)
  // The wording only counts for messages saved before readiness was recorded
  assert.equal(readyToPlan([master('Task Breakdown follows once you answer', { ready: false, step: 2 })]), false)
})

import { requestsByStep, briefByStep, isGeneratePlanMessage, agreedByStep, conversationForMaster } from '../utils/chatSteps.js'

test('requests are grouped by step, without the Generate plan button presses', () => {
  const msgs = [
    user('Build a link store', { step: 1 }),
    master('Questions?', { step: 1 }),
    user('Use SQLite', { step: 1 }),
    user('Generate the plan from this conversation.', { step: 1 }),
    master('The plan is ready', { planReady: true, step: 1 }),
    user('Add a simple web UI', { step: 2 }),
    user('   ', { step: 2 })
  ]
  assert.deepEqual(requestsByStep(msgs, 1, true), [
    { step: 1, requests: ['Build a link store', 'Use SQLite'] },
    { step: 2, requests: ['Add a simple web UI'] }
  ])
  assert.equal(isGeneratePlanMessage('⚡ Generate DAG Plan from current requirements'), true)
})

test('the brief has a section per step', () => {
  const brief = briefByStep([
    user('Build a link store'),
    master('The plan is ready', { planReady: true }),
    user('buatkan UI sederhana')
  ], 1, true)
  assert.equal(brief, '# Project Requirements & Specifications\n\n' + PRECEDENCE +
    '## Step 1\n\n### Request 1\nBuild a link store\n\n' +
    '## Step 2\n\n### Request 1\nbuatkan UI sederhana\n')
  assert.equal(briefByStep([master('hello')], 0, false), '')
})

// The shorty scenario: step 1 asked for go 1.22, then for the latest Go, and
// the Master settled on Go 1.27. Step 2 must still know that.
const shorty = [
  master('Tell me what you want to build.', { step: 1 }),
  user('Build shorty. Module name: shorty, go 1.22.', { step: 1 }),
  master('Plan: Go 1.22, ServeMux ...', { step: 1, ready: true }),
  user('gunakan go versi terbaru', { step: 1 }),
  master('Update: go.mod uses go 1.27.\n[RINGMASTER_CAN_GENERATE: TRUE]', { step: 1, ready: true }),
  user('Generate the plan from this conversation.', { step: 1 }),
  master('The plan is ready: 4 tasks.', { step: 1, planReady: true }),
  user('Add a simple web UI', { step: 2 }),
  master('Which colours?', { step: 2 }),
  user('Dark', { step: 2 })
]

test('what was agreed in a step is its last ready answer, without the signal', () => {
  const agreed = agreedByStep(shorty, 1, true)
  assert.deepEqual([...agreed.entries()], [[1, 'Update: go.mod uses go 1.27.']])
})

test('the brief carries what was agreed, and says what takes precedence', () => {
  const brief = briefByStep(shorty, 1, true)
  assert.ok(brief.includes('### Request 2\ngunakan go versi terbaru\n\n### Agreed with the Master\nUpdate: go.mod uses go 1.27.'))
  assert.ok(brief.includes('Decisions of earlier steps still hold'))
  assert.ok(!brief.includes('RINGMASTER_CAN_GENERATE'))
})

test('the Master in a later step gets earlier decisions, then this step', () => {
  const conv = conversationForMaster(shorty, 1, true, 'Dark')
  assert.deepEqual(conv, [
    { role: 'user', content: 'What did we agree for Step 1?' },
    { role: 'assistant', content: 'Agreed for Step 1, and still in force unless we change it:\n\nUpdate: go.mod uses go 1.27.' },
    { role: 'user', content: 'Add a simple web UI' },
    { role: 'assistant', content: 'Which colours?' }
  ])
  // The current step is never cut down to earlier steps' raw messages
  const long = [...shorty, ...Array.from({ length: 30 }, (_, i) => user(`m${i}`, { step: 2 }))]
  const cut = conversationForMaster(long, 1, true, 'next', 20)
  assert.equal(cut.length, 22)
  assert.equal(cut[1].content.startsWith('Agreed for Step 1'), true)
})

test('the manifest lists of the web UI and the planner match', () => {
  const ts = readFileSync(new URL('../server/utils/chat.ts', import.meta.url), 'utf8').match(/MANIFEST_FILES = \[([^\]]*)\]/)[1]
  const go = readFileSync(new URL('../../internal/planner/prior.go', import.meta.url), 'utf8').match(/manifestFiles = \[\]string\{([^}]*)\}/)[1]
  const names = (s: string) => s.match(/['"][^'"]+['"]/g).map(n => n.slice(1, -1))
  assert.deepEqual(names(ts), names(go))
})

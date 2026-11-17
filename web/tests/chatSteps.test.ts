import { test } from 'node:test'
import assert from 'node:assert/strict'
import { stepsOfMessages, readyToPlan } from '../utils/chatSteps.js'

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

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { statusLabel, statusClass } from '../utils/status.js'

test('every task status the engine writes has a plain-word label', () => {
  const labels: Record<string, string> = {
    PENDING: 'Queued',
    RUNNING: 'Running',
    COMPLETED: 'Done',
    BLOCKED: 'Blocked',
    FAILED: 'Failed',
    NEEDS_REVIEW: 'Needs review',
    PAUSED: 'Paused',
    AWAITING_INPUT: 'Needs answer'
  }
  for (const [status, label] of Object.entries(labels)) {
    assert.equal(statusLabel(status), label, status)
  }
})

test('an unknown status is shown as words, not as a raw enum', () => {
  assert.equal(statusLabel('WAITING_FOR_MERGE'), 'Waiting for merge')
  assert.equal(statusLabel('READY'), 'Ready')
  assert.equal(statusLabel(''), '—')
  assert.equal(statusLabel(undefined), '—')
})

test('a failed task stands out at least as much as a blocked one', () => {
  assert.match(statusClass('FAILED'), /red/)
  assert.notEqual(statusClass('FAILED'), statusClass('PENDING'))
})

test('an unknown status falls back to the neutral badge', () => {
  assert.equal(statusClass('SOMETHING_NEW'), statusClass('PENDING'))
})

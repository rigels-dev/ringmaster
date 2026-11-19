import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { SUMMARY_PLACEHOLDER, insertSummary, needsSummary } from '../utils/releaseNotes.js'

const draft = `${SUMMARY_PLACEHOLDER}\n\n### Features\n\n- Link store (T01)\n`

test('the placeholder is the one the Go draft writes', () => {
  const go = readFileSync(new URL('../../internal/release/notes.go', import.meta.url), 'utf8')
  assert.ok(go.includes(`const SummaryPlaceholder = "${SUMMARY_PLACEHOLDER}"`))
})

test('the summary replaces the placeholder', () => {
  assert.equal(insertSummary(draft, '  shorty shortens links.  '), 'shorty shortens links.\n\n### Features\n\n- Link store (T01)\n')
  assert.equal(needsSummary(draft), true)
  assert.equal(needsSummary(insertSummary(draft, 'x')), false)
})

test('without the placeholder the summary goes on top, and edits stay', () => {
  assert.equal(insertSummary('\n### Mine\n', 'Sum.'), 'Sum.\n\n### Mine\n')
  assert.equal(insertSummary('', 'Sum.'), 'Sum.\n')
  assert.equal(insertSummary(draft, '  '), draft)
})

test('a summary with $ signs is inserted as it is', () => {
  assert.equal(insertSummary(SUMMARY_PLACEHOLDER, 'Costs $& less'), 'Costs $& less')
})

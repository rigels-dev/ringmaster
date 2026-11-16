import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { isGeneratePlanMessage, finishedWorkNote } from '../server/utils/chat.ts'

test('the generate-plan button message is recognised in both wordings', () => {
  assert.equal(isGeneratePlanMessage('Generate the plan from this conversation.'), true)
  assert.equal(isGeneratePlanMessage('⚡ Generate DAG Plan from current requirements and BRIEF.md'), true)
  assert.equal(isGeneratePlanMessage('Please generate the plan from this conversation, but use SQLite.'), false)
  assert.equal(isGeneratePlanMessage('Add due dates to cards'), false)
  assert.equal(isGeneratePlanMessage(undefined), false)
})

function project(files: Record<string, string>): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'rm-chat-'))
  fs.mkdirSync(path.join(dir, '.ringmaster'))
  for (const [name, content] of Object.entries(files)) {
    fs.writeFileSync(path.join(dir, '.ringmaster', name), content)
  }
  return dir
}

test('a new project has no finished work to mention', () => {
  assert.equal(finishedWorkNote(project({})), '')
  assert.equal(finishedWorkNote(project({ 'state.json': '{"task_states":{"T01":{"status":"RUNNING"}}}' })), '')
  assert.equal(finishedWorkNote(project({ 'state.json': 'not json' })), '')
})

test('finished tasks are listed in order, with titles from the state or the plan', () => {
  const note = finishedWorkNote(project({
    'state.json': JSON.stringify({
      task_states: {
        T10: { status: 'COMPLETED', title: 'Due dates' },
        T02: { status: 'COMPLETED' },
        T01: { status: 'COMPLETED', title: 'Link store' },
        T03: { status: 'NEEDS_REVIEW', title: 'Not done' }
      }
    }),
    'plan.json': JSON.stringify({ tasks: [{ id: 'T02', title: 'HTTP API' }] })
  }))
  const lines = note.split('\n')
  assert.deepEqual(lines.slice(1, 4), ['- T01: Link store', '- T02: HTTP API', '- T10: Due dates'])
  assert.ok(!note.includes('Not done'))
  assert.match(note, /do not propose building it again/)
})

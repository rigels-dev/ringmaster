import { test } from 'node:test'
import assert from 'node:assert/strict'
import { cliError } from '../server/utils/cli.ts'

test('the reason is taken from the CLI error line, without the usage text', () => {
  const stderr = 'Error: leave credentials out of the URL\nUsage:\n  ringmaster remote set <url> [flags]\n'
  assert.equal(cliError({ stderr }, 'fallback'), 'leave credentials out of the URL')
})

test('without an Error: line the first line is used', () => {
  assert.equal(cliError({ message: 'spawn ENOENT\nmore' }, 'fallback'), 'spawn ENOENT')
})

test('an empty failure falls back to the given text', () => {
  assert.equal(cliError({}, 'Failed to set the remote'), 'Failed to set the remote')
  assert.equal(cliError(undefined, 'Failed to set the remote'), 'Failed to set the remote')
})

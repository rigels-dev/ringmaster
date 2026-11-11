import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isSkillName } from '../server/utils/skills.ts'

test('skill names are plain folder names', () => {
  for (const name of ['go-style', 'deploy', 'api_docs', 'v2.review', 'A1']) {
    assert.equal(isSkillName(name), true, name)
  }
})

test('anything that could be read as an option or a path is refused', () => {
  for (const name of ['', '--config', '-x', '../etc', 'a/b', 'a b', '.hidden', 'x'.repeat(65), null, undefined, 42, ['deploy']]) {
    assert.equal(isSkillName(name), false, String(name))
  }
})

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { acceptanceOf } from '../server/utils/acceptance.ts'

test('the acceptance list is read as Ringmaster writes it', () => {
  const spec = '---\nid: T13\ntype: infra\nacceptance:\n  - "docker build -t shorty:test ."\n  - "grep -q \\"FROM\\" Dockerfile"\n  - go test ./...\nacceptance_timeout: 900\n---\n\nBody\n- not a command\n'
  assert.deepEqual(acceptanceOf(spec), ['docker build -t shorty:test .', 'grep -q "FROM" Dockerfile', 'go test ./...'])
})

test('an inline list is read, and nothing without frontmatter', () => {
  assert.deepEqual(acceptanceOf('---\nid: T01\nacceptance: ["make", "make test"]\n---\n'), ['make', 'make test'])
  assert.deepEqual(acceptanceOf('# no frontmatter\nacceptance:\n  - make\n'), [])
})

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { renderMarkdown } from '../utils/markdown.js'

test('markdown is rendered', () => {
  const html = renderMarkdown('## Plan\n\n- **store** first\n- then `router`\n\n| a | b |\n|---|---|\n| 1 | 2 |')
  assert.match(html, /<h2>Plan<\/h2>/)
  assert.match(html, /<strong>store<\/strong>/)
  assert.match(html, /<code>router<\/code>/)
  assert.match(html, /<table>/)
})

test('raw HTML in a reply is escaped, never executed', () => {
  const html = renderMarkdown('<script>alert(1)</script> <img src=x onerror=alert(1)>')
  assert.doesNotMatch(html, /<script/i)
  assert.doesNotMatch(html, /<img/i)
  assert.match(html, /&lt;script&gt;/)
})

test('unsafe link schemes are not turned into links', () => {
  const html = renderMarkdown('[x](javascript:alert(1)) [y](data:text/html,hi)')
  assert.doesNotMatch(html, /href="javascript:/i)
  assert.doesNotMatch(html, /href="data:/i)
})

test('links open in a new tab without access to the opener', () => {
  const html = renderMarkdown('[docs](https://example.com)')
  assert.match(html, /href="https:\/\/example\.com"/)
  assert.match(html, /target="_blank"/)
  assert.match(html, /rel="noopener noreferrer"/)
})

test('images are not rendered, so a reply cannot trigger a remote fetch', () => {
  const html = renderMarkdown('![pixel](https://example.com/p.png)')
  assert.doesNotMatch(html, /<img/i)
})

test('single newlines are kept as line breaks', () => {
  assert.match(renderMarkdown('one\ntwo'), /one<br>\s*two/)
})

test('empty input renders nothing', () => {
  assert.equal(renderMarkdown(''), '')
  assert.equal(renderMarkdown(undefined), '')
})

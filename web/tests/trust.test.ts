import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isLoopbackAddress, isLoopbackHost, isTrustedRequest, isTrustedUpgrade } from '../server/utils/trust.ts'

const h = (init: Record<string, string>) => new Headers(init)

test('loopback hosts are recognized with and without a port', () => {
  for (const host of ['localhost', 'localhost:12053', '127.0.0.1:12053', '[::1]:12053', 'LOCALHOST:3053']) {
    assert.equal(isLoopbackHost(host), true, host)
  }
  for (const host of ['', 'evil.example', 'evil.example:12053', 'localhost.evil.example', '192.168.1.5:12053', '127.0.0.1.evil.example']) {
    assert.equal(isLoopbackHost(host), false, host)
  }
})

test('the UI itself may call the API', () => {
  // A read: browsers send no Origin on a same-origin GET
  assert.equal(isTrustedRequest(h({ host: 'localhost:12053', 'sec-fetch-site': 'same-origin' })), true)
  // A write from our own page
  assert.equal(
    isTrustedRequest(h({ host: 'localhost:12053', origin: 'http://localhost:12053', 'sec-fetch-site': 'same-origin' })),
    true
  )
  // Through an SSH tunnel on another local port
  assert.equal(isTrustedRequest(h({ host: 'localhost:3053', origin: 'http://localhost:3053' })), true)
  // Typed into the address bar
  assert.equal(isTrustedRequest(h({ host: '127.0.0.1:12053', 'sec-fetch-site': 'none' })), true)
})

test('local tools without browser headers may call the API', () => {
  assert.equal(isTrustedRequest(h({ host: '127.0.0.1:12053' })), true)
})

test('other websites may not call the API', () => {
  // A cross-site POST: the browser names the attacking page
  assert.equal(
    isTrustedRequest(h({ host: 'localhost:12053', origin: 'https://evil.example', 'sec-fetch-site': 'cross-site' })),
    false
  )
  // Older browsers without Sec-Fetch-Site still send Origin
  assert.equal(isTrustedRequest(h({ host: 'localhost:12053', origin: 'https://evil.example' })), false)
  // A sandboxed iframe or a redirect chain reports an opaque origin
  assert.equal(isTrustedRequest(h({ host: 'localhost:12053', origin: 'null' })), false)
  // A cross-site read (an <img> or no-cors fetch) carries no Origin
  assert.equal(isTrustedRequest(h({ host: 'localhost:12053', 'sec-fetch-site': 'cross-site' })), false)
  // Another app on a different localhost port
  assert.equal(
    isTrustedRequest(h({ host: 'localhost:12053', origin: 'http://localhost:8080', 'sec-fetch-site': 'same-site' })),
    false
  )
})

test('DNS rebinding is refused: the Host is not this machine', () => {
  assert.equal(
    isTrustedRequest(h({ host: 'evil.example:12053', origin: 'http://evil.example:12053', 'sec-fetch-site': 'same-origin' })),
    false
  )
  assert.equal(isTrustedRequest(h({})), false)
})

test('the terminal socket requires a matching Origin', () => {
  assert.equal(isTrustedUpgrade(h({ host: 'localhost:12053', origin: 'http://localhost:12053' })), true)
  assert.equal(isTrustedUpgrade(h({ host: 'localhost:12053' })), false)
  assert.equal(isTrustedUpgrade(h({ host: 'localhost:12053', origin: 'https://evil.example' })), false)
  assert.equal(isTrustedUpgrade(h({ host: 'localhost:12053', origin: 'http://localhost:8080' })), false)
  assert.equal(isTrustedUpgrade(h({ host: 'evil.example', origin: 'http://evil.example' })), false)
})

test('only loopback peers count as local', () => {
  for (const addr of ['127.0.0.1', '::1', '::ffff:127.0.0.1']) assert.equal(isLoopbackAddress(addr), true, addr)
  for (const addr of [undefined, '', '192.168.1.5', '10.0.0.1']) assert.equal(isLoopbackAddress(addr), false, String(addr))
})

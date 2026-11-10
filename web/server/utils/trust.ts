// Who may talk to the web UI's server. It runs commands and edits the fleet
// on behalf of whoever reaches it, so only pages it served itself, on this
// machine, are trusted. Kept free of imports so it can be tested with plain Node.

const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '::1', '[::1]'])
const LOOPBACK_ADDRS = new Set(['127.0.0.1', '::1', '::ffff:127.0.0.1'])

// Site relations a browser reports for a request our own page (or the address
// bar) made; "same-site" is left out because it covers other ports on localhost
const OWN_FETCH_SITES = new Set(['same-origin', 'none'])

// The Host header names this machine. A page on another domain that resolves
// to 127.0.0.1 (DNS rebinding) still sends its own name here.
export function isLoopbackHost(host: string): boolean {
  if (!host) return false
  try {
    return LOOPBACK_HOSTS.has(new URL(`http://${host}`).hostname)
  } catch {
    return false
  }
}

function isSameOrigin(origin: string, host: string): boolean {
  try {
    return new URL(origin).host === host
  } catch {
    return false
  }
}

// API requests: the Host must be loopback, and when the browser says where the
// request came from (Origin, Sec-Fetch-Site) it must be our own page. Browsers
// send Origin on every cross-site request that can change state, so a request
// without one comes from a same-origin read or a local tool such as curl.
export function isTrustedRequest(headers: Headers): boolean {
  const host = headers.get('host') || ''
  if (!isLoopbackHost(host)) return false
  const origin = headers.get('origin')
  if (origin !== null && !isSameOrigin(origin, host)) return false
  const site = headers.get('sec-fetch-site')
  if (site !== null && !OWN_FETCH_SITES.has(site)) return false
  return true
}

// The terminal is a full shell, so its WebSocket must name its origin: browsers
// always send Origin on an upgrade, and one that lacks it is refused. Only
// Origin is checked, since browsers disagree on Sec-Fetch-Site for WebSockets.
export function isTrustedUpgrade(headers: Headers): boolean {
  const host = headers.get('host') || ''
  return isLoopbackHost(host) && isSameOrigin(headers.get('origin') || '', host)
}

export function isLoopbackAddress(addr: string | undefined): boolean {
  return Boolean(addr && LOOPBACK_ADDRS.has(addr))
}

import { resolveProjectDir } from '../utils/ringmaster'
import { isLoopbackAddress, isTrustedUpgrade } from '../utils/trust'
import {
  createSession,
  getTerminalSession,
  killSession,
  resize,
  resolveTerminalCwd,
  writeInput,
  type TerminalPeer
} from '../utils/terminal'

// WebSocket for the web UI terminal. One socket per terminal tab:
//   client → {t:'attach', id?, cwd?, cols, rows} | {t:'i', d} | {t:'r', c, r} | {t:'kill'}
//   server → {t:'ready', id, title, cwd, replay} | {t:'o', d} | {t:'x', code} | {t:'error', message}
export default defineWebSocketHandler({
  upgrade(request) {
    if (!isTrustedUpgrade(request.headers)) {
      return new Response('Terminal is only available from localhost', { status: 403 })
    }
  },

  open(peer) {
    if (!isLoopbackAddress(peer.remoteAddress)) {
      peer.send(JSON.stringify({ t: 'error', message: 'Terminal is only available from localhost' }))
      peer.close(1008, 'forbidden')
    }
  },

  message(peer, message) {
    let msg: any
    try {
      msg = JSON.parse(message.text())
    } catch {
      return
    }
    const ctx = peer.context as { sessionId?: string; sink?: TerminalPeer }

    if (msg.t === 'attach') {
      let session = msg.id ? getTerminalSession(String(msg.id)) : undefined
      if (!session) {
        const cwd = resolveTerminalCwd(resolveProjectDir(), msg.cwd)
        session = createSession(cwd, Number(msg.cols) || 80, Number(msg.rows) || 24)
      } else {
        resize(session, Number(msg.cols), Number(msg.rows))
      }
      const sink: TerminalPeer = { send: (data) => peer.send(data) }
      session.peers.add(sink)
      ctx.sessionId = session.id
      ctx.sink = sink
      peer.send(JSON.stringify({ t: 'ready', id: session.id, title: session.title, cwd: session.cwd, replay: session.replay }))
      if (session.exited) peer.send(JSON.stringify({ t: 'x', code: session.exitCode }))
      return
    }

    const session = ctx.sessionId ? getTerminalSession(ctx.sessionId) : undefined
    if (!session) return
    if (msg.t === 'i' && typeof msg.d === 'string') writeInput(session, msg.d)
    else if (msg.t === 'r') resize(session, Number(msg.c), Number(msg.r))
    else if (msg.t === 'kill') killSession(session.id)
  },

  close(peer) {
    // The shell keeps running; only this view of it goes away
    const ctx = peer.context as { sessionId?: string; sink?: TerminalPeer }
    const session = ctx.sessionId ? getTerminalSession(ctx.sessionId) : undefined
    if (session && ctx.sink) session.peers.delete(ctx.sink)
  }
})

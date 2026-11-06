import { listSessions } from '../utils/terminal'

// Live terminal sessions, so the UI can restore its tabs after a page refresh
export default defineEventHandler(() => ({ ok: true, sessions: listSessions() }))

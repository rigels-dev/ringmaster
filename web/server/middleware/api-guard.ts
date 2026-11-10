import { isTrustedRequest } from '../utils/trust'

// Every /api route acts on the project (approving commands, starting runs,
// editing providers), so refuse callers that are not this UI on this machine.
// Without this, any website open in the user's browser could post to it.
export default defineEventHandler((event) => {
  if (!event.path.startsWith('/api/')) return
  if (!isTrustedRequest(event.headers)) {
    throw createError({
      statusCode: 403,
      statusMessage: 'Forbidden',
      message: 'The Ringmaster API is only available to its own web UI on localhost'
    })
  }
})

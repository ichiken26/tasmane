import { createError, defineEventHandler, getRequestHeader, getRequestURL, sendNoContent, setResponseHeader } from 'h3'
import { clearAuthSession } from '../../utils/auth'

export default defineEventHandler((event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const origin = getRequestHeader(event, 'origin')
  if (!origin || origin !== getRequestURL(event).origin)
    throw createError({ statusCode: 403, statusMessage: 'Cross-origin logout rejected' })
  clearAuthSession(event)
  return sendNoContent(event, 204)
})

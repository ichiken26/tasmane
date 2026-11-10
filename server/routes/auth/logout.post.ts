import { createError, defineEventHandler, getRequestHeader, getRequestURL, sendNoContent, setResponseHeader } from 'h3'
import { clearAuthSession } from '../../utils/auth'
import { clearMemoHandoff } from '../../utils/memoHandoff'

// 同一オリジン限定のセッション破棄
export default defineEventHandler((event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const origin = getRequestHeader(event, 'origin')
  if (!origin || origin !== getRequestURL(event).origin)
    throw createError({ statusCode: 403, statusMessage: 'Cross-origin logout rejected' })
  clearAuthSession(event)
  clearMemoHandoff(event)
  return sendNoContent(event, 204)
})

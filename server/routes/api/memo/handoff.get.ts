import { createError, defineEventHandler, getRequestHeader, setResponseHeader } from 'h3'
import { requireSession } from '../../../utils/auth'
import { authConfig } from '../../../utils/config'
import { getMemoHandoff, usableMemoHandoff } from '../../../utils/memoHandoff'

// メモアプリへ渡す Google ID トークン（同一オリジンの画面からのみ）
export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const fetchSite = getRequestHeader(event, 'sec-fetch-site')
  if (fetchSite && fetchSite !== 'same-origin')
    throw createError({ statusCode: 403, statusMessage: 'Cross-origin request rejected' })
  const config = authConfig(useRuntimeConfig(event))
  const session = await requireSession(event, config.sessionSecret)
  const handoff = usableMemoHandoff(await getMemoHandoff(event, config.sessionSecret), session.sub)
  if (!handoff) throw createError({ statusCode: 404, statusMessage: 'MEMO_REAUTH_REQUIRED', data: { code: 'MEMO_REAUTH_REQUIRED' } })
  return handoff
})

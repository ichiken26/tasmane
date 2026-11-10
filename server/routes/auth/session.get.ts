import { defineEventHandler, setResponseHeader } from 'h3'
import { requireSession } from '../../utils/auth'
import { authConfig } from '../../utils/config'

// 現行セッションの公開プロフィール返却
export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const config = authConfig(useRuntimeConfig(event))
  const session = await requireSession(event, config.sessionSecret)
  return { user: { email: session.email, name: session.name, picture: session.picture } }
})

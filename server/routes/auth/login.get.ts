import { defineEventHandler, getQuery, getRequestURL, sendRedirect, setResponseHeader } from 'h3'
import { setOAuthTransaction, safeReturnTo } from '../../utils/auth'
import { authConfig } from '../../utils/config'
import { randomBase64Url, sha256Base64Url } from '../../utils/security'

export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const config = authConfig(useRuntimeConfig(event))
  const now = Math.floor(Date.now() / 1000)
  const state = randomBase64Url()
  const nonce = randomBase64Url()
  const verifier = randomBase64Url(64)
  const returnTo = safeReturnTo(getQuery(event).returnTo)
  await setOAuthTransaction(event, { state, nonce, verifier, returnTo, expiresAt: now + 600 }, config.sessionSecret)

  const redirectUri = config.googleRedirectUri || new URL('/auth/callback', getRequestURL(event)).toString()
  const authorization = new URL('https://accounts.google.com/o/oauth2/v2/auth')
  authorization.search = new URLSearchParams({
    client_id: config.googleClientId,
    redirect_uri: redirectUri,
    response_type: 'code',
    scope: 'openid email profile https://www.googleapis.com/auth/tasks',
    access_type: 'offline',
    prompt: 'consent',
    include_granted_scopes: 'true',
    state,
    nonce,
    code_challenge: await sha256Base64Url(verifier),
    code_challenge_method: 'S256',
  }).toString()
  return sendRedirect(event, authorization.toString(), 302)
})

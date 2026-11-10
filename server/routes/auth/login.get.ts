import { defineEventHandler, getQuery, getRequestURL, sendRedirect, setResponseHeader } from 'h3'
import { currentSession, setOAuthTransaction, safeReturnTo } from '../../utils/auth'
import { authConfig } from '../../utils/config'
import { randomBase64Url, sha256Base64Url, sha256Hex } from '../../utils/security'

// Google OAuth 認可へのリダイレクト
export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const config = authConfig(useRuntimeConfig(event))
  const now = Math.floor(Date.now() / 1000)
  const state = randomBase64Url()
  const nonce = randomBase64Url()
  const verifier = randomBase64Url(64)
  const returnTo = safeReturnTo(getQuery(event).returnTo)
  // ログイン済みなら新しいトランザクションを作らない（同時に開かれたログインが進行中のトランザクションを上書きしないように）
  if (await currentSession(event, config.sessionSecret)) return sendRedirect(event, returnTo, 302)
  // OAuth トランザクション Cookie の発行
  await setOAuthTransaction(event, { state, nonce, verifier, returnTo, expiresAt: now + 600 }, config.sessionSecret)

  // 認可 URL の組み立てと 302 転送
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
    // 生nonceはトランザクションに保持し、Google には SHA-256 を渡す（メモアプリの Firebase へ引き継げる形式）
    nonce: await sha256Hex(nonce),
    code_challenge: await sha256Base64Url(verifier),
    code_challenge_method: 'S256',
  }).toString()
  return sendRedirect(event, authorization.toString(), 302)
})

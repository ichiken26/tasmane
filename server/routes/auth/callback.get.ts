import { createError, defineEventHandler, getQuery, getRequestURL, sendRedirect, setResponseHeader } from 'h3'
import { setSession, takeOAuthTransaction } from '../../utils/auth'
import { authConfig } from '../../utils/config'
import { verifyGoogleIDToken } from '../../utils/oauth'
import { memoHandoffExpiry } from '../../utils/oauth'
import { setMemoHandoff } from '../../utils/memoHandoff'
import { isEmailAllowed, sha256Hex } from '../../utils/security'
import { fetchGoAPI, trustedProxyHeaders } from '../../utils/upstream'

interface GoogleTokenResponse {
  id_token?: string
  refresh_token?: string
  scope?: string
  expires_in?: number
  error?: string
}
interface BootstrapResponse { userId?: string }

// Google OAuth コールバックの検証とセッション発行
export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const config = authConfig(useRuntimeConfig(event))
  const query = getQuery(event)
  const transaction = await takeOAuthTransaction(event, config.sessionSecret)
  // state/code とトランザクションの突合
  if (!transaction || typeof query.state !== 'string' || query.state !== transaction.state
    || typeof query.code !== 'string' || query.error)
    throw createError({ statusCode: 401, statusMessage: 'OAuth callback validation failed' })

  const redirectUri = config.googleRedirectUri || new URL('/auth/callback', getRequestURL(event)).toString()
  // 認可コードのトークン交換
  const tokenResponse = await fetch('https://oauth2.googleapis.com/token', {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      code: query.code,
      client_id: config.googleClientId,
      client_secret: config.googleClientSecret,
      redirect_uri: redirectUri,
      grant_type: 'authorization_code',
      code_verifier: transaction.verifier,
    }),
  })
  const tokens = await tokenResponse.json() as GoogleTokenResponse
  if (!tokenResponse.ok || tokens.error || !tokens.id_token) {
    // 原因調査用（トークンや個人情報は出さない）
    console.error('auth callback: Google token exchange failed', { status: tokenResponse.status, error: tokens.error ?? null, hasIdToken: Boolean(tokens.id_token) })
    throw createError({ statusCode: 502, statusMessage: 'Google token exchange failed' })
  }
  // ID トークン検証と内部ブートストラップ
  const identity = await verifyGoogleIDToken(tokens.id_token, {
    audience: config.googleClientId,
    nonce: await sha256Hex(transaction.nonce),
  })

  // 許可リスト外のアカウントはユーザー作成前に拒否
  if (!isEmailAllowed(identity.email, config.allowedEmails))
    throw createError({ statusCode: 403, statusMessage: 'This Google account is not allowed to use Tasmane' })

  const bootstrapBody = JSON.stringify({
    googleSubject: identity.sub,
    email: identity.email,
    displayName: identity.name,
    avatarUrl: identity.picture,
    refreshToken: tokens.refresh_token,
    scopes: tokens.scope?.split(' ').filter(Boolean) || [],
  })
  const bootstrap = await fetchGoAPI(event, '/internal/auth/bootstrap', {
    method: 'POST',
    headers: trustedProxyHeaders(new Headers({ 'content-type': 'application/json' }), 'oauth-bootstrap', config.internalBindingSecret),
    body: bootstrapBody,
  }, config.apiInternalUrl)
  if (!bootstrap.ok) {
    console.error('auth callback: user provisioning failed', { status: bootstrap.status, body: (await bootstrap.text().catch(() => '')).slice(0, 200) })
    throw createError({ statusCode: 502, statusMessage: 'User provisioning failed' })
  }
  const provisioned = await bootstrap.json() as BootstrapResponse
  if (!provisioned.userId) throw createError({ statusCode: 502, statusMessage: 'User provisioning returned no identity' })

  // セッション Cookie 発行と returnTo 転送
  const now = Math.floor(Date.now() / 1000)
  await setSession(event, {
    userId: provisioned.userId,
    sub: identity.sub,
    email: identity.email,
    name: identity.name,
    picture: identity.picture,
    issuedAt: now,
    expiresAt: now + 60 * 60 * 24 * 7,
  }, config.sessionSecret)
  // メモアプリへのログイン引き継ぎ（ID トークンの有効期限まで）
  await setMemoHandoff(event, {
    sub: identity.sub, idToken: tokens.id_token, rawNonce: transaction.nonce, expiresAt: memoHandoffExpiry(tokens.id_token, now),
  }, config.sessionSecret, now)
  return sendRedirect(event, transaction.returnTo, 303)
})

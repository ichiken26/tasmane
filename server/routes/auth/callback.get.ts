import { createError, defineEventHandler, getQuery, getRequestURL, sendRedirect, setResponseHeader } from 'h3'
import { setSession, takeOAuthTransaction } from '../../utils/auth'
import { authConfig } from '../../utils/config'
import { verifyGoogleIDToken } from '../../utils/oauth'
import { fetchGoAPI, trustedProxyHeaders } from '../../utils/upstream'

interface GoogleTokenResponse {
  id_token?: string
  refresh_token?: string
  scope?: string
  expires_in?: number
  error?: string
}
interface BootstrapResponse { userId?: string }

export default defineEventHandler(async (event) => {
  setResponseHeader(event, 'cache-control', 'no-store')
  const config = authConfig(useRuntimeConfig(event))
  const query = getQuery(event)
  const transaction = await takeOAuthTransaction(event, config.sessionSecret)
  if (!transaction || typeof query.state !== 'string' || query.state !== transaction.state
    || typeof query.code !== 'string' || query.error)
    throw createError({ statusCode: 401, statusMessage: 'OAuth callback validation failed' })

  const redirectUri = config.googleRedirectUri || new URL('/auth/callback', getRequestURL(event)).toString()
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
  if (!tokenResponse.ok || tokens.error || !tokens.id_token)
    throw createError({ statusCode: 502, statusMessage: 'Google token exchange failed' })
  const identity = await verifyGoogleIDToken(tokens.id_token, {
    audience: config.googleClientId,
    nonce: transaction.nonce,
  })

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
  if (!bootstrap.ok) throw createError({ statusCode: 502, statusMessage: 'User provisioning failed' })
  const provisioned = await bootstrap.json() as BootstrapResponse
  if (!provisioned.userId) throw createError({ statusCode: 502, statusMessage: 'User provisioning returned no identity' })

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
  return sendRedirect(event, transaction.returnTo, 303)
})

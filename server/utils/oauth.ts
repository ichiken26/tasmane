import { createError } from 'h3'
import { decodeBase64UrlBuffer, decodeBase64UrlJSON } from './security'

export interface GoogleIdentity {
  sub: string
  email: string
  name?: string
  picture?: string
}

interface TokenHeader { alg?: string, kid?: string }
interface TokenClaims extends GoogleIdentity {
  aud?: string
  iss?: string
  exp?: number
  iat?: number
  nonce?: string
  email_verified?: boolean
}
interface SigningJWK extends JsonWebKey { kid?: string }
interface GoogleJWKSet { keys?: SigningJWK[] }

let cachedJWKs: { expiresAt: number, keys: SigningJWK[] } | undefined

async function googleKeys(fetcher: typeof fetch, now: number): Promise<SigningJWK[]> {
  if (cachedJWKs && cachedJWKs.expiresAt > now) return cachedJWKs.keys
  const response = await fetcher('https://www.googleapis.com/oauth2/v3/certs')
  if (!response.ok) throw createError({ statusCode: 502, statusMessage: 'Google key discovery failed' })
  const body = await response.json() as GoogleJWKSet
  if (!Array.isArray(body.keys) || body.keys.length === 0)
    throw createError({ statusCode: 502, statusMessage: 'Google returned no signing keys' })
  const maxAge = /max-age=(\d+)/u.exec(response.headers.get('cache-control') || '')?.[1]
  cachedJWKs = { keys: body.keys, expiresAt: now + Math.min(Number(maxAge || 300), 3600) }
  return body.keys
}

export async function verifyGoogleIDToken(
  token: string,
  expected: { audience: string, nonce: string },
  options: { fetcher?: typeof fetch, now?: number, keys?: SigningJWK[] } = {},
): Promise<GoogleIdentity> {
  const parts = token.split('.')
  if (parts.length !== 3 || !parts[0] || !parts[1] || !parts[2])
    throw createError({ statusCode: 401, statusMessage: 'Invalid Google identity token' })
  let header: TokenHeader
  let claims: TokenClaims
  try {
    header = decodeBase64UrlJSON<TokenHeader>(parts[0])
    claims = decodeBase64UrlJSON<TokenClaims>(parts[1])
  } catch {
    throw createError({ statusCode: 401, statusMessage: 'Invalid Google identity token' })
  }
  if (header.alg !== 'RS256' || !header.kid)
    throw createError({ statusCode: 401, statusMessage: 'Unsupported Google identity token' })
  const now = options.now ?? Math.floor(Date.now() / 1000)
  const keys = options.keys ?? await googleKeys(options.fetcher ?? fetch, now)
  const jwk = keys.find(key => key.kid === header.kid && key.kty === 'RSA')
  if (!jwk) throw createError({ statusCode: 401, statusMessage: 'Unknown Google signing key' })
  const key = await crypto.subtle.importKey(
    'jwk', jwk, { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' }, false, ['verify'],
  )
  const validSignature = await crypto.subtle.verify(
    'RSASSA-PKCS1-v1_5', key, decodeBase64UrlBuffer(parts[2]),
    new TextEncoder().encode(`${parts[0]}.${parts[1]}`),
  )
  const validIssuer = claims.iss === 'https://accounts.google.com' || claims.iss === 'accounts.google.com'
  if (!validSignature || !validIssuer || claims.aud !== expected.audience || claims.nonce !== expected.nonce
    || !claims.exp || claims.exp <= now || (claims.iat !== undefined && claims.iat > now + 60)
    || !claims.sub || !claims.email || claims.email_verified !== true)
    throw createError({ statusCode: 401, statusMessage: 'Google identity token validation failed' })
  return { sub: claims.sub, email: claims.email, name: claims.name, picture: claims.picture }
}

export function resetGoogleKeyCacheForTests(): void {
  cachedJWKs = undefined
}

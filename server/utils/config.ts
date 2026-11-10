import type { RuntimeConfig } from 'nuxt/schema'
import { createError } from 'h3'

interface AuthRuntimeConfig {
  googleClientId: string
  googleClientSecret: string
  googleRedirectUri: string
  sessionSecret: string
  internalBindingSecret: string
  apiInternalUrl: string
  allowedEmails: string
}

// 認証ランタイム設定の検証と正規化
export function authConfig(config: RuntimeConfig): AuthRuntimeConfig {
  const value = config as RuntimeConfig & Partial<AuthRuntimeConfig>
  const missing = [
    !value.googleClientId && 'NUXT_GOOGLE_CLIENT_ID',
    !value.googleClientSecret && 'NUXT_GOOGLE_CLIENT_SECRET',
    !value.sessionSecret && 'NUXT_SESSION_SECRET',
    !value.internalBindingSecret && 'NUXT_INTERNAL_BINDING_SECRET',
  ].filter((name): name is string => Boolean(name))
  if (missing.length)
    throw createError({ statusCode: 503, statusMessage: `Authentication is not configured (${missing.join(', ')})` })
  if (value.sessionSecret.length < 32 || value.internalBindingSecret.length < 32)
    throw createError({ statusCode: 503, statusMessage: 'Authentication secrets do not meet minimum length' })
  return {
    googleClientId: value.googleClientId,
    googleClientSecret: value.googleClientSecret,
    googleRedirectUri: value.googleRedirectUri || '',
    sessionSecret: value.sessionSecret,
    internalBindingSecret: value.internalBindingSecret,
    apiInternalUrl: value.apiInternalUrl || '',
    allowedEmails: value.allowedEmails || '',
  }
}

// メモアプリのオリジン（http(s) 以外・不正値は無効）
export function memoAppOrigin(value: unknown): string {
  if (typeof value !== 'string' || !value.trim()) return ''
  try {
    const url = new URL(value.trim())
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.origin : ''
  } catch { return '' }
}

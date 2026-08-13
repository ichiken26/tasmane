import type { RuntimeConfig } from 'nuxt/schema'
import { createError } from 'h3'

interface AuthRuntimeConfig {
  googleClientId: string
  googleClientSecret: string
  googleRedirectUri: string
  sessionSecret: string
  internalBindingSecret: string
  apiInternalUrl: string
}

export function authConfig(config: RuntimeConfig): AuthRuntimeConfig {
  const value = config as RuntimeConfig & Partial<AuthRuntimeConfig>
  if (!value.googleClientId || !value.googleClientSecret || !value.sessionSecret || !value.internalBindingSecret)
    throw createError({ statusCode: 503, statusMessage: 'Authentication is not configured' })
  if (value.sessionSecret.length < 32 || value.internalBindingSecret.length < 32)
    throw createError({ statusCode: 503, statusMessage: 'Authentication secrets do not meet minimum length' })
  return {
    googleClientId: value.googleClientId,
    googleClientSecret: value.googleClientSecret,
    googleRedirectUri: value.googleRedirectUri || '',
    sessionSecret: value.sessionSecret,
    internalBindingSecret: value.internalBindingSecret,
    apiInternalUrl: value.apiInternalUrl || '',
  }
}

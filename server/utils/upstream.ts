import type { H3Event } from 'h3'
import { createError } from 'h3'

interface FetchBinding { fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> }

interface CloudflareContext { env?: Record<string, unknown> }

function binding(event: H3Event): FetchBinding | undefined {
  const cloudflare = (event.context as typeof event.context & { cloudflare?: CloudflareContext }).cloudflare
  const candidate = cloudflare?.env?.TASMANE_API as Partial<FetchBinding> | undefined
  return candidate && typeof candidate.fetch === 'function' ? candidate as FetchBinding : undefined
}

export async function fetchGoAPI(event: H3Event, path: string, init: RequestInit, internalURL: string): Promise<Response> {
  const service = binding(event)
  if (service) return service.fetch(new URL(path, 'http://tasmane-api.internal'), init)
  if (!internalURL || !/^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?(?:\/|$)/u.test(internalURL))
    throw createError({ statusCode: 503, statusMessage: 'Internal API binding is unavailable' })
  return fetch(new URL(path, internalURL), init)
}

export function trustedProxyHeaders(source: Headers, userId: string, bindingSecret: string): Headers {
  const headers = new Headers(source)
  for (const name of [...headers.keys()]) {
    if (name.toLowerCase().startsWith('x-tasmane-')) headers.delete(name)
  }
  for (const name of ['host', 'connection', 'content-length', 'cookie', 'authorization', 'cf-connecting-ip', 'x-forwarded-for', 'x-forwarded-host', 'x-forwarded-proto'])
    headers.delete(name)
  headers.set('X-Tasmane-Authenticated-User', userId)
  headers.set('X-Tasmane-Internal-Binding', bindingSecret)
  return headers
}

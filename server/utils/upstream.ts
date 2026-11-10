import type { H3Event } from 'h3'
import { createError } from 'h3'

interface FetchBinding { fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> }

interface CloudflareContext { env?: Record<string, unknown> }

// Workers VPC Service の宛先 Host（ルーティングには使われず Host ヘッダのみに入る）
const VPC_SERVICE_ORIGIN = 'http://tasmane-api.internal'

// 自宅サーバー Go API への Workers VPC Service バインディング（Cloudflare Tunnel 経由）の取得
function binding(event: H3Event): FetchBinding | undefined {
  const cloudflare = (event.context as typeof event.context & { cloudflare?: CloudflareContext }).cloudflare
  const candidate = cloudflare?.env?.TASMANE_API as Partial<FetchBinding> | undefined
  return candidate && typeof candidate.fetch === 'function' ? candidate as FetchBinding : undefined
}

// Go API への内部転送（本番は VPC Service のみ。ローカル開発だけ loopback へ fallback）
export async function fetchGoAPI(event: H3Event, path: string, init: RequestInit, internalURL: string): Promise<Response> {
  const service = binding(event)
  if (service) {
    try {
      return await service.fetch(new URL(path, VPC_SERVICE_ORIGIN), init)
    } catch {
      // Tunnel 断・自宅サーバー停止時（VPC Service の fetch は接続不能で例外）
      throw createError({ statusCode: 503, statusMessage: 'Tasmane API is unreachable' })
    }
  }
  if (!internalURL || !/^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?(?:\/|$)/u.test(internalURL))
    throw createError({ statusCode: 503, statusMessage: 'Internal API binding is unavailable' })
  return fetch(new URL(path, internalURL), init)
}

// Go へ渡せる変更経路（Worker が判定したものだけ）
export type ChangeVia = 'WEB' | 'WEB_MCP'

// 信頼済み内部プロキシヘッダの再構築
export function trustedProxyHeaders(source: Headers, userId: string, bindingSecret: string, via: ChangeVia = 'WEB'): Headers {
  const headers = new Headers(source)
  for (const name of [...headers.keys()]) {
    if (name.toLowerCase().startsWith('x-tasmane-')) headers.delete(name)
  }
  for (const name of ['host', 'connection', 'content-length', 'cookie', 'authorization', 'cf-connecting-ip', 'x-forwarded-for', 'x-forwarded-host', 'x-forwarded-proto'])
    headers.delete(name)
  headers.set('X-Tasmane-Authenticated-User', userId)
  headers.set('X-Tasmane-Internal-Binding', bindingSecret)
  if (via === 'WEB_MCP') headers.set('X-Tasmane-Via', 'WEB_MCP')
  return headers
}

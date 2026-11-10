import type { H3Event } from 'h3'
import { createError, getMethod, getRequestHeaders, getRequestURL, readRawBody, sendWebResponse } from 'h3'
import { requireSession } from './auth'
import { authConfig } from './config'
import { fetchGoAPI, trustedProxyHeaders } from './upstream'
import { isSameOriginWrite } from './security'

// セッション検証済みリクエストの Go API 転送
export async function proxyAuthenticatedGoAPI(event: H3Event, rewritePath: (pathname: string) => string) {
  // セッション検証と上流パスの組み立て
  const config = authConfig(useRuntimeConfig(event))
  const session = await requireSession(event, config.sessionSecret)
  const method = getMethod(event)
  const incomingURL = getRequestURL(event)
  const upstreamPath = `${rewritePath(incomingURL.pathname)}${incomingURL.search}`
  const incoming = new Headers(getRequestHeaders(event) as HeadersInit)
  // Cookie 認証の状態変更は同一オリジンからのみ受け付ける（CSRF 対策）
  if (!isSameOriginWrite(method, incoming, incomingURL.origin))
    throw createError({ statusCode: 403, statusMessage: 'Cross-origin request rejected' })
  // WebMCP（ブラウザ内Agent）経由の操作はフロントが付与する印で判定し、履歴の via を WEB_MCP にする
  const via = incoming.get('x-tasmane-client') === 'webmcp' ? 'WEB_MCP' : 'WEB'
  const headers = trustedProxyHeaders(incoming, session.userId, config.internalBindingSecret, via)
  const rawBody = method === 'GET' || method === 'HEAD' ? undefined : await readRawBody(event, false)
  // Node の Buffer はプール共有のため .buffer 全体ではなく該当範囲だけを切り出す
  const body = rawBody ? rawBody.buffer.slice(rawBody.byteOffset, rawBody.byteOffset + rawBody.byteLength) as ArrayBuffer : undefined
  const response = await fetchGoAPI(event, upstreamPath, { method, headers, body }, config.apiInternalUrl)
  // 上流応答の Cookie 除去と返却
  const responseHeaders = new Headers(response.headers)
  responseHeaders.delete('set-cookie')
  responseHeaders.delete('server')
  return sendWebResponse(event, new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  }))
}

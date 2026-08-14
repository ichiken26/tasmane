import type { H3Event } from 'h3'
import { getMethod, getRequestHeaders, getRequestURL, readRawBody, sendWebResponse } from 'h3'
import { requireSession } from './auth'
import { authConfig } from './config'
import { fetchGoAPI, trustedProxyHeaders } from './upstream'

export async function proxyAuthenticatedGoAPI(event: H3Event, rewritePath: (pathname: string) => string) {
  const config = authConfig(useRuntimeConfig(event))
  const session = await requireSession(event, config.sessionSecret)
  const method = getMethod(event)
  const incomingURL = getRequestURL(event)
  const upstreamPath = `${rewritePath(incomingURL.pathname)}${incomingURL.search}`
  const headers = trustedProxyHeaders(new Headers(getRequestHeaders(event) as HeadersInit), session.userId, config.internalBindingSecret)
  const rawBody = method === 'GET' || method === 'HEAD' ? undefined : await readRawBody(event, false)
  const body = rawBody ? rawBody.slice().buffer as ArrayBuffer : undefined
  const response = await fetchGoAPI(event, upstreamPath, { method, headers, body }, config.apiInternalUrl)
  const responseHeaders = new Headers(response.headers)
  responseHeaders.delete('set-cookie')
  responseHeaders.delete('server')
  return sendWebResponse(event, new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  }))
}

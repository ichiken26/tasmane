import { defineEventHandler, getMethod, getRequestHeaders, getRequestURL, readRawBody, sendWebResponse } from 'h3'
import { requireSession } from '../../utils/auth'
import { authConfig } from '../../utils/config'
import { fetchGoAPI, trustedProxyHeaders } from '../../utils/upstream'

export default defineEventHandler(async (event) => {
  const config = authConfig(useRuntimeConfig(event))
  const session = await requireSession(event, config.sessionSecret)
  const method = getMethod(event)
  const incomingURL = getRequestURL(event)
  const upstreamPath = `${incomingURL.pathname}${incomingURL.search}`
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
})

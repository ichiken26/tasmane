import { defineEventHandler, setResponseHeaders } from 'h3'
import { memoAppOrigin } from '../utils/config'

// 応答セキュリティヘッダの付与
export default defineEventHandler((event) => {
  const memoOrigin = memoAppOrigin(useRuntimeConfig(event).public.memoAppOrigin)
  setResponseHeaders(event, {
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
    'Content-Security-Policy': [
      "default-src 'self'",
      "base-uri 'self'",
      "frame-ancestors 'none'",
      // メモアプリの埋め込みだけを許可
      `frame-src ${memoOrigin || "'none'"}`,
      "form-action 'self'",
      "img-src 'self' data: https://*.googleusercontent.com",
      "font-src 'self' https://fonts.gstatic.com data:",
      "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
      "script-src 'self' 'unsafe-inline'",
      "connect-src 'self'",
      "object-src 'none'",
    ].join('; '),
  })
})

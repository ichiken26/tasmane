import { defineEventHandler } from 'h3'
import { proxyAuthenticatedGoAPI } from '../../../utils/proxyGo'

// /api/v1 配下の認証済み Go API プロキシ
export default defineEventHandler(event =>
  proxyAuthenticatedGoAPI(event, pathname => pathname.replace(/^\/api(?=\/)/u, '')),
)

import { defineEventHandler } from 'h3'
import { proxyAuthenticatedGoAPI } from '../../../utils/proxyGo'

export default defineEventHandler(event =>
  proxyAuthenticatedGoAPI(event, pathname => pathname.replace(/^\/api(?=\/)/u, '')),
)

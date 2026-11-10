import type { H3Event } from 'h3'
import { createError, deleteCookie, getCookie, setCookie } from 'h3'
import { seal, unseal } from './security'

export const SESSION_COOKIE = '__Host-tasmane-session'
export const OAUTH_COOKIE = '__Host-tasmane-oauth'

export interface OAuthTransaction {
  state: string
  nonce: string
  verifier: string
  returnTo: string
  expiresAt: number
}

export interface SessionIdentity {
  userId: string
  sub: string
  email: string
  name?: string
  picture?: string
  issuedAt: number
  expiresAt: number
}

const cookieBase = { httpOnly: true, secure: true, sameSite: 'lax' as const, path: '/' }

// OAuth トランザクション Cookie の発行
export async function setOAuthTransaction(event: H3Event, value: OAuthTransaction, secret: string): Promise<void> {
  setCookie(event, OAUTH_COOKIE, await seal(value, secret), { ...cookieBase, maxAge: 600 })
}

// 消費型 OAuth トランザクション Cookie の取得
export async function takeOAuthTransaction(event: H3Event, secret: string): Promise<OAuthTransaction | null> {
  const cookie = getCookie(event, OAUTH_COOKIE)
  deleteCookie(event, OAUTH_COOKIE, cookieBase)
  if (!cookie) return null
  const value = await unseal<OAuthTransaction>(cookie, secret)
  if (!value || value.expiresAt <= Math.floor(Date.now() / 1000)) return null
  return value
}

// セッション Cookie の発行
export async function setSession(event: H3Event, value: SessionIdentity, secret: string): Promise<void> {
  setCookie(event, SESSION_COOKIE, await seal(value, secret), { ...cookieBase, maxAge: 60 * 60 * 24 * 7 })
}

// セッション Cookie の削除
export function clearAuthSession(event: H3Event): void {
  deleteCookie(event, SESSION_COOKIE, cookieBase)
}

// 有効なセッション Cookie の読み取り（無効・期限切れは null）
export async function readValidSession(cookie: string | undefined, secret: string, now = Math.floor(Date.now() / 1000)): Promise<SessionIdentity | null> {
  if (!cookie) return null
  const session = await unseal<SessionIdentity>(cookie, secret)
  return session && session.userId && session.expiresAt > now ? session : null
}

// リクエストのログイン済みセッション（任意）
export async function currentSession(event: H3Event, secret: string): Promise<SessionIdentity | null> {
  return readValidSession(getCookie(event, SESSION_COOKIE), secret)
}

// 有効セッションの必須取得
export async function requireSession(event: H3Event, secret: string): Promise<SessionIdentity> {
  const cookie = getCookie(event, SESSION_COOKIE)
  if (!cookie) throw createError({ statusCode: 401, statusMessage: 'Authentication required' })
  const session = await unseal<SessionIdentity>(cookie, secret)
  if (!session || !session.userId || session.expiresAt <= Math.floor(Date.now() / 1000)) {
    clearAuthSession(event)
    throw createError({ statusCode: 401, statusMessage: 'Authentication required' })
  }
  return session
}

// オープンリダイレクト防止の相対パス正規化
export function safeReturnTo(input: unknown): string {
  if (typeof input !== 'string' || !input.startsWith('/') || input.startsWith('//') || input.includes('\\')) return '/'
  return input
}

import type { H3Event } from 'h3'
import { deleteCookie, getCookie, setCookie } from 'h3'
import { seal, unseal } from './security'

// メモアプリ（Firebase 認証）へのログイン引き継ぎ用 Google ID トークン
// - ログイン時に受け取った ID トークンを暗号化 Cookie で短時間（トークン有効期限まで）保持する
// - 同一オリジンの埋め込みフレーム制御からのみ取得し、postMessage で許可済みメモアプリへだけ渡す
export const MEMO_HANDOFF_COOKIE = '__Host-tasmane-memo'
const cookieBase = { httpOnly: true, secure: true, sameSite: 'lax' as const, path: '/' }
const SAFETY_MARGIN_SECONDS = 60

export interface MemoHandoff {
  sub: string
  idToken: string
  rawNonce: string
  expiresAt: number
}

// 引き継ぎ値の保存
export async function setMemoHandoff(event: H3Event, value: MemoHandoff, secret: string, now = Math.floor(Date.now() / 1000)): Promise<void> {
  const maxAge = value.expiresAt - now
  if (maxAge <= SAFETY_MARGIN_SECONDS) return
  setCookie(event, MEMO_HANDOFF_COOKIE, await seal(value, secret), { ...cookieBase, maxAge })
}

// 引き継ぎ値の読み出し
export async function getMemoHandoff(event: H3Event, secret: string): Promise<MemoHandoff | null> {
  const cookie = getCookie(event, MEMO_HANDOFF_COOKIE)
  return cookie ? await unseal<MemoHandoff>(cookie, secret) : null
}

// 引き継ぎ値の破棄（ログアウト時）
export function clearMemoHandoff(event: H3Event): void {
  deleteCookie(event, MEMO_HANDOFF_COOKIE, cookieBase)
}

// 同じ Google アカウントで、期限まで余裕のあるトークンだけ使う
export function usableMemoHandoff(value: MemoHandoff | null, sub: string, now = Math.floor(Date.now() / 1000)): { idToken: string, rawNonce: string } | null {
  if (!value || value.sub !== sub || value.expiresAt - now <= SAFETY_MARGIN_SECONDS) return null
  return { idToken: value.idToken, rawNonce: value.rawNonce }
}

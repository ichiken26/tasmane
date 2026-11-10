import { describe, expect, it } from 'vitest'
import { readValidSession } from '../../server/utils/auth'
import { seal } from '../../server/utils/security'
import { shouldRedirectToLogin } from '../layers/core/composables/loginRedirect'

const secret = 'x'.repeat(48)
const identity = { userId: 'u1', sub: 'g', email: 'a@example.com', name: 'A', issuedAt: 100, expiresAt: 1_000 }

describe('login flow robustness', () => {
  // ログイン済みならコールバックの重複・ログイン開始の重複を無害化するための判定
  it('reads only a valid, unexpired session cookie', async () => {
    const sealed = await seal(identity, secret)
    expect(await readValidSession(sealed, secret, 900)).toMatchObject({ userId: 'u1' })
    expect(await readValidSession(sealed, secret, 1_000)).toBeNull()
    expect(await readValidSession(sealed, 'y'.repeat(48), 900)).toBeNull()
    expect(await readValidSession(undefined, secret, 900)).toBeNull()
    expect(await readValidSession(await seal({ ...identity, userId: '' }, secret), secret, 900)).toBeNull()
  })

  // Chrome の先読み（prerender）中のページや認証画面からはログインへ飛ばさない
  it('redirects to login only from a visible app page', () => {
    expect(shouldRedirectToLogin({ prerendering: false, pathname: '/dashboard' })).toBe(true)
    expect(shouldRedirectToLogin({ prerendering: true, pathname: '/dashboard' })).toBe(false)
    expect(shouldRedirectToLogin({ prerendering: false, pathname: '/auth/callback' })).toBe(false)
  })
})

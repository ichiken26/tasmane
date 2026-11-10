import { describe, expect, it } from 'vitest'
import { safeReturnTo } from '../../server/utils/auth'
import { verifyGoogleIDToken } from '../../server/utils/oauth'
import { isEmailAllowed, isSameOriginWrite, randomBase64Url, seal, unseal } from '../../server/utils/security'
import { trustedProxyHeaders } from '../../server/utils/upstream'

const encode = (value: unknown): string => Buffer.from(JSON.stringify(value)).toString('base64url')

describe('Cloudflare authentication gateway', () => {
  // Cookie暗号化と改ざん・鍵誤り拒否の確認
  it('encrypts cookies and rejects tampering or the wrong key', async () => {
    const firstSecret = 'a'.repeat(32)
    const secondSecret = 'b'.repeat(32)
    const sealed = await seal({ userId: 'u1', marker: 'not-plaintext' }, firstSecret)
    expect(sealed).not.toContain('not-plaintext')
    expect(await unseal(sealed, firstSecret)).toEqual({ userId: 'u1', marker: 'not-plaintext' })
    expect(await unseal(`${sealed.slice(0, -1)}${sealed.endsWith('a') ? 'b' : 'a'}`, firstSecret)).toBeNull()
    expect(await unseal(sealed, secondSecret)).toBeNull()
  })

  // なりすましヘッダ除去と信頼値注入の確認
  it('strips spoofed identity and forwarding headers before trusted injection', () => {
    const headers = trustedProxyHeaders(new Headers({
      'X-Tasmane-Authenticated-User': 'attacker',
      'X-Tasmane-Internal-Binding': 'attacker-secret',
      'X-Tasmane-Arbitrary': 'spoof',
      'X-Forwarded-For': 'spoofed-ip',
      'Cookie': '__Host-tasmane-session=must-not-reach-go',
      'Authorization': 'Bearer untrusted',
      'Content-Type': 'application/json',
    }), 'trusted-user', 'trusted-binding')
    expect(headers.get('X-Tasmane-Authenticated-User')).toBe('trusted-user')
    expect(headers.get('X-Tasmane-Internal-Binding')).toBe('trusted-binding')
    expect(headers.get('X-Tasmane-Arbitrary')).toBeNull()
    expect(headers.get('X-Forwarded-For')).toBeNull()
    expect(headers.get('Cookie')).toBeNull()
    expect(headers.get('Authorization')).toBeNull()
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  // 変更経路はWorkerが判定した値のみをGoへ渡す（クライアント指定のViaは破棄）
  it('forwards only the Worker-decided change channel to Go', () => {
    const spoofed = trustedProxyHeaders(new Headers({ 'X-Tasmane-Via': 'SYSTEM' }), 'u', 'b')
    expect(spoofed.get('X-Tasmane-Via')).toBeNull()
    const webmcp = trustedProxyHeaders(new Headers({ 'X-Tasmane-Via': 'SYSTEM' }), 'u', 'b', 'WEB_MCP')
    expect(webmcp.get('X-Tasmane-Via')).toBe('WEB_MCP')
  })

  // 状態変更リクエストは同一オリジンのみ（同一サイトの別サブドメインからのCSRFも拒否）
  it('accepts state-changing requests only from the same origin', () => {
    const origin = 'https://tasmane.kokage-studio.com'
    expect(isSameOriginWrite('GET', new Headers(), origin)).toBe(true)
    expect(isSameOriginWrite('POST', new Headers({ origin }), origin)).toBe(true)
    expect(isSameOriginWrite('PATCH', new Headers({ origin: 'https://evil.kokage-studio.com' }), origin)).toBe(false)
    expect(isSameOriginWrite('DELETE', new Headers(), origin)).toBe(false)
    expect(isSameOriginWrite('PUT', new Headers({ 'sec-fetch-site': 'same-origin' }), origin)).toBe(true)
    expect(isSameOriginWrite('POST', new Headers({ 'sec-fetch-site': 'same-site' }), origin)).toBe(false)
  })

  // 許可リスト設定時は列挙されたメールアドレスだけがログインできる
  it('restricts sign-in to the configured email allowlist', () => {
    expect(isEmailAllowed('anyone@example.com', '')).toBe(true)
    expect(isEmailAllowed('Me@Example.com', 'me@example.com, partner@example.com')).toBe(true)
    expect(isEmailAllowed('stranger@example.com', 'me@example.com')).toBe(false)
  })

  // 同一サイト相対パスのみをログイン後遷移先として受理
  it('accepts only same-site relative post-login destinations', () => {
    expect(safeReturnTo('/dashboard?sort=score')).toBe('/dashboard?sort=score')
    for (const unsafe of ['https://evil.example', '//evil.example', '/\\evil.example', '', null])
      expect(safeReturnTo(unsafe)).toBe('/')
  })

  // Google JWTの署名・audience・nonce・issuer・寿命・email検証
  it('verifies Google JWT signature, audience, nonce, issuer, lifetime and email', async () => {
    const pair = await crypto.subtle.generateKey(
      { name: 'RSASSA-PKCS1-v1_5', modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: 'SHA-256' },
      true,
      ['sign', 'verify'],
    )
    const publicJWK = await crypto.subtle.exportKey('jwk', pair.publicKey) as JsonWebKey & { kid: string }
    publicJWK.kid = 'test-key'
    const now = 2_000_000_000
    const header = encode({ alg: 'RS256', kid: 'test-key' })
    const claims = encode({
      iss: 'https://accounts.google.com', aud: 'client-id', sub: 'google-subject',
      email: 'verified@example.com', email_verified: true, nonce: 'expected-nonce', iat: now - 1, exp: now + 300,
    })
    const unsigned = `${header}.${claims}`
    const signature = await crypto.subtle.sign('RSASSA-PKCS1-v1_5', pair.privateKey, new TextEncoder().encode(unsigned))
    const token = `${unsigned}.${Buffer.from(signature).toString('base64url')}`
    await expect(verifyGoogleIDToken(token, { audience: 'client-id', nonce: 'expected-nonce' }, { now, keys: [publicJWK] }))
      .resolves.toMatchObject({ sub: 'google-subject', email: 'verified@example.com' })
    await expect(verifyGoogleIDToken(token, { audience: 'other-client', nonce: 'expected-nonce' }, { now, keys: [publicJWK] }))
      .rejects.toMatchObject({ statusCode: 401 })
  })

  // 高エントロピーなURL安全OAuth値の生成確認
  it('generates high-entropy URL-safe OAuth values', () => {
    const values = new Set(Array.from({ length: 20 }, () => randomBase64Url()))
    expect(values.size).toBe(20)
    for (const value of values) expect(value).toMatch(/^[A-Za-z0-9_-]{43}$/u)
  })
})

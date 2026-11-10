import { describe, expect, it } from 'vitest'
import { usableMemoHandoff } from '../../server/utils/memoHandoff'
import { sha256Hex } from '../../server/utils/security'

describe('memo app login handoff', () => {
  // Firebase の nonce 形式（生nonceのSHA-256 16進）
  it('hashes the raw nonce as lowercase hex SHA-256', async () => {
    expect(await sha256Hex('abc')).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad')
  })

  // 同じGoogleアカウントの、期限に余裕があるトークンだけ渡す
  it('returns the handoff token only for the same subject before expiry', () => {
    const handoff = { sub: 'google-sub', idToken: 'a.b.c', rawNonce: 'raw', expiresAt: 1_000 }
    expect(usableMemoHandoff(handoff, 'google-sub', 900)).toEqual({ idToken: 'a.b.c', rawNonce: 'raw' })
    expect(usableMemoHandoff(handoff, 'google-sub', 950)).toBeNull()
    expect(usableMemoHandoff(handoff, 'other-sub', 900)).toBeNull()
    expect(usableMemoHandoff(null, 'google-sub', 900)).toBeNull()
  })
})

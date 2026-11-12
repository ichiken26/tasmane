import { describe, expect, it } from 'vitest'
import { shareInFlight } from '../layers/core/composables/inFlight'

describe('shareInFlight', () => {
  // 同じGETが同時に呼ばれたら通信は1回（同じ結果を共有）
  it('shares one request among concurrent callers with the same key', async () => {
    const pending = new Map<string, Promise<unknown>>()
    let calls = 0
    let resolve!: (value: string) => void
    const load = () => { calls++; return new Promise<string>((r) => { resolve = r }) }
    const first = shareInFlight(pending, 'GET /dashboard', load)
    const second = shareInFlight(pending, 'GET /dashboard', load)
    resolve('ok')
    expect(await Promise.all([first, second])).toEqual(['ok', 'ok'])
    expect(calls).toBe(1)
  })

  // 完了後（失敗含む）は次の呼び出しで取り直す（古い結果を使い回さない）
  it('forgets the request once it settles', async () => {
    const pending = new Map<string, Promise<unknown>>()
    let calls = 0
    await expect(shareInFlight(pending, 'k', async () => { calls++; throw new Error('down') })).rejects.toThrow('down')
    expect(await shareInFlight(pending, 'k', async () => { calls++; return 2 })).toBe(2)
    expect(calls).toBe(2)
    expect(pending.size).toBe(0)
  })
})

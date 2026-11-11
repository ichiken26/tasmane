import { describe, expect, it } from 'vitest'
import { isEmbeddableMemoUrl, memoEmbedUrl, memoTitleFromUrl, opensInNewTab, parseMemoMessage, upsertMemoLink } from '../layers/memo/composables/memoBridge'

const origin = 'https://memo.kokage-studio.com'

describe('memo app bridge', () => {
  // 埋め込みURL（新規メモ／既存メモは編集モード）
  it('builds embed URLs on the memo app origin only', () => {
    expect(memoEmbedUrl(null, origin)).toBe('https://memo.kokage-studio.com/memo/new?mode=edit')
    expect(memoEmbedUrl('https://memo.kokage-studio.com/memo/memo-1%E3%81%82', origin)).toBe('https://memo.kokage-studio.com/memo/memo-1%E3%81%82?mode=edit')
    expect(memoEmbedUrl('https://evil.example/memo/1', origin)).toBeNull()
    expect(isEmbeddableMemoUrl('https://memo.kokage-studio.com/memo/x', origin)).toBe(true)
    expect(isEmbeddableMemoUrl('https://memo.kokage-studio.com.evil.example/memo/x', origin)).toBe(false)
    expect(isEmbeddableMemoUrl('not a url', origin)).toBe(false)
  })

  // メモアプリからのメッセージは送信元フレームとオリジンを両方確認
  it('accepts messages only from the embedded memo frame and origin', () => {
    const frame = {}
    const saved = { source: 'memo-app', type: 'memo-saved', memo: { id: 'm1', title: 'T', url: 'https://memo.kokage-studio.com/memo/m1' } }
    expect(parseMemoMessage({ origin, source: frame, data: saved }, frame, origin)).toEqual({ type: 'memo-saved', memo: saved.memo })
    expect(parseMemoMessage({ origin: 'https://evil.example', source: frame, data: saved }, frame, origin)).toBeNull()
    expect(parseMemoMessage({ origin, source: {}, data: saved }, frame, origin)).toBeNull()
    expect(parseMemoMessage({ origin, source: frame, data: { ...saved, memo: { ...saved.memo, url: 'https://evil.example/x' } } }, frame, origin)).toBeNull()
    expect(parseMemoMessage({ origin, source: frame, data: { source: 'memo-app', type: 'ready', signedIn: false, email: null } }, frame, origin)).toEqual({ type: 'ready', signedIn: false, email: null })
    expect(parseMemoMessage({ origin, source: frame, data: { source: 'memo-app', type: 'auth-result', ok: false, error: 'auth/invalid-credential' } }, frame, origin)).toEqual({ type: 'auth-result', ok: false, error: 'auth/invalid-credential' })
  })

  // 保存要求への応答（保存した／しなかった）を要求IDつきで受け取る
  it('parses save responses with their request id', () => {
    const frame = {}
    const saved = { source: 'memo-app', type: 'memo-saved', requestId: 'r1', memo: { id: 'm1', title: 'T', url: 'https://memo.kokage-studio.com/memo/m1' } }
    expect(parseMemoMessage({ origin, source: frame, data: saved }, frame, origin)).toEqual({ type: 'memo-saved', memo: saved.memo, requestId: 'r1' })
    expect(parseMemoMessage({ origin, source: frame, data: { source: 'memo-app', type: 'save-skipped', reason: 'empty', requestId: 'r1' } }, frame, origin)).toEqual({ type: 'save-skipped', reason: 'empty', requestId: 'r1' })
    expect(parseMemoMessage({ origin, source: frame, data: { source: 'memo-app', type: 'save-skipped', reason: 'other', requestId: 'r1' } }, frame, origin)).toBeNull()
  })

  // 同じURLは重複させずタイトルだけ更新
  it('upserts links by URL', () => {
    const links = [{ url: 'u1', title: 'a' }]
    expect(upsertMemoLink(links, { url: 'u1', title: 'b' })).toEqual([{ url: 'u1', title: 'b' }])
    expect(upsertMemoLink(links, { url: 'u2', title: 'c' })).toEqual([{ url: 'u1', title: 'a' }, { url: 'u2', title: 'c' }])
  })

  // 作成後に一部の紐付けが失敗しても、成功済みは再送せず未保存だけ再試行する
  it('retries failed task memo links without duplicating the created task or stored links', async () => {
    const { persistPendingMemoLinks } = await import('../layers/memo/composables/memoBridge')
    const drafts = [
      { url: 'https://memo.kokage-studio.com/memo/a', title: 'A' },
      { url: 'https://memo.kokage-studio.com/memo/b', title: 'B' },
    ]
    const firstCalls: string[] = []
    const first = await persistPendingMemoLinks(drafts, async (link) => {
      firstCalls.push(link.url)
      if (link.title === 'B') throw new Error('temporarily unavailable')
      return { ...link, id: 'stored-a' }
    })
    expect(firstCalls).toHaveLength(2)
    expect(first.failed).toHaveLength(1)
    expect(first.links[0]?.id).toBe('stored-a')
    expect(first.links[1]?.id).toBeUndefined()
    const retried: string[] = []
    const second = await persistPendingMemoLinks(first.links, async (link) => {
      retried.push(link.url)
      return { ...link, id: 'stored-b' }
    })
    expect(retried).toEqual(['https://memo.kokage-studio.com/memo/b'])
    expect(second.failed).toEqual([])
    expect(second.links.map(link => link.id)).toEqual(['stored-a', 'stored-b'])
  })

  // Ctrl/⌘/Shift/中クリックは別タブ（ブラウザ標準）、通常クリックはアプリ内表示
  it('detects modified clicks that should open a new tab', () => {
    expect(opensInNewTab({ button: 0, ctrlKey: true, metaKey: false, shiftKey: false })).toBe(true)
    expect(opensInNewTab({ button: 0, ctrlKey: false, metaKey: true, shiftKey: false })).toBe(true)
    expect(opensInNewTab({ button: 1, ctrlKey: false, metaKey: false, shiftKey: false })).toBe(true)
    expect(opensInNewTab({ button: 0, ctrlKey: false, metaKey: false, shiftKey: false })).toBe(false)
  })
})

describe('memoTitleFromUrl', () => {
  it('メモアプリのメモID（memo-yyyymmdd-名前）は名前だけを表示名にする', () => {
    expect(memoTitleFromUrl('https://memo.kokage-studio.com/memo/memo-20260520-%E3%83%86%E3%82%B9%E3%83%88-2?mode=edit')).toBe('テスト-2')
  })
  it('メモID以外はURL末尾をそのまま、取れなければ「メモ」', () => {
    expect(memoTitleFromUrl('https://example.com/notes/meeting')).toBe('meeting')
    expect(memoTitleFromUrl('https://example.com/')).toBe('メモ')
    expect(memoTitleFromUrl('https://example.com/memo/memo-2026-x')).toBe('memo-2026-x')
  })
  it('URLとして読めなければ null', () => {
    expect(memoTitleFromUrl('not a url')).toBeNull()
  })
})

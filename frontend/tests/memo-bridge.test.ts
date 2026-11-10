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

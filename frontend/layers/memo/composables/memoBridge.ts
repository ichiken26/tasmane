// メモアプリ（別オリジン）との連携ヘルパー
// - 埋め込みはメモアプリのオリジンのURLだけ
// - postMessage は送信元フレーム・オリジン・形式をすべて確認してから扱う

export interface MemoLinkDraft { url: string; title: string }
export type MemoMessage =
  | { type: 'ready'; signedIn: boolean; email: string | null }
  | { type: 'auth-result'; ok: boolean; error?: string }
  | { type: 'memo-saved'; memo: { id: string; title: string; url: string }; requestId?: string }
  | { type: 'save-skipped'; reason: 'empty' | 'signed-out' | 'failed'; requestId: string }

// メモアプリのURLか（オリジン完全一致）
export function isEmbeddableMemoUrl(url: string, memoOrigin: string): boolean {
  if (!memoOrigin) return false
  try { return new URL(url).origin === memoOrigin }
  catch { return false }
}

// 埋め込みURL（null は新規メモ。既存メモは編集モードで開く）
export function memoEmbedUrl(url: string | null, memoOrigin: string): string | null {
  if (!memoOrigin) return null
  if (url === null) return `${memoOrigin}/memo/new?mode=edit`
  if (!isEmbeddableMemoUrl(url, memoOrigin)) return null
  const target = new URL(url)
  target.searchParams.set('mode', 'edit')
  return target.toString()
}

// メモアプリからのメッセージの検証
export function parseMemoMessage(event: { origin: string; source: unknown; data: unknown }, frame: unknown, memoOrigin: string): MemoMessage | null {
  if (!frame || event.source !== frame || event.origin !== memoOrigin) return null
  const data = event.data as Record<string, unknown> | null
  if (!data || typeof data !== 'object' || data.source !== 'memo-app') return null
  if (data.type === 'ready' && typeof data.signedIn === 'boolean')
    return { type: 'ready', signedIn: data.signedIn, email: typeof data.email === 'string' ? data.email : null }
  if (data.type === 'auth-result' && typeof data.ok === 'boolean')
    return { type: 'auth-result', ok: data.ok, ...(typeof data.error === 'string' ? { error: data.error } : {}) }
  if (data.type === 'memo-saved') {
    const memo = data.memo as Record<string, unknown> | null
    if (!memo || typeof memo.id !== 'string' || typeof memo.title !== 'string' || typeof memo.url !== 'string') return null
    if (!isEmbeddableMemoUrl(memo.url, memoOrigin)) return null
    return { type: 'memo-saved', memo: { id: memo.id, title: memo.title, url: memo.url }, ...(typeof data.requestId === 'string' ? { requestId: data.requestId } : {}) }
  }
  if (data.type === 'save-skipped' && typeof data.requestId === 'string' && (data.reason === 'empty' || data.reason === 'signed-out' || data.reason === 'failed'))
    return { type: 'save-skipped', reason: data.reason, requestId: data.requestId }
  return null
}

// URL単位の追加・タイトル更新
export function upsertMemoLink<T extends MemoLinkDraft>(links: T[], link: T): T[] {
  const index = links.findIndex(item => item.url === link.url)
  if (index < 0) return [...links, link]
  return links.map((item, position) => position === index ? { ...item, title: link.title } : item)
}

// 作成済みタスクへ未保存リンクだけ登録。失敗したリンクはIDなしで保持し、再試行できるようにする
export async function persistPendingMemoLinks<T extends MemoLinkDraft & { id?: string }>(
  links: T[],
  add: (link: T) => Promise<{ id: string; url: string; title: string }>,
): Promise<{ links: T[]; failed: T[] }> {
  const pending = links.filter(link => !link.id)
  const results = await Promise.allSettled(pending.map(link => add(link)))
  const storedByOriginalUrl = new Map<string, { id: string; url: string; title: string }>()
  const failed: T[] = []
  results.forEach((result, index) => {
    const draft = pending[index]!
    if (result.status === 'fulfilled') storedByOriginalUrl.set(draft.url, result.value)
    else failed.push(draft)
  })
  return {
    links: links.map(link => {
      const stored = storedByOriginalUrl.get(link.url)
      return stored ? { ...link, ...stored } : link
    }),
    failed,
  }
}

// 別タブで開くクリックか（Ctrl/⌘/Shift/中クリック）
export function opensInNewTab(event: { button: number; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }): boolean {
  return event.button === 1 || event.ctrlKey || event.metaKey || event.shiftKey
}

// URLからのメモ表示名（メモアプリのメモID memo-yyyymmdd-名前 は名前だけ。URLでなければ null）
export function memoTitleFromUrl(url: string): string | null {
  let last: string
  try { last = decodeURIComponent(new URL(url).pathname.split('/').filter(Boolean).pop() || '') }
  catch { return null }
  return /^memo-\d{8}-(.+)$/.exec(last)?.[1] || last || 'メモ'
}

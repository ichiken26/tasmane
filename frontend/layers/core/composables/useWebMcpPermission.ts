import { useState } from '#app'

interface PendingPermission { summary: string; resolve: (approved: boolean) => void }

// WebMCP 書き込み操作のユーザー許可（ブラウザ上のダイアログで本人が承認する）
export function useWebMcpPermission() {
  const pending = useState<PendingPermission | null>('webmcp-permission', () => null)
  // 許可要求（同時に1件。前の要求は拒否扱い）
  const ask = (summary: string) => new Promise<boolean>((resolve) => {
    pending.value?.resolve(false)
    pending.value = { summary, resolve }
  })
  const answer = (approved: boolean) => {
    pending.value?.resolve(approved)
    pending.value = null
  }
  return { pending, ask, answer }
}

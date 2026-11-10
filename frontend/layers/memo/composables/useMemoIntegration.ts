import { computed } from 'vue'
import { useRuntimeConfig, useState } from '#imports'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import type { MemoLinkDraft } from './memoBridge'

// メモアプリ連携の状態（設定フラグ・表示中のメモ）
export function useMemoIntegration() {
  const api = useTasmaneApi()
  const memoOrigin = computed(() => String(useRuntimeConfig().public.memoAppOrigin || ''))
  const enabled = useState<boolean>('memo-integration-enabled', () => false)
  const loaded = useState<boolean>('memo-integration-loaded', () => false)
  // 右側パネルで表示中のメモ
  const openedMemo = useState<MemoLinkDraft | null>('memo-panel-open', () => null)
  // 右側パネルで保存されたメモ（紐付けのタイトル更新用）
  const savedMemo = useState<MemoLinkDraft | null>('memo-panel-saved', () => null)

  // 設定の取得（1回だけ）
  async function ensureLoaded() {
    if (loaded.value) return
    try {
      enabled.value = (await api.settings()).memoIntegrationEnabled === true
      loaded.value = true
    } catch { enabled.value = false }
  }

  // 連携のオン/オフ（ユーザー設定として保存）
  async function setEnabled(value: boolean) {
    const saved = await api.updateSettings({ memoIntegrationEnabled: value })
    enabled.value = saved.memoIntegrationEnabled === true
    loaded.value = true
  }

  return {
    setEnabled,
    memoOrigin, enabled, ensureLoaded, openedMemo, savedMemo,
    reportSaved: (memo: MemoLinkDraft) => { savedMemo.value = { url: memo.url, title: memo.title } },
    openMemo: (link: MemoLinkDraft) => { openedMemo.value = { url: link.url, title: link.title } },
    closeMemo: () => { openedMemo.value = null },
  }
}

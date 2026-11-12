import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (path: string) => readFileSync(path, 'utf8')

describe('memo link loading indicator', () => {
  // 紐付けの読み込み中は、チップの位置に読み込み中の表示（スピナー）を出す
  it('shows a spinner in the chip area while links are loading', () => {
    const chips = read('frontend/layers/memo/components/MemoLinkChips.vue')
    expect(chips).toMatch(/loading\?: boolean/)
    expect(chips).toMatch(/v-if="(props\.)?loading"[^>]*role="status"/)
    expect(chips).toContain('i-lucide-loader-circle')
    expect(chips).toContain('animate-spin')
  })

  // 既存タスクの編集画面は、読み込み開始から完了（失敗含む）まで loading を渡す
  it('passes the loading state from the task editor', () => {
    const editor = read('frontend/layers/task/components/TaskEditor.vue')
    expect(editor).toContain('const memoLinksLoading = ref(Boolean(props.task))')
    expect(editor).toMatch(/finally \{ memoLinksLoading\.value = false \}/)
    expect(editor).toMatch(/<MemoLinkChips[^>]*:loading="memoLinksLoading"/)
  })
})

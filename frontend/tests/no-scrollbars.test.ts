import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// スクロールバーは可能な限り出さない（必要な横スクロールはバーを隠す。ガントの時間軸だけは操作のため残す）
const read = (path: string) => readFileSync(path, 'utf8')

describe('scrollbars', () => {
  it('wraps the deadline tabs onto more rows instead of scrolling sideways, and lists tasks without an inner scroll', () => {
    const view = read('frontend/layers/dashboard/components/DashboardView.vue')
    const tabs = view.match(/<UTabs[^>]*deadlineTabs[^>]*>/)?.[0] ?? ''
    expect(tabs).toMatch(/flex-wrap/)
    expect(tabs).not.toMatch(/overflow-x-auto/)
    expect(view).not.toMatch(/max-h-96/)
  })

  it('does not cap the impact list height', () => {
    expect(read('frontend/layers/shared/components/ImpactModal.vue')).not.toMatch(/overflow-y-auto/)
  })

  it('hides the bar on scroll areas that remain', () => {
    expect(read('assets/css/main.css')).toMatch(/@utility scrollbar-none\s*\{[^}]*scrollbar-width:\s*none/)
    expect(read('layouts/default.vue')).toMatch(/overflow-y-auto scrollbar-none/)
    expect(read('frontend/layers/shared/components/AppFooter.vue')).toMatch(/overflow-x-auto scrollbar-none/)
  })
})

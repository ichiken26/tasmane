import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// ハンバーガーメニューでサイドバーを折りたためる（デスクトップは開閉、モバイルはメニューを開く）
describe('sidebar toggle', () => {
  const header = readFileSync('frontend/layers/shared/components/AppHeader.vue', 'utf8')
  const layout = readFileSync('layouts/default.vue', 'utf8')

  it('shows the hamburger button on every width and toggles the desktop sidebar', () => {
    const button = header.match(/<UButton[^>]*i-lucide-menu[^>]*\/>/)?.[0] ?? ''
    expect(button).not.toBe('')
    expect(button).not.toMatch(/lg:hidden/)
    expect(button).toMatch(/@click="onMenu"/)
    expect(button).toMatch(/:aria-expanded=/)
    expect(header).toMatch(/useSidebar\(\)/)
  })

  it('hides the desktop sidebar while collapsed', () => {
    expect(layout).toMatch(/useSidebar\(\)/)
    expect(layout).toMatch(/<aside[^>]*v-if="!collapsed"/)
  })
})

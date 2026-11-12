import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { GANTT_KEY_DELAY_MS, createKeyNudge } from '../layers/core/composables/gantt'

// Gantt のキーボード操作: ←→ は期限（バーの右端）だけを動かし、API は最後の操作から3秒後に1回だけ
describe('Gantt keyboard nudge', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('waits 3 seconds after the last key and commits the accumulated days once', () => {
    const commit = vi.fn()
    const nudge = createKeyNudge(commit)
    nudge.press('a', 1)
    vi.advanceTimersByTime(2000)
    nudge.press('a', 1)
    nudge.press('a', 1)
    expect(nudge.delta('a')).toBe(3)
    vi.advanceTimersByTime(GANTT_KEY_DELAY_MS - 1)
    expect(commit).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(commit).toHaveBeenCalledExactlyOnceWith('a', 3)
    expect(nudge.delta('a')).toBe(0)
    expect(GANTT_KEY_DELAY_MS).toBe(3000)
  })

  it('skips the API when the keys cancel out', () => {
    const commit = vi.fn()
    const nudge = createKeyNudge(commit)
    nudge.press('a', 1)
    nudge.press('a', -1)
    vi.advanceTimersByTime(GANTT_KEY_DELAY_MS)
    expect(commit).not.toHaveBeenCalled()
  })

  it('commits the previous task right away when another bar is operated, and on flush', () => {
    const commit = vi.fn()
    const nudge = createKeyNudge(commit)
    nudge.press('a', -2)
    nudge.press('b', 1)
    expect(commit).toHaveBeenCalledExactlyOnceWith('a', -2)
    nudge.flush()
    expect(commit).toHaveBeenLastCalledWith('b', 1)
    vi.advanceTimersByTime(GANTT_KEY_DELAY_MS)
    expect(commit).toHaveBeenCalledTimes(2)
  })

  it('moves only the deadline from the keyboard', () => {
    const source = readFileSync('frontend/layers/dashboard/components/GanttEditor.vue', 'utf8')
    const onKey = source.slice(source.indexOf('function onKey'), source.indexOf('\n}', source.indexOf('function onKey')))
    expect(onKey).not.toMatch(/shiftKey|altKey|'move'|'start'/)
    expect(source).toContain("shift(task, 'end'")
  })
})

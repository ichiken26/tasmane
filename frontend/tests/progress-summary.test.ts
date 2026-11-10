import { describe, expect, it } from 'vitest'
import { progressSummary } from '../layers/core/composables/progressSummary'
import type { ApiConfirmation, ApiTask } from '../layers/core/composables/useCriticalData'

const task = (overrides: Partial<ApiTask>): ApiTask => ({
  id: 't', genreId: 'g', title: 'x', body: null, status: 'NOT_STARTED', progress: 0, isAllDay: true,
  effectivePriority: 'MEDIUM', isOverdue: false, showFire: false, version: 1, ...overrides,
})
const confirmation = (status: string): ApiConfirmation => ({ id: status, taskId: 't', taskTitle: 'x', content: 'c', targetName: 'n', status, dueAt: null })

describe('progressSummary', () => {
  it('フッター用に状態別件数・期限・確認待ち・全体進捗をまとめる', () => {
    const now = new Date('2026-11-11T12:00:00+09:00')
    const summary = progressSummary([
      task({ id: 'a', status: 'NOT_STARTED', progress: 0, endAt: '2026-11-11T14:59:59Z' }),
      task({ id: 'b', status: 'NOT_STARTED', progress: 0, showFire: true, isOverdue: true, endAt: '2026-11-09T14:59:59Z' }),
      task({ id: 'c', status: 'IN_PROGRESS', progress: 50, endAt: '2026-11-12T14:59:59Z' }),
      task({ id: 'd', status: 'COMPLETED', progress: 100, endAt: '2026-11-11T14:59:59Z' }),
    ], [confirmation('UNCONFIRMED'), confirmation('CHECKING'), confirmation('CONFIRMED')], now)
    expect(summary).toEqual({ total: 4, notStarted: 2, inProgress: 1, completed: 1, open: 3, overdue: 1, dueToday: 1, openConfirmations: 2, progress: 37 })
  })
  it('タスクが無ければすべて0', () => {
    expect(progressSummary([], [])).toEqual({ total: 0, notStarted: 0, inProgress: 0, completed: 0, open: 0, overdue: 0, dueToday: 0, openConfirmations: 0, progress: 0 })
  })
})

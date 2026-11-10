import type { ApiConfirmation, ApiTask } from './useCriticalData'
import { deadlineSection } from './taskState'

// 進捗概要（フッター等のひと目で分かる件数）
export interface ProgressSummary {
  total: number
  notStarted: number
  inProgress: number
  completed: number
  open: number
  overdue: number
  dueToday: number
  openConfirmations: number
  progress: number
}

// 全タスクと確認事項からの進捗概要の算出（期限区分はDashboardと同じ判定）
export function progressSummary(tasks: ApiTask[], confirmations: ApiConfirmation[], now = new Date()): ProgressSummary {
  const count = (status: string) => tasks.filter(task => task.status === status).length
  const open = tasks.filter(task => task.status !== 'COMPLETED')
  return {
    total: tasks.length,
    notStarted: count('NOT_STARTED'),
    inProgress: count('IN_PROGRESS'),
    completed: count('COMPLETED'),
    open: open.length,
    overdue: open.filter(task => task.showFire).length,
    dueToday: open.filter(task => task.endAt && deadlineSection(task.endAt.slice(0, 10), now) === 'today').length,
    openConfirmations: confirmations.filter(item => item.status !== 'CONFIRMED').length,
    progress: tasks.length ? Math.floor(tasks.reduce((sum, task) => sum + (task.progress ?? 0), 0) / tasks.length) : 0,
  }
}

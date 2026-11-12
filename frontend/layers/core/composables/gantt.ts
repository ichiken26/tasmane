import { dayDiff, startOfDay } from './dates'

export interface GanttDates { startAt?: string | null; endAt?: string | null }
export type GanttDragMode = 'move' | 'start' | 'end'

const DAY = 86_400_000

// 表示範囲（前3日・後7日の余白、最低28日）
export function ganttRange(tasks: GanttDates[], today = new Date()) {
  const stamps = tasks.flatMap(task => [task.startAt, task.endAt]).filter((value): value is string => Boolean(value)).map(value => startOfDay(new Date(value)).getTime())
  const base = startOfDay(today).getTime()
  const min = Math.min(base, ...stamps)
  const max = Math.max(base, ...stamps)
  const start = new Date(min)
  start.setDate(start.getDate() - 3)
  const end = new Date(max)
  end.setDate(end.getDate() + 7)
  return { start, days: Math.max(28, dayDiff(start, end) + 1) }
}

// バーの位置と長さ（日単位）
export function barGeometry(task: GanttDates, rangeStart: Date) {
  const startValue = task.startAt || task.endAt
  const endValue = task.endAt || task.startAt
  if (!startValue || !endValue) return undefined
  const offset = dayDiff(rangeStart, new Date(startValue))
  return { offset, span: Math.max(1, dayDiff(new Date(startValue), new Date(endValue)) + 1) }
}

const shift = (value: string | null | undefined, days: number) =>
  value ? new Date(new Date(value).getTime() + days * DAY).toISOString() : null

// ドラッグ後の日付（開始<=期限を維持）
export function dragDates(task: GanttDates, mode: GanttDragMode, days: number) {
  const startAt = task.startAt || null
  const endAt = task.endAt || null
  if (mode === 'move') return { startAt: shift(startAt, days), endAt: shift(endAt, days) }
  if (mode === 'start') {
    const next = shift(startAt ?? endAt, days)
    return { startAt: next && endAt && next > endAt ? endAt : next, endAt }
  }
  const next = shift(endAt ?? startAt, days)
  return { startAt, endAt: next && startAt && next < startAt ? startAt : next }
}

// キーボードでの期限変更は、最後の操作から少し待ってまとめて保存する（矢印ごとにAPIを呼ぶとロック競合が起きるため）
export const GANTT_KEY_DELAY_MS = 3000
export function createKeyNudge(commit: (id: string, days: number) => void, delayMs = GANTT_KEY_DELAY_MS) {
  let current: { id: string; days: number; timer: ReturnType<typeof setTimeout> } | undefined
  const flush = () => {
    if (!current) return
    const { id, days, timer } = current
    clearTimeout(timer)
    current = undefined
    if (days) commit(id, days)
  }
  return {
    press(id: string, step: number) {
      if (current && current.id !== id) flush()
      const days = (current?.days ?? 0) + step
      if (current) clearTimeout(current.timer)
      current = { id, days, timer: setTimeout(flush, delayMs) }
    },
    delta: (id: string) => current?.id === id ? current.days : 0,
    flush,
  }
}

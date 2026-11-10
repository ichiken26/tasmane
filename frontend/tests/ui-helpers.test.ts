import { describe, expect, it } from 'vitest'
import { apiErrorMessage, apiErrorStatus } from '../layers/core/composables/apiError'
import { fromInputValue, inputDatePart, relativeDeadline, toDateInput, toDateTimeInput, withPickedDate } from '../layers/core/composables/dates'
import { barGeometry, dragDates, ganttRange } from '../layers/core/composables/gantt'
import { confirmationMeta, priorityMeta, statusMeta } from '../layers/core/composables/labels'

const local = (value: string) => new Date(value)

describe('calendar picker', () => {
  // カレンダーで選んだ日付は時刻を保ったまま入力値へ反映する
  it('replaces only the date part and keeps the time', () => {
    expect(withPickedDate('2026-11-09T13:45', '2026-11-20', true)).toBe('2026-11-20T13:45')
    expect(withPickedDate('', '2026-11-20', true, '18:00')).toBe('2026-11-20T18:00')
    expect(withPickedDate('2026-11-09', '2026-11-20', false)).toBe('2026-11-20')
    expect(withPickedDate('2026-11-09T13:45', '2026-11-20', false)).toBe('2026-11-20')
  })
  // 入力値からカレンダー用の日付部分を取り出す（不正値は空）
  it('extracts a valid date part from the input value', () => {
    expect(inputDatePart('2026-11-09T13:45')).toBe('2026-11-09')
    expect(inputDatePart('2026-11-09')).toBe('2026-11-09')
    expect(inputDatePart('')).toBe('')
    expect(inputDatePart('2026-13-40')).toBe('')
  })
})

describe('date input conversion', () => {
  // 終日入力は開始=当日0時・期限=当日終わりとして送信
  it('converts all-day date inputs to the start and the end of the local day', () => {
    expect(new Date(fromInputValue('2026-11-09', true, 'start')!).getTime()).toBe(local('2026-11-09T00:00:00').getTime())
    expect(new Date(fromInputValue('2026-11-09', true, 'end')!).getTime()).toBe(local('2026-11-09T23:59:59.999').getTime())
    expect(fromInputValue('', true, 'end')).toBeNull()
  })

  // 時刻指定入力の往復
  it('round-trips datetime-local values', () => {
    const iso = fromInputValue('2026-11-09T13:45', false, 'start')!
    expect(toDateTimeInput(iso)).toBe('2026-11-09T13:45')
    expect(toDateInput(iso)).toBe('2026-11-09')
    expect(toDateInput(null)).toBe('')
  })

  // 期限の相対表示
  it('describes deadlines relative to today', () => {
    const now = local('2026-11-09T12:00:00')
    expect(relativeDeadline(local('2026-11-09T23:00:00').toISOString(), now)).toBe('今日')
    expect(relativeDeadline(local('2026-11-10T09:00:00').toISOString(), now)).toBe('明日')
    expect(relativeDeadline(local('2026-11-06T09:00:00').toISOString(), now)).toBe('3日超過')
    expect(relativeDeadline(local('2026-11-14T09:00:00').toISOString(), now)).toBe('あと5日')
  })
})

describe('gantt geometry', () => {
  const today = local('2026-11-09T10:00:00')

  // 表示範囲は余白付きで最低4週間
  it('pads the visible range around task dates with a 28-day minimum', () => {
    const range = ganttRange([{ startAt: local('2026-11-10T00:00:00').toISOString(), endAt: local('2026-11-12T00:00:00').toISOString() }], today)
    expect(range.start.getTime()).toBe(local('2026-11-06T00:00:00').getTime())
    expect(range.days).toBeGreaterThanOrEqual(28)
  })

  // 片側のみの日付は1日バーとして配置
  it('places bars by day offset and treats a single date as a one-day bar', () => {
    const start = local('2026-11-06T00:00:00')
    expect(barGeometry({ startAt: local('2026-11-08T09:00:00').toISOString(), endAt: local('2026-11-10T18:00:00').toISOString() }, start)).toEqual({ offset: 2, span: 3 })
    expect(barGeometry({ startAt: null, endAt: local('2026-11-07T18:00:00').toISOString() }, start)).toEqual({ offset: 1, span: 1 })
    expect(barGeometry({ startAt: null, endAt: null }, start)).toBeUndefined()
  })

  // 移動は両端を同量、リサイズは片端のみで前後関係を維持
  it('moves both edges and resizes a single edge without inverting the range', () => {
    const task = { startAt: '2026-11-08T00:00:00.000Z', endAt: '2026-11-10T00:00:00.000Z' }
    expect(dragDates(task, 'move', 2)).toEqual({ startAt: '2026-11-10T00:00:00.000Z', endAt: '2026-11-12T00:00:00.000Z' })
    expect(dragDates(task, 'start', 1)).toEqual({ startAt: '2026-11-09T00:00:00.000Z', endAt: task.endAt })
    expect(dragDates(task, 'end', -1)).toEqual({ startAt: task.startAt, endAt: '2026-11-09T00:00:00.000Z' })
    expect(dragDates(task, 'start', 5)).toEqual({ startAt: task.endAt, endAt: task.endAt })
    expect(dragDates(task, 'end', -5)).toEqual({ startAt: task.startAt, endAt: task.startAt })
  })
})

describe('api error messages', () => {
  // ステータスとコードに応じた日本語メッセージ
  it('maps API failures to actionable Japanese messages', () => {
    expect(apiErrorStatus({ statusCode: 401 })).toBe(401)
    expect(apiErrorMessage({ statusCode: 401 }, 'x')).toContain('ログイン')
    expect(apiErrorMessage({ statusCode: 409, data: { code: 'CONFLICT' } }, 'x')).toContain('最新')
    expect(apiErrorMessage({ statusCode: 409, data: { code: 'IMPACT_TOKEN_INVALID' } }, 'x')).toContain('もう一度')
    expect(apiErrorMessage({ statusCode: 404 }, 'x')).toContain('見つかりません')
    expect(apiErrorMessage(new Error('network'), '保存できませんでした。')).toBe('保存できませんでした。')
  })
})

describe('display labels', () => {
  // 列挙値の表示名と色
  it('labels statuses and priorities in Japanese with semantic colors', () => {
    expect(statusMeta('IN_PROGRESS')).toMatchObject({ label: '進行中' })
    expect(priorityMeta('HIGH')).toMatchObject({ label: '高', color: 'error' })
    expect(confirmationMeta('CONFIRMED')).toMatchObject({ label: '確認済み', color: 'success' })
    expect(statusMeta('UNKNOWN').label).toBe('UNKNOWN')
  })
})

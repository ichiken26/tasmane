const pad = (value: number) => String(value).padStart(2, '0')
const DAY = 86_400_000

// ローカル日付の0時
export const startOfDay = (value: Date) => new Date(value.getFullYear(), value.getMonth(), value.getDate())

// 日数の差（ローカル日付基準）
export const dayDiff = (from: Date, to: Date) => Math.round((startOfDay(to).getTime() - startOfDay(from).getTime()) / DAY)

// date入力値への変換
export function toDateInput(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

// datetime-local入力値への変換
export function toDateTimeInput(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  return `${toDateInput(value)}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// 入力値からISO文字列への変換（終日は開始=0時、期限=当日終わり）
export function fromInputValue(value: string, isAllDay: boolean, edge: 'start' | 'end'): string | null {
  if (!value) return null
  if (isAllDay) {
    const [year, month, day] = value.slice(0, 10).split('-').map(Number)
    const date = edge === 'start'
      ? new Date(year!, month! - 1, day!, 0, 0, 0, 0)
      : new Date(year!, month! - 1, day!, 23, 59, 59, 999)
    return date.toISOString()
  }
  return new Date(value).toISOString()
}

// 入力値の日付部分（YYYY-MM-DD。不正値は空）
export function inputDatePart(value: string): string {
  const part = value.slice(0, 10)
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(part)
  if (!match) return ''
  const [year, month, day] = match.slice(1).map(Number)
  const date = new Date(year!, month! - 1, day!)
  return date.getFullYear() === year && date.getMonth() === month! - 1 && date.getDate() === day ? part : ''
}

// カレンダーで選んだ日付を入力値へ反映（時刻ありなら既存の時刻を保つ）
export function withPickedDate(value: string, picked: string, withTime: boolean, defaultTime = '09:00'): string {
  if (!withTime) return picked
  const time = /T(\d{2}:\d{2})/.exec(value)?.[1] ?? defaultTime
  return `${picked}T${time}`
}

const weekdays = ['日', '月', '火', '水', '木', '金', '土']
// 日付の表示（例: 11/9(月) 13:45）
export function formatDate(value?: string | null, withTime = false) {
  if (!value) return '—'
  const date = new Date(value)
  const base = `${date.getMonth() + 1}/${date.getDate()}(${weekdays[date.getDay()]})`
  return withTime ? `${base} ${pad(date.getHours())}:${pad(date.getMinutes())}` : base
}

// 期限の相対表示
export function relativeDeadline(value: string, now = new Date()) {
  const diff = dayDiff(now, new Date(value))
  if (diff === 0) return '今日'
  if (diff === 1) return '明日'
  if (diff < 0) return `${-diff}日超過`
  return `あと${diff}日`
}

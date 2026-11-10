export interface Versioned { id: string; version: number }
export interface SearchState { q: string; genres: string[]; priority: string; deadline: string; sort: string }
const SEARCH_SORTS = ['taskScore', 'priority', 'deadline']
export type DeadlineSection = 'overdue'|'today'|'tomorrow'|'week'|'month'|'later'

// 新しいversionのスナップショットのみ受理
export const acceptSnapshot = (current: Versioned | undefined, incoming: Versioned) =>
  !current || current.id !== incoming.id || incoming.version > current.version

const day = (value: Date) => new Date(value.getFullYear(), value.getMonth(), value.getDate()).getTime()
// 期限日の区分判定
export function deadlineSection(value: string, now = new Date()): DeadlineSection {
  const diff = Math.round((day(new Date(`${value}T00:00:00`)) - day(now)) / 86_400_000)
  if (diff < 0) return 'overdue'
  if (diff === 0) return 'today'
  if (diff === 1) return 'tomorrow'
  if (diff <= 7) return 'week'
  if (diff <= 30) return 'month'
  return 'later'
}
// 検索条件のクエリ復元
export function parseSearch(query: Record<string, unknown>): SearchState {
  const text = (key: string) => typeof query[key] === 'string' ? query[key] as string : ''
  return { q: text('q'), genres: text('genres').split(',').filter(Boolean), priority: text('priority'), deadline: text('deadline'), sort: SEARCH_SORTS.includes(text('sort')) ? text('sort') : '' }
}
// 検索条件のクエリ化
export function encodeSearch(state: SearchState): Record<string, string> {
  return Object.fromEntries(Object.entries({ q: state.q, genres: state.genres.join(','), priority: state.priority, deadline: state.deadline, sort: state.sort }).filter(([, value]) => value))
}
// 画面の検索条件からOpenAPI searchTasksクエリ（genreIds, Priority enum）への変換
export function toSearchApiQuery(state: SearchState): Record<string, string> {
  return Object.fromEntries(Object.entries({ q: state.q, genreIds: state.genres.join(','), priority: state.priority.toUpperCase(), deadline: state.deadline, sort: state.sort }).filter(([, value]) => value))
}

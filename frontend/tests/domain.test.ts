import { describe, expect, it } from 'vitest'
import { schedulePrefetch } from '../layers/core/composables/criticalOrder'
import { acceptSnapshot, deadlineSection, encodeSearch, parseSearch, toSearchApiQuery } from '../layers/core/composables/taskState'
import { applyEntityMessage, parseEntityMessage } from '../layers/core/composables/useEntityChannel'

describe('frontend domain-facing state', () => {
  // 新しいエンティティを古いスナップショットで上書きしないことの確認
  it('never replaces a newer entity with an old BroadcastChannel snapshot', () => {
    expect(acceptSnapshot({ id: 'a', version: 4 }, { id: 'a', version: 3 })).toBe(false)
    expect(acceptSnapshot({ id: 'a', version: 4 }, { id: 'a', version: 5 })).toBe(true)
  })

  // 期限日の区分判定
  it.each([
    ['2026-08-11', 'overdue'], ['2026-08-12', 'today'], ['2026-08-13', 'tomorrow'],
    ['2026-08-19', 'week'], ['2026-09-11', 'month'], ['2026-10-01', 'later'],
  ])('classifies %s as %s', (date, section) => {
    expect(deadlineSection(date, new Date('2026-08-12T12:00:00+09:00'))).toBe(section)
  })

  // 複数ジャンルOR検索条件のURL往復
  it('round-trips multi-genre OR search state through URL query values', () => {
    const state = parseSearch({ q: 'google', genres: 'dev,infra', priority: 'high', deadline: 'week' })
    expect(state.genres).toEqual(['dev', 'infra'])
    expect(encodeSearch(state)).toEqual({ q: 'google', genres: 'dev,infra', priority: 'high', deadline: 'week' })
  })

  // 画面URL（genres）からOpenAPIのsearchTasksクエリ（genreIds, Priority enum）への変換
  it('maps search state to the OpenAPI searchTasks query parameters', () => {
    const state = parseSearch({ q: 'google', genres: 'dev,infra', priority: 'high', deadline: 'overdue' })
    expect(toSearchApiQuery(state)).toEqual({ q: 'google', genreIds: 'dev,infra', priority: 'HIGH', deadline: 'overdue' })
    expect(toSearchApiQuery(parseSearch({}))).toEqual({})
  })

  // 並び順（OpenAPI sort）もURLと同期し、未知の値は捨てる
  it('round-trips the search sort order and drops unknown values', () => {
    const state = parseSearch({ sort: 'priority' })
    expect(encodeSearch(state)).toEqual({ sort: 'priority' })
    expect(toSearchApiQuery(state)).toEqual({ sort: 'priority' })
    expect(parseSearch({ sort: 'title' }).sort).toBe('')
  })
})

describe('P1 BroadcastChannel message safety', () => {
  const task = { id: '018f47a0-7b6c-7f6e-8a1b-9c0d1e2f3a4b', version: 4, title: 'current' }

  // スキーマ準拠スナップショットの受理とpayload適用
  it('accepts a schema-conformant snapshot and applies its payload', () => {
    const message = parseEntityMessage({
      type: 'ENTITY_SNAPSHOT',
      entity: { kind: 'task', id: task.id, version: 5, payload: { ...task, version: 5, title: 'new' } },
      sentAt: '2026-08-12T03:00:00.000Z',
    })
    expect(message).toBeDefined()
    expect(applyEntityMessage('task', task, message)).toEqual({ ...task, version: 5, title: 'new' })
  })

  // 一致かつ非staleなENTITY_INVALIDATEDのみ適用
  it('applies ENTITY_INVALIDATED only for a matching non-stale entity', () => {
    const matching = parseEntityMessage({ type: 'ENTITY_INVALIDATED', entityKind: 'task', entityId: task.id, version: 4, sentAt: '2026-08-12T03:00:00Z' })
    const stale = parseEntityMessage({ type: 'ENTITY_INVALIDATED', entityKind: 'task', entityId: task.id, version: 3, sentAt: '2026-08-12T03:00:00Z' })
    expect(applyEntityMessage('task', task, matching)).toBeUndefined()
    expect(applyEntityMessage('task', task, stale)).toEqual(task)
  })

  // 別kindのスナップショットと無効化の無視
  it('ignores snapshots and invalidations for a different entity kind', () => {
    const snapshot = parseEntityMessage({
      type: 'ENTITY_SNAPSHOT',
      entity: { kind: 'genre', id: task.id, version: 5, payload: { ...task, version: 5, title: 'wrong kind' } },
      sentAt: '2026-08-12T03:00:00Z',
    })
    const invalidation = parseEntityMessage({ type: 'ENTITY_INVALIDATED', entityKind: 'genre', entityId: task.id, version: 5, sentAt: '2026-08-12T03:00:00Z' })
    expect(applyEntityMessage('task', task, snapshot)).toEqual(task)
    expect(applyEntityMessage('task', task, invalidation)).toEqual(task)
  })

  // 不正メッセージの棄却
  it.each([
    null,
    { type: 'ENTITY_SNAPSHOT', entity: { kind: 'task', id: 'not-a-uuid', version: 5, payload: {} }, sentAt: '2026-08-12T03:00:00Z' },
    { type: 'ENTITY_SNAPSHOT', entity: { kind: 'task', id: task.id, version: 0, payload: {} }, sentAt: '2026-08-12T03:00:00Z' },
    { type: 'ENTITY_SNAPSHOT', entity: { kind: 'task', id: task.id, version: 5, payload: {}, extra: true }, sentAt: '2026-08-12T03:00:00Z' },
    { type: 'ENTITY_INVALIDATED', entityKind: 'task', entityId: task.id, version: 5, sentAt: '2026-02-30T03:00:00Z' },
    { type: 'ENTITY_INVALIDATED', entityKind: 'task', entityId: task.id, version: 5, sentAt: 'yesterday', extra: true },
    { type: 'UNKNOWN', entityId: task.id, version: 5, sentAt: '2026-08-12T03:00:00Z' },
  ])('rejects invalid data without changing current state', (data) => {
    const message = parseEntityMessage(data)
    expect(message).toBeUndefined()
    expect(applyEntityMessage('task', task, message)).toEqual(task)
  })
})

describe('critical fetch ordering', () => {
  // 描画tick後のプリフェッチ予約確認
  it('schedules prefetch only after the render tick', async () => {
    const events: string[] = ['critical', 'render']
    await schedulePrefetch(async () => { events.push('prefetch') })
    expect(events).toEqual(['critical', 'render', 'prefetch'])
  })
})

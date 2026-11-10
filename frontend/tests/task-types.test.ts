import { describe, expect, it, vi } from 'vitest'
import { ensureGenreTaskType } from '../layers/core/composables/taskTypes'

const api = (all: { id: string; name: string }[]) => ({
  allTaskTypes: vi.fn().mockResolvedValue(all),
  createTaskType: vi.fn().mockImplementation(async (name: string) => ({ id: 'new', name })),
  attachTaskType: vi.fn().mockResolvedValue(undefined),
})

describe('task type creation from the task editor', () => {
  // 新しい名前なら作成してジャンルへ紐付ける
  it('creates and attaches a new task type', async () => {
    const client = api([])
    expect(await ensureGenreTaskType(client, 'g1', '  設計 ')).toEqual({ id: 'new', name: '設計' })
    expect(client.createTaskType).toHaveBeenCalledWith('設計')
    expect(client.attachTaskType).toHaveBeenCalledWith('g1', 'new')
  })
  // 同名（大文字小文字無視）の定義があれば再作成せず紐付けだけ行う
  it('reuses an existing user-wide task type case-insensitively', async () => {
    const client = api([{ id: 't1', name: 'Review' }])
    expect(await ensureGenreTaskType(client, 'g1', 'review')).toEqual({ id: 't1', name: 'Review' })
    expect(client.createTaskType).not.toHaveBeenCalled()
    expect(client.attachTaskType).toHaveBeenCalledWith('g1', 't1')
  })
  // 空の名前は拒否
  it('rejects an empty name', async () => {
    const client = api([])
    await expect(ensureGenreTaskType(client, 'g1', '   ')).rejects.toThrow()
    expect(client.attachTaskType).not.toHaveBeenCalled()
  })
})

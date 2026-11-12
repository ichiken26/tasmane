import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// 入力欄は初期値を入れず、未指定時の意味はプレースホルダーで示す
describe('inputs start empty', () => {
  it('task editor selects start empty and show the unset meaning as a placeholder', () => {
    const source = readFileSync('frontend/layers/task/components/TaskEditor.vue', 'utf8')
    expect(source).toMatch(/parentTaskId: task\?\.parentTaskId \?\? props\.defaults\.parentTaskId \?\? ''/)
    expect(source).toMatch(/taskTypeId: task\?\.taskTypeId \?\? ''/)
    expect(source).toMatch(/priority: task\?\.manualPriority \?\? ''/)
    expect(source).toMatch(/v-model="form\.parentTaskId"[^>]*placeholder="なし（Root Task）"/)
    expect(source).toMatch(/v-model="form\.priority"[^>]*placeholder="自動/)
  })

  it('search filters start empty', () => {
    const source = readFileSync('pages/search.vue', 'utf8')
    expect(source).toMatch(/get: \(\) => state\.priority \|\| undefined/)
    expect(source).toMatch(/get: \(\) => state\.deadline \|\| undefined/)
    expect(source).toMatch(/get: \(\) => state\.sort \|\| undefined/)
  })
})

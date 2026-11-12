import { describe, expect, it } from 'vitest'
import { deletionImpact } from '../layers/core/composables/impact'
import type { ImpactPlan } from '../layers/core/composables/useCriticalData'

const plan = (ids: string[]): ImpactPlan => ({
  code: 'IMPACT_CONFIRMATION_REQUIRED',
  impactToken: 'tok',
  affectedEntities: ids.map(id => ({ id, type: 'TASK', title: id, reason: 'SOFT_DELETE' })),
  warnings: ['これらのタスクも削除されます。よろしいですか？'],
})

describe('deletion impact', () => {
  // 削除しようとしているタスク自身は「これらのタスクも」に含めない
  it('lists only the other tasks deleted together', () => {
    const result = deletionImpact(plan(['target', 'child', 'dependent']), 'target')
    expect(result.together.map(item => item.id)).toEqual(['child', 'dependent'])
    expect(result.needsConfirmation).toBe(true)
  })

  // 巻き込むタスクがなければ2段階目の確認は不要（1段階目で確定済み）
  it('needs no second confirmation when only the target is deleted', () => {
    const result = deletionImpact(plan(['target']), 'target')
    expect(result.together).toEqual([])
    expect(result.needsConfirmation).toBe(false)
  })

  // 警告があれば2段階目で必ず見せる
  it('still confirms when the server adds a warning beyond the default', () => {
    const value = plan(['target'])
    value.warnings.push('同期中のGoogle Tasksからも削除されます。')
    expect(deletionImpact(value, 'target').needsConfirmation).toBe(true)
  })
})

import type { FetchError } from 'ofetch'
import type { ImpactPlan } from './useCriticalData'

// 影響確認エラーからの計画抽出
export function impactPlanFrom(error: unknown): ImpactPlan | undefined {
  const data = (error as FetchError<ImpactPlan>)?.data
  return data?.code === 'IMPACT_CONFIRMATION_REQUIRED' ? data : undefined
}

// 影響内容の表示行生成
export function impactLines(plan: ImpactPlan | undefined): string[] {
  if (!plan) return []
  const entities = plan.affectedEntities.map(item => `${item.title || item.id}: ${item.reason || item.type}`)
  return [...plan.warnings, ...entities]
}

// 影響理由コードの表示名
const reasonLabels: Record<string, string> = {
  DATE_PROPAGATION: '日付が変わります', GENRE_MOVE: 'ジャンルを移動します', GENRE_BOUNDARY_REMOVAL: '依存関係が解除されます',
  LEAF_TO_PARENT: '親タスクになり進捗は子から自動計算されます', SOFT_DELETE: '削除されます（30日以内は復元可）',
  STATUS_ROLLBACK: '状態が戻ります', ANCESTOR_REOPEN: '上位タスクが進行中に戻ります', DESCENDANT_RESET: '子タスクが未着手に戻ります',
  UNMET_DEPENDENCY: '未完了の前提タスクです',
}
export function impactReasonLabel(reason?: string | null): string | undefined {
  if (!reason) return undefined
  return reasonLabels[reason] ?? reason
}

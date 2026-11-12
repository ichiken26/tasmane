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

// 削除確認の既定警告（APIが常に付ける文言。モーダルのタイトルと重複するため本文には出さない）
export const DELETE_TOGETHER_WARNING = 'これらのタスクも削除されます。よろしいですか？'

// 削除の影響表示（操作対象自身を除いた「一緒に削除されるタスク」と追加の警告）
export function deletionImpact(plan: ImpactPlan, targetId: string) {
  const together = plan.affectedEntities.filter(entity => entity.id !== targetId)
  const warnings = plan.warnings.filter(warning => warning !== DELETE_TOGETHER_WARNING)
  return { together, warnings, needsConfirmation: together.length > 0 || warnings.length > 0 }
}

export type SemanticColor = 'primary' | 'secondary' | 'success' | 'info' | 'warning' | 'error' | 'neutral'
export interface EnumMeta { label: string; color: SemanticColor; icon: string }

const statuses: Record<string, EnumMeta> = {
  NOT_STARTED: { label: '未着手', color: 'neutral', icon: 'i-lucide-circle' },
  IN_PROGRESS: { label: '進行中', color: 'info', icon: 'i-lucide-circle-dot' },
  COMPLETED: { label: '完了', color: 'success', icon: 'i-lucide-circle-check' },
}
const priorities: Record<string, EnumMeta> = {
  HIGH: { label: '高', color: 'error', icon: 'i-lucide-chevrons-up' },
  MEDIUM: { label: '中', color: 'warning', icon: 'i-lucide-equal' },
  LOW: { label: '低', color: 'neutral', icon: 'i-lucide-chevrons-down' },
}
const confirmations: Record<string, EnumMeta> = {
  UNCONFIRMED: { label: '未確認', color: 'warning', icon: 'i-lucide-circle-help' },
  CHECKING: { label: '確認中', color: 'info', icon: 'i-lucide-message-circle-more' },
  CONFIRMED: { label: '確認済み', color: 'success', icon: 'i-lucide-badge-check' },
}
const unknown = (value: string): EnumMeta => ({ label: value, color: 'neutral', icon: 'i-lucide-circle' })

// タスク状態の表示情報
export const statusMeta = (value: string) => statuses[value] ?? unknown(value)
// 優先度の表示情報
export const priorityMeta = (value: string) => priorities[value] ?? unknown(value)
// 確認事項状態の表示情報
export const confirmationMeta = (value: string) => confirmations[value] ?? unknown(value)

export const priorityOptions = [
  { label: '高', value: 'HIGH' },
  { label: '中', value: 'MEDIUM' },
  { label: '低', value: 'LOW' },
]

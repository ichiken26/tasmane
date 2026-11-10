import { useState } from '#app'
import type { ConfirmationTarget } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'

// 確認先プリセットの共有状態（設定画面と確認事項入力で同期）
export function useConfirmationTargets() {
  const api = useTasmaneApi()
  const targets = useState<ConfirmationTarget[]>('confirmation-targets', () => [])
  const loaded = useState<boolean>('confirmation-targets-loaded', () => false)
  const sort = () => targets.value.sort((a, b) => a.name.localeCompare(b.name, 'ja'))
  // 一覧の読込（取得済みなら再利用）
  const load = async (force = false) => {
    if (loaded.value && !force) return
    targets.value = await api.confirmationTargets()
    loaded.value = true
  }
  // 登録
  const create = async (name: string) => {
    const target = await api.createConfirmationTarget(name.trim())
    targets.value.push(target)
    sort()
    return target
  }
  // 削除
  const remove = async (id: string) => {
    await api.deleteConfirmationTarget(id)
    targets.value = targets.value.filter(target => target.id !== id)
  }
  return { targets, loaded, load, create, remove }
}

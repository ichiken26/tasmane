<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DropdownMenuItem } from '@nuxt/ui'
import { useRouter } from '#imports'
import type { ApiTask, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { impactPlanFrom } from '../../core/composables/impact'
import { apiErrorStatus } from '../../core/composables/apiError'
import { useNotify } from '../../shared/composables/useNotify'

// タスクの状態遷移と削除（削除は2段階確認）
const props = defineProps<{ task: ApiTask }>()
const emit = defineEmits<{ updated: [task: ApiTask] }>()
const api = useTasmaneApi()
const router = useRouter()
const notify = useNotify()
const impact = ref<ImpactPlan>()
const pending = ref<{ kind: 'transition'; status: string } | { kind: 'delete' }>()
const busy = ref(false)
const confirmDelete = ref(false)

// 遷移可能な状態一覧
const transitions = computed(() => {
  if (props.task.status === 'NOT_STARTED') return ['IN_PROGRESS']
  if (props.task.status === 'IN_PROGRESS') return ['COMPLETED', 'NOT_STARTED']
  return ['IN_PROGRESS']
})
// 状態遷移ボタンの表示
const actionMeta = (status: string) => ({
  NOT_STARTED: { label: '未着手へ戻す', icon: 'i-lucide-undo-2' },
  IN_PROGRESS: { label: props.task.status === 'COMPLETED' ? '再開する' : '着手する', icon: props.task.status === 'COMPLETED' ? 'i-lucide-rotate-ccw' : 'i-lucide-play' },
  COMPLETED: { label: '完了する', icon: 'i-lucide-check' },
} as Record<string, { label: string; icon: string }>)[status] ?? { label: status, icon: 'i-lucide-circle' }
const primary = computed(() => transitions.value[0])
const menuItems = computed<DropdownMenuItem[][]>(() => {
  const others: DropdownMenuItem[] = transitions.value.slice(1).map(status => ({ ...actionMeta(status), onSelect: () => { void transition(status) } }))
  const danger: DropdownMenuItem[] = [{ label: 'タスクを削除', icon: 'i-lucide-trash-2', color: 'error', onSelect: () => { confirmDelete.value = true } }]
  return others.length ? [others, danger] : [danger]
})

// タスク状態の遷移
async function transition(status: string, token?: string) {
  busy.value = true
  try {
    const saved = await api.transitionTask(props.task.id, status, props.task.version, token)
    impact.value = undefined
    pending.value = undefined
    notify.success(`「${actionMeta(status).label.replace(/する$/, '')}」に変更しました`)
    emit('updated', saved)
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) {
      impact.value = plan
      pending.value = { kind: 'transition', status }
      return
    }
    impact.value = undefined
    if (status === 'COMPLETED' && apiErrorStatus(error) === 422) notify.error(error, '未完了の子タスクや未確認の確認事項があるため完了できません。')
    else notify.error(error, '状態を変更できませんでした。')
  } finally {
    busy.value = false
  }
}

// タスク削除
async function remove(token?: string) {
  busy.value = true
  try {
    await api.deleteTask(props.task.id, props.task.version, token)
    confirmDelete.value = false
    impact.value = undefined
    notify.success('タスクを削除しました', '30日後に完全に削除されます。')
    await router.push(`/dashboard/genres/${props.task.genreId}`)
  } catch (error) {
    const plan = impactPlanFrom(error)
    confirmDelete.value = false
    if (plan) {
      impact.value = plan
      pending.value = { kind: 'delete' }
      return
    }
    notify.error(error, '削除できませんでした。')
  } finally {
    busy.value = false
  }
}

// 影響確認後の操作確定
async function confirmImpact() {
  if (pending.value?.kind === 'transition') await transition(pending.value.status, impact.value?.impactToken)
  else if (pending.value?.kind === 'delete') await remove(impact.value?.impactToken)
}
</script>

<template>
  <!-- 状態操作 -->
  <div class="flex items-center gap-2">
    <UButton v-if="primary" :icon="actionMeta(primary).icon" :loading="busy && pending?.kind !== 'delete'" :color="primary === 'COMPLETED' ? 'success' : 'primary'" @click="transition(primary)">
      {{ actionMeta(primary).label }}
    </UButton>
    <UDropdownMenu :items="menuItems" :content="{ align: 'end' }">
      <UButton icon="i-lucide-ellipsis-vertical" color="neutral" variant="outline" aria-label="その他の操作" />
    </UDropdownMenu>

    <!-- 削除の1段階目 -->
    <ConfirmDialog v-model:open="confirmDelete" title="このタスクを削除しますか？" description="子タスクがある場合は子孫も含めて削除されます。削除後30日で完全に削除されます。" confirm-label="削除する" :loading="busy" @confirm="remove()">
      <p class="rounded-lg bg-(--ui-bg-muted) px-3 py-2 font-medium text-(--ui-text-highlighted)">{{ task.title }}</p>
    </ConfirmDialog>
    <!-- 操作影響の確認（削除の2段階目・状態伝播） -->
    <ImpactModal
      :open="Boolean(impact)"
      :title="pending?.kind === 'delete' ? 'これらのタスクも削除されます。よろしいですか？' : '操作の影響を確認'"
      :plan="impact"
      :loading="busy"
      :danger="pending?.kind === 'delete'"
      :confirm-label="pending?.kind === 'delete' ? 'すべて削除する' : '承認して続行'"
      @cancel="impact = undefined; pending = undefined"
      @confirm="confirmImpact"
    />
  </div>
</template>

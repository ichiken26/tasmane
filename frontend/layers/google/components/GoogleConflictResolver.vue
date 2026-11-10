<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ApiTask, GoogleConflict, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { impactPlanFrom } from '../../core/composables/impact'
import { useNotify } from '../../shared/composables/useNotify'

// タスク詳細で Google 側の値を採用してコンフリクトを解決（通常の更新・影響確認を通す）
const props = defineProps<{ task: ApiTask }>()
const emit = defineEmits<{ resolved: [task: ApiTask] }>()
const api = useTasmaneApi()
const notify = useNotify()
const conflict = ref<GoogleConflict>()
const busy = ref(false)
const impact = ref<ImpactPlan>()

onMounted(async () => {
  try { conflict.value = (await api.googleConflicts()).find(item => item.taskId === props.task.id) }
  catch { conflict.value = undefined }
})
const remote = computed(() => conflict.value?.remote ?? {})

// Google の期限日（日付のみ）をタスマネの期限へ。既存の時刻は保持
function remoteEndAt(): string | null | undefined {
  const due = remote.value.due
  if (due === undefined) return undefined
  if (!due) return null
  if (props.task.endAt && !props.task.isAllDay) {
    const current = new Date(props.task.endAt)
    const [year, month, day] = due.split('-').map(Number) as [number, number, number]
    current.setFullYear(year, month - 1, day)
    return current.toISOString()
  }
  const [year, month, day] = due.split('-').map(Number) as [number, number, number]
  return new Date(year, month - 1, day, 23, 59, 59, 999).toISOString()
}

// Google の値で更新 → タスマネの値として Google へ反映（KEEP_TASMANE）
async function adopt(token?: string) {
  busy.value = true
  try {
    const endAt = remoteEndAt()
    const saved = await api.updateTask(props.task.id, {
      expectedVersion: props.task.version,
      ...(remote.value.title !== undefined ? { title: remote.value.title || props.task.title } : {}),
      ...(remote.value.notes !== undefined ? { body: remote.value.notes || null } : {}),
      ...(endAt !== undefined ? { endAt } : {}),
      ...(token ? { impactToken: token } : {}),
    })
    impact.value = undefined
    await api.resolveGoogleConflict(props.task.id, 'KEEP_TASMANE')
    conflict.value = undefined
    notify.success('Googleの値を反映しました', '完了状態は必要に応じて操作してください。')
    emit('resolved', saved)
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) { impact.value = plan; return }
    notify.error(error, 'Googleの値を反映できませんでした。')
  } finally { busy.value = false }
}
</script>

<template>
  <UAlert v-if="conflict && conflict.code !== 'GOOGLE_DELETED'" color="warning" variant="subtle" icon="i-lucide-git-compare" title="Google Tasks との同期コンフリクト" class="mb-6">
    <template #description>
      <dl class="mt-1 grid grid-cols-[5rem_1fr] gap-x-3 gap-y-0.5 text-xs">
        <template v-if="remote.title !== undefined"><dt>タイトル</dt><dd>{{ remote.title || '—' }}</dd></template>
        <template v-if="remote.notes !== undefined"><dt>メモ</dt><dd class="whitespace-pre-wrap">{{ remote.notes || '—' }}</dd></template>
        <template v-if="remote.due !== undefined"><dt>期限</dt><dd>{{ remote.due || 'なし' }}</dd></template>
        <template v-if="remote.status"><dt>状態</dt><dd>{{ remote.status === 'completed' ? '完了' : '未完了' }}</dd></template>
      </dl>
    </template>
    <template #actions>
      <UButton size="xs" icon="i-lucide-download" :loading="busy" @click="adopt()">Googleの値を採用</UButton>
      <UButton size="xs" color="neutral" variant="soft" to="/settings">設定でほかの解決方法を選ぶ</UButton>
    </template>
  </UAlert>
  <ImpactModal :open="Boolean(impact)" title="Googleの値を反映すると次の変更も行われます" :plan="impact" :loading="busy" confirm-label="承認して反映" @cancel="impact = undefined" @confirm="adopt(impact?.impactToken)" />
</template>

<script setup lang="ts">
import type { ApiTask, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { impactLines, impactPlanFrom } from '../../core/composables/impact'

defineProps<{ tasks: ApiTask[] }>()
const emit = defineEmits<{ updated: [task: ApiTask] }>()
const api = useTasmaneApi()
const impact = ref<ImpactPlan>()
const pending = ref<{ task: ApiTask; startAt: string | null; endAt: string | null }>()
const errorMessage = ref('')
let drag: { task: ApiTask; mode: 'move' | 'start' | 'end'; x: number } | undefined

function begin(event: { clientX: number; pointerId: number; currentTarget: unknown }, task: ApiTask, mode: 'move' | 'start' | 'end') {
  drag = { task, mode, x: event.clientX }
  ;(event.currentTarget as { setPointerCapture: (id: number) => void }).setPointerCapture(event.pointerId)
}

async function finish(event: { clientX: number }) {
  if (!drag) return
  const days = Math.round((event.clientX - drag.x) / 32)
  const { task, mode } = drag
  drag = undefined
  if (days) await shift(task, mode, days)
}

const shifted = (value: string | null | undefined, days: number) =>
  value ? new Date(new Date(value).getTime() + days * 86_400_000).toISOString() : null

async function shift(task: ApiTask, mode: 'move' | 'start' | 'end', days: number, token?: string) {
  const change = {
    task,
    startAt: mode === 'end' ? task.startAt || null : shifted(task.startAt, days),
    endAt: mode === 'start' ? task.endAt || null : shifted(task.endAt, days),
  }
  pending.value = change
  try {
    const saved = await api.updateTask(task.id, {
      expectedVersion: task.version,
      startAt: change.startAt,
      endAt: change.endAt,
      ...(token ? { impactToken: token } : {}),
    })
    impact.value = undefined
    pending.value = undefined
    errorMessage.value = ''
    emit('updated', saved)
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) impact.value = plan
    else errorMessage.value = 'Ganttの日付を更新できませんでした。'
  }
}

async function confirm() {
  if (!impact.value || !pending.value) return
  const { task, startAt, endAt } = pending.value
  try {
    const saved = await api.updateTask(task.id, {
      expectedVersion: task.version,
      startAt,
      endAt,
      impactToken: impact.value.impactToken,
    })
    impact.value = undefined
    pending.value = undefined
    emit('updated', saved)
  } catch {
    errorMessage.value = '状態が変わったため、もう一度操作してください。'
    impact.value = undefined
  }
}
</script>

<template>
  <section class="card">
    <h2>Gantt</h2>
    <p class="muted">バー: 移動 / 左端: 開始 / 右端: 期限。矢印キーは1日単位。</p>
    <p v-if="errorMessage" role="alert">{{ errorMessage }}</p>
    <div class="gantt">
      <div v-for="task in tasks.filter(item => item.startAt || item.endAt)" :key="task.id" class="gantt-row">
        <span>{{ task.title }}</span>
        <div
          class="gantt-bar"
          tabindex="0"
          role="slider"
          :aria-label="`${task.title}の日程`"
          @pointerdown="begin($event, task, 'move')"
          @pointerup="finish"
        >
          <button type="button" aria-label="開始日を1日前" @pointerdown.stop="begin($event, task, 'start')" @pointerup.stop="finish" @keydown.left.prevent="shift(task, 'start', -1)" @keydown.right.prevent="shift(task, 'start', 1)">←</button>
          <span>{{ task.startAt?.slice(0, 10) || '未設定' }} — {{ task.endAt?.slice(0, 10) || '未設定' }}</span>
          <button type="button" aria-label="期限を1日後" @pointerdown.stop="begin($event, task, 'end')" @pointerup.stop="finish" @keydown.left.prevent="shift(task, 'end', -1)" @keydown.right.prevent="shift(task, 'end', 1)">→</button>
        </div>
      </div>
    </div>
    <ImpactModal
      :open="Boolean(impact)"
      title="日付変更の影響を確認"
      :impacts="impactLines(impact)"
      @cancel="impact=undefined; pending=undefined"
      @confirm="confirm"
    />
  </section>
</template>

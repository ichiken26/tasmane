<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import type { ApiTask, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { impactPlanFrom } from '../../core/composables/impact'
import { barGeometry, createKeyNudge, dragDates, ganttRange, type GanttDragMode } from '../../core/composables/gantt'
import { dayDiff, formatDate } from '../../core/composables/dates'
import { useNotify } from '../../shared/composables/useNotify'

// Gantt（バー全体Drag=移動 / 左端=開始 / 右端=期限）
const props = defineProps<{ tasks: ApiTask[] }>()
const emit = defineEmits<{ updated: [task: ApiTask] }>()
const api = useTasmaneApi()
const notify = useNotify()
const DAY = 32
const impact = ref<ImpactPlan>()
const pending = ref<{ task: ApiTask; startAt: string | null; endAt: string | null }>()
const savingId = ref('')
const showCompleted = ref(false)
const scroller = ref<HTMLElement>()
const drag = ref<{ task: ApiTask; mode: GanttDragMode; x: number; delta: number }>()

// 階層順（親→子）に並べた日程ありタスク
const rows = computed(() => {
  const scheduled = props.tasks.filter(task => (task.startAt || task.endAt) && (showCompleted.value || task.status !== 'COMPLETED'))
  const ids = new Set(scheduled.map(task => task.id))
  const children = new Map<string | null, ApiTask[]>()
  for (const task of scheduled) {
    const key = task.parentTaskId && ids.has(task.parentTaskId) ? task.parentTaskId : null
    children.set(key, [...(children.get(key) ?? []), task])
  }
  const start = (task: ApiTask) => Date.parse(task.startAt || task.endAt || '')
  const result: { task: ApiTask; depth: number }[] = []
  const visit = (parent: string | null, depth: number) => {
    for (const task of (children.get(parent) ?? []).sort((a, b) => start(a) - start(b))) {
      result.push({ task, depth })
      visit(task.id, depth + 1)
    }
  }
  visit(null, 0)
  return result
})
const unscheduled = computed(() => props.tasks.filter(task => !task.startAt && !task.endAt && task.status !== 'COMPLETED').length)

// 日付軸
const range = computed(() => ganttRange(rows.value.map(row => row.task)))
const days = computed(() => Array.from({ length: range.value.days }, (_, index) => {
  const date = new Date(range.value.start)
  date.setDate(date.getDate() + index)
  return date
}))
const months = computed(() => {
  const result: { label: string; span: number }[] = []
  for (const date of days.value) {
    const label = `${date.getFullYear()}年${date.getMonth() + 1}月`
    const last = result.at(-1)
    if (last?.label === label) last.span += 1
    else result.push({ label, span: 1 })
  }
  return result
})
const todayOffset = computed(() => dayDiff(range.value.start, new Date()))
const isWeekend = (date: Date) => date.getDay() === 0 || date.getDay() === 6

// バー位置（ドラッグ中・キー操作の保存待ちはプレビュー）
function geometry(task: ApiTask) {
  const keyed = keyPreview.value?.id === task.id ? keyPreview.value.days : 0
  const dates = drag.value?.task.id === task.id && drag.value.delta ? dragDates(task, drag.value.mode, drag.value.delta)
    : keyed ? dragDates(task, 'end', keyed) : task
  return barGeometry(dates, range.value.start) ?? { offset: 0, span: 1 }
}
// 変更中の日数表示（ドラッグ中 / キー操作の保存待ち）
function badge(task: ApiTask) {
  const days = drag.value?.task.id === task.id ? drag.value.delta : keyPreview.value?.id === task.id ? keyPreview.value.days : 0
  if (!days) return ''
  return `${drag.value?.task.id === task.id ? '' : '期限 '}${days > 0 ? '+' : ''}${days}日`
}
const barColor = (task: ApiTask) => task.isOverdue ? 'bg-(--ui-error)' : task.status === 'COMPLETED' ? 'bg-(--ui-success)' : task.status === 'IN_PROGRESS' ? 'bg-(--ui-primary)' : 'bg-(--ui-text-dimmed)'

// ドラッグ操作
function begin(event: PointerEvent, task: ApiTask, mode: GanttDragMode) {
  if (savingId.value || event.button !== 0) return
  drag.value = { task, mode, x: event.clientX, delta: 0 }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}
function move(event: PointerEvent) {
  if (drag.value) drag.value.delta = Math.round((event.clientX - drag.value.x) / DAY)
}
async function finish() {
  if (!drag.value) return
  const { task, mode, delta } = drag.value
  if (delta) await shift(task, mode, delta)
  drag.value = undefined
}

// キーボード操作（←→: 期限だけを1日ずつ。保存は最後の操作から3秒後にまとめて1回）
const keyPreview = ref<{ id: string; days: number }>()
const nudge = createKeyNudge((id, days) => {
  keyPreview.value = undefined
  const task = props.tasks.find(item => item.id === id)
  if (task) void shift(task, 'end', days)
})
function onKey(event: KeyboardEvent, task: ApiTask) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  if (savingId.value) return
  nudge.press(task.id, event.key === 'ArrowLeft' ? -1 : 1)
  keyPreview.value = { id: task.id, days: nudge.delta(task.id) }
}
// 画面を離れるときは保存待ちの変更をすぐ送る
onBeforeUnmount(() => nudge.flush())

// 日付変更の送信と影響確認
async function shift(task: ApiTask, mode: GanttDragMode, delta: number, token?: string) {
  const change = { task, ...dragDates(task, mode, delta) }
  if (change.startAt === (task.startAt ?? null) && change.endAt === (task.endAt ?? null)) return
  pending.value = change
  await submit(token)
}
async function submit(token?: string) {
  if (!pending.value) return
  const { task, startAt, endAt } = pending.value
  savingId.value = task.id
  try {
    const saved = await api.updateTask(task.id, { expectedVersion: task.version, startAt, endAt, ...(token ? { impactToken: token } : {}) })
    impact.value = undefined
    pending.value = undefined
    emit('updated', saved)
    notify.success('日程を変更しました', `${saved.title}: ${formatDate(saved.startAt)} → ${formatDate(saved.endAt)}`)
    // 依存関係と日程の矛盾は保存した上で警告
    if (saved.warnings?.length) notify.warning('依存関係と日程が矛盾しています', saved.warnings.join(' '))
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan && !token) { impact.value = plan; return }
    impact.value = undefined
    pending.value = undefined
    notify.error(error, '日程を更新できませんでした。')
  } finally {
    savingId.value = ''
  }
}

// 初期表示で今日付近へスクロール
onMounted(async () => {
  await nextTick()
  if (scroller.value) scroller.value.scrollLeft = Math.max(0, (todayOffset.value - 3) * DAY)
})
</script>

<template>
  <UCard :ui="{ body: 'p-0 sm:p-0' }">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="flex items-center gap-2 font-semibold text-(--ui-text-highlighted)"><UIcon name="i-lucide-gantt-chart" class="size-5 text-(--ui-text-muted)" />Gantt</h2>
          <p class="text-xs text-(--ui-text-muted)">バーをドラッグで移動、左右の端で開始/期限を変更。キーボード: <UKbd value="←" /><UKbd value="→" /> で期限を1日ずつ変更（操作が止まって3秒後に保存）</p>
        </div>
        <USwitch v-model="showCompleted" label="完了済みも表示" size="sm" />
      </div>
    </template>

    <EmptyState v-if="!rows.length" class="m-4" icon="i-lucide-calendar-range" title="日程が設定されたタスクはありません" description="タスクに開始日または期限を設定するとGanttに表示されます。" />
    <!-- Ganttバー操作 -->
    <div v-else ref="scroller" class="relative overflow-x-auto">
      <div class="grid min-w-max" :style="{ gridTemplateColumns: `minmax(180px, 240px) ${range.days * DAY}px` }">
        <!-- 軸ヘッダー -->
        <div class="sticky left-0 z-20 row-span-2 flex items-end border-r border-b border-(--ui-border) bg-(--ui-bg) px-4 pb-2 text-xs font-medium text-(--ui-text-muted)">タスク</div>
        <div class="flex border-b border-(--ui-border-muted) text-xs text-(--ui-text-muted)">
          <div v-for="month in months" :key="month.label" class="truncate border-l border-(--ui-border-muted) px-2 py-1 first:border-l-0" :style="{ width: `${month.span * DAY}px` }">{{ month.label }}</div>
        </div>
        <div class="flex border-b border-(--ui-border)">
          <div v-for="(date, index) in days" :key="index" class="py-1 text-center font-mono text-[13px] tabular-nums" :class="[index === todayOffset ? 'font-bold text-(--ui-primary)' : isWeekend(date) ? 'text-(--ui-text-dimmed)' : 'text-(--ui-text-muted)']" :style="{ width: `${DAY}px` }">{{ date.getDate() }}</div>
        </div>

        <template v-for="{ task, depth } in rows" :key="task.id">
          <div class="sticky left-0 z-10 flex h-10 items-center gap-1.5 border-r border-b border-(--ui-border-muted) bg-(--ui-bg) pr-3 text-sm" :style="{ paddingLeft: `${18 + depth * 14}px` }">
            <UIcon v-if="depth" name="i-lucide-corner-down-right" class="size-3.5 shrink-0 text-(--ui-text-dimmed)" />
            <NuxtLink :to="`/tasks/${task.id}`" class="truncate hover:underline" :title="task.title">{{ task.title }}</NuxtLink>
            <FireMark v-if="task.showFire" />
          </div>
          <div class="relative h-10 border-b border-(--ui-border-muted)" :style="{ backgroundImage: `repeating-linear-gradient(to right, transparent 0 ${DAY - 1}px, var(--ui-border-muted) ${DAY - 1}px ${DAY}px)` }">
            <template v-for="(date, index) in days" :key="index">
              <div v-if="isWeekend(date)" class="absolute inset-y-0 bg-(--ui-bg-muted)" :style="{ left: `${index * DAY}px`, width: `${DAY}px` }" />
            </template>
            <div v-if="todayOffset >= 0 && todayOffset < range.days" class="absolute inset-y-0 z-[1] w-0.5 bg-(--ui-primary)/60" :style="{ left: `${todayOffset * DAY + DAY / 2}px` }" />
            <div
              class="group absolute top-1.5 z-[2] flex h-7 cursor-grab touch-none items-center overflow-hidden rounded-md text-xs font-medium text-white shadow-sm select-none active:cursor-grabbing"
              :class="[barColor(task), { 'opacity-60': savingId === task.id, 'ring-2 ring-(--ui-primary) ring-offset-1 ring-offset-(--ui-bg)': drag?.task.id === task.id }]"
              :style="{ left: `${geometry(task).offset * DAY + 2}px`, width: `${geometry(task).span * DAY - 4}px` }"
              tabindex="0"
              role="button"
              aria-roledescription="Ganttバー"
              :aria-label="`${task.title}の日程 ${formatDate(task.startAt)}〜${formatDate(task.endAt)}。左右の矢印キーで期限を変更`"
              aria-keyshortcuts="ArrowLeft ArrowRight"
              @pointerdown="begin($event, task, 'move')"
              @pointermove="move"
              @pointerup="finish"
              @pointercancel="drag = undefined"
              @keydown="onKey($event, task)"
            >
              <div v-if="task.progress" class="absolute inset-y-0 left-0 bg-white/20" :style="{ width: `${task.progress}%` }" />
              <span class="absolute inset-y-0 left-0 w-2 cursor-ew-resize hover:bg-white/30" aria-hidden="true" @pointerdown.stop="begin($event, task, 'start')" @pointermove="move" @pointerup.stop="finish" />
              <span class="relative truncate px-2.5">{{ geometry(task).span > 2 ? task.title : '' }}</span>
              <span class="absolute inset-y-0 right-0 w-2 cursor-ew-resize hover:bg-white/30" aria-hidden="true" @pointerdown.stop="begin($event, task, 'end')" @pointermove="move" @pointerup.stop="finish" />
            </div>
            <div v-if="badge(task)" class="pointer-events-none absolute -top-0.5 z-[3] rounded bg-(--ui-bg-inverted) px-1.5 py-0.5 text-[13px] text-(--ui-text-inverted)" :style="{ left: `${geometry(task).offset * DAY}px` }">
              {{ badge(task) }}
            </div>
          </div>
        </template>
      </div>
    </div>
    <p v-if="unscheduled" class="border-t border-(--ui-border) px-4 py-2 text-xs text-(--ui-text-muted)">日程未設定のタスクが {{ unscheduled }} 件あります（タスク詳細で設定できます）。</p>

    <!-- 日付変更の影響確認 -->
    <ImpactModal :open="Boolean(impact)" title="日付変更の影響を確認" :plan="impact" :loading="Boolean(savingId)" @cancel="impact = undefined; pending = undefined" @confirm="impact && submit(impact.impactToken)" />
  </UCard>
</template>

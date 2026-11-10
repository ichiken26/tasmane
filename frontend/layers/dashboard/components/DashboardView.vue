<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from '#imports'
import type { ApiAdvice, ApiConfirmation, ApiTask } from '../../core/composables/useCriticalData'
import { deadlineSection, type DeadlineSection } from '../../core/composables/taskState'
import { formatDate, relativeDeadline } from '../../core/composables/dates'

// Dashboard本体（全体/ジャンル共通）
const props = withDefaults(defineProps<{ genre?: string; tasks: ApiTask[]; confirmations?: ApiConfirmation[]; advice?: ApiAdvice | null }>(), { genre: undefined, confirmations: () => [], advice: null })
const route = useRoute()
const router = useRouter()
type SortKey = 'score' | 'priority' | 'deadline'

// 並び順（URLに保持しブックマーク・戻る操作で復元）
const sort = computed<SortKey>({
  get: () => (['score', 'priority', 'deadline'] as const).find(key => key === route.query.sort) ?? 'score',
  set: (value) => { void router.replace({ query: { ...route.query, sort: value === 'score' ? undefined : value } }) },
})
const sortTabs = [
  { label: 'おすすめ順', value: 'score', icon: 'i-lucide-sparkles' },
  { label: '優先度順', value: 'priority', icon: 'i-lucide-signal-high' },
  { label: '期限順', value: 'deadline', icon: 'i-lucide-calendar-clock' },
]
const priorityRank: Record<string, number> = { HIGH: 3, MEDIUM: 2, LOW: 1 }
const endTime = (task: ApiTask) => task.endAt ? Date.parse(task.endAt) : Number.MAX_SAFE_INTEGER

// おすすめ着手順の並び替え
const open = computed(() => props.tasks.filter(task => task.status !== 'COMPLETED'))
const sorted = computed(() => open.value.filter(task => task.recommended !== false).sort((a, b) => {
  if (sort.value === 'priority') return (priorityRank[b.effectivePriority] || 0) - (priorityRank[a.effectivePriority] || 0) || endTime(a) - endTime(b) || a.title.localeCompare(b.title)
  if (sort.value === 'deadline') return endTime(a) - endTime(b) || a.title.localeCompare(b.title)
  return (b.taskScore || 0) - (a.taskScore || 0) || endTime(a) - endTime(b) || a.title.localeCompare(b.title)
}))

// 進捗と期限切れ件数の算出
const progress = computed(() => props.tasks.length ? Math.floor(props.tasks.reduce((sum, task) => sum + (task.progress ?? 0), 0) / props.tasks.length) : 0)
const completed = computed(() => props.tasks.filter(task => task.status === 'COMPLETED').length)
const overdue = computed(() => props.tasks.filter(task => task.showFire).length)
const openConfirmations = computed(() => props.confirmations.filter(item => item.status !== 'CONFIRMED'))

// 期限区分ごとのタスク分類（未完了のみ）
const sectionLabels: Record<DeadlineSection, string> = { overdue: '期限切れ', today: '今日まで', tomorrow: '明日まで', week: '1週間以内', month: '1か月以内', later: 'それ以降' }
const deadlineGroups = computed(() => (Object.entries(sectionLabels) as [DeadlineSection, string][]).map(([key, label]) => ({
  key, label, tasks: open.value.filter(task => task.endAt && deadlineSection(task.endAt.slice(0, 10)) === key).sort((a, b) => endTime(a) - endTime(b)),
})))
const dueToday = computed(() => (deadlineGroups.value.find(group => group.key === 'today')?.tasks.length ?? 0))
const deadlineTabs = computed(() => deadlineGroups.value.map(group => ({ label: group.label, value: group.key, badge: group.tasks.length ? { label: String(group.tasks.length), color: group.key === 'overdue' ? 'error' as const : 'neutral' as const, variant: 'subtle' as const } : undefined })))
const firstFilledSection = computed(() => deadlineGroups.value.find(group => group.tasks.length)?.key ?? 'today')
</script>

<template>
  <div class="space-y-6">
    <!-- サマリー -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <StatCard label="全体の進捗" :value="`${progress}%`" icon="i-lucide-chart-pie" tone="primary">
        <UProgress :model-value="progress" size="xs" class="mb-1.5" />完了 {{ completed }} / 全 {{ tasks.length }} 件
      </StatCard>
      <StatCard label="期限切れ" :value="overdue" icon="i-lucide-flame" :tone="overdue ? 'error' : 'neutral'" hint="子孫の期限切れを含む" />
      <StatCard label="今日が期限" :value="dueToday" icon="i-lucide-calendar-check" :tone="dueToday ? 'warning' : 'neutral'" hint="未完了のタスク" />
      <StatCard label="未完了の確認事項" :value="openConfirmations.length" icon="i-lucide-message-circle-question" :tone="openConfirmations.length ? 'info' : 'neutral'" hint="確認待ちの事項" />
    </div>

    <!-- AIアドバイス -->
    <UCard class="border-(--ui-primary)/30 bg-gradient-to-br from-(--ui-primary)/8 to-transparent">
      <div class="flex gap-3">
        <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-(--ui-primary)/15 text-(--ui-primary)"><UIcon name="i-lucide-sparkles" class="size-5" /></span>
        <div class="min-w-0">
          <p class="text-sm font-medium text-(--ui-text-highlighted)">今日の焦点 <span class="ml-1 text-xs font-normal text-(--ui-text-muted)">AI Advice<template v-if="advice"> ・ {{ formatDate(advice.generatedAt, true) }} 生成</template></span></p>
          <p class="mt-1 whitespace-pre-line text-(--ui-text-toned)">{{ advice?.content || 'アドバイスはまだ生成されていません。定期実行で生成されると、ここに表示されます。' }}</p>
        </div>
      </div>
    </UCard>

    <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
      <!-- おすすめ着手順 -->
      <UCard :ui="{ body: 'p-0 sm:p-0' }">
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold text-(--ui-text-highlighted)">おすすめ着手順</h2>
              <p class="text-xs text-(--ui-text-muted)">期限・優先度・依存・進捗から決定論的に算出しています</p>
            </div>
            <UTabs v-model="sort" :items="sortTabs" :content="false" size="xs" color="neutral" aria-label="並び順" />
          </div>
        </template>
        <ol v-if="sorted.length" class="divide-y divide-(--ui-border)">
          <li v-for="(task, index) in sorted" :key="task.id"><TaskRow :task="task" :rank="sort === 'score' ? index + 1 : undefined" :show-genre="!genre" reasons /></li>
        </ol>
        <div v-else class="p-4">
          <EmptyState icon="i-lucide-party-popper" title="着手候補はありません" description="未完了のタスクがありません。新しいタスクを追加しましょう。">
            <UButton to="/tasks/new" icon="i-lucide-plus" size="sm">タスクを追加</UButton>
          </EmptyState>
        </div>
      </UCard>

      <div class="space-y-6">
        <!-- 期限区分 -->
        <UCard :ui="{ body: 'p-0 sm:p-0' }">
          <template #header><h2 class="font-semibold text-(--ui-text-highlighted)">期限区分</h2></template>
          <UTabs :key="firstFilledSection" :items="deadlineTabs" :default-value="firstFilledSection" variant="link" size="xs" class="w-full" :ui="{ list: 'overflow-x-auto px-2', trigger: 'shrink-0' }">
            <template #content="{ item }">
              <ul v-if="deadlineGroups.find(group => group.key === item.value)?.tasks.length" class="max-h-96 divide-y divide-(--ui-border) overflow-y-auto">
                <li v-for="task in deadlineGroups.find(group => group.key === item.value)?.tasks" :key="task.id" class="flex items-center gap-2 px-4 py-2.5 text-sm">
                  <NuxtLink :to="`/tasks/${task.id}`" class="min-w-0 flex-1 truncate hover:underline">{{ task.title }}</NuxtLink>
                  <span class="shrink-0 text-xs" :class="task.isOverdue ? 'text-(--ui-error)' : 'text-(--ui-text-muted)'">{{ relativeDeadline(task.endAt!) }}</span>
                </li>
              </ul>
              <p v-else class="px-4 py-6 text-center text-sm text-(--ui-text-muted)">該当するタスクはありません。</p>
            </template>
          </UTabs>
        </UCard>

        <!-- 確認事項（ConfirmationScore順） -->
        <UCard :ui="{ body: 'p-0 sm:p-0' }">
          <template #header><h2 class="font-semibold text-(--ui-text-highlighted)">確認事項</h2></template>
          <ol v-if="openConfirmations.length" class="divide-y divide-(--ui-border)">
            <li v-for="item in openConfirmations" :key="item.id" class="px-4 py-3">
              <div class="flex items-start justify-between gap-2">
                <p class="font-medium text-(--ui-text-highlighted)">{{ item.content }}</p>
                <StatusBadge :value="item.status" kind="confirmation" />
              </div>
              <p class="mt-1 flex flex-wrap gap-x-3 text-xs text-(--ui-text-muted)">
                <NuxtLink :to="`/tasks/${item.taskId}`" class="inline-flex items-center gap-1 hover:underline"><UIcon name="i-lucide-square-check-big" />{{ item.taskTitle }}</NuxtLink>
                <span class="inline-flex items-center gap-1"><UIcon name="i-lucide-user" />{{ item.targetName }}</span>
                <span v-if="item.dueAt" class="inline-flex items-center gap-1"><UIcon name="i-lucide-calendar" />{{ relativeDeadline(item.dueAt) }}</span>
              </p>
            </li>
          </ol>
          <p v-else class="px-4 py-6 text-center text-sm text-(--ui-text-muted)">未完了の確認事項はありません。</p>
        </UCard>
      </div>
    </div>
  </div>
</template>

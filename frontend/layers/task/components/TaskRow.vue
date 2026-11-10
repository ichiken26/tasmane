<script setup lang="ts">
import type { ApiTask } from '../../core/composables/useCriticalData'
import { formatDate, relativeDeadline } from '../../core/composables/dates'
import { useGenreList } from '../../core/composables/useGenreList'

// タスク一覧の1行（タイトル・ジャンル・期限・状態・優先度・進捗）
withDefaults(defineProps<{ task: ApiTask; rank?: number; showGenre?: boolean; reasons?: boolean }>(), { rank: undefined, showGenre: true, reasons: false })
const { nameOf } = useGenreList()
</script>

<template>
  <div class="group flex items-start gap-3 px-4 py-3 transition-colors hover:bg-(--ui-bg-muted)">
    <span v-if="rank" class="mt-0.5 grid size-7 shrink-0 place-items-center rounded-full font-mono text-xs font-medium" :class="rank <= 3 ? 'bg-(--ui-primary) text-(--ui-text-inverted)' : 'bg-(--ui-bg-accented) text-(--ui-text-toned)'">{{ rank }}</span>
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <NuxtLink :to="`/tasks/${task.id}`" class="truncate font-medium text-(--ui-text-highlighted) hover:underline">{{ task.title }}</NuxtLink>
        <FireMark v-if="task.showFire" />
      </div>
      <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-(--ui-text-muted)">
        <span v-if="showGenre && nameOf(task.genreId)" class="inline-flex items-center gap-1"><UIcon name="i-lucide-folder" />{{ nameOf(task.genreId) }}</span>
        <span v-if="task.endAt" class="inline-flex items-center gap-1" :class="{ 'font-medium text-(--ui-error)': task.isOverdue }">
          <UIcon name="i-lucide-calendar-clock" />{{ formatDate(task.endAt, !task.isAllDay) }}<template v-if="task.status !== 'COMPLETED'">・{{ relativeDeadline(task.endAt) }}</template>
        </span>
        <span v-if="task.progress !== null" class="inline-flex items-center gap-1.5"><UProgress :model-value="task.progress" size="2xs" class="w-14" />{{ task.progress }}%</span>
      </div>
      <div v-if="reasons && task.recommendationReasons?.length" class="mt-1.5 flex flex-wrap gap-1">
        <UBadge v-for="reason in task.recommendationReasons" :key="reason" :label="reason" size="sm" color="neutral" variant="outline" />
      </div>
    </div>
    <div class="flex shrink-0 flex-col items-end gap-1 sm:flex-row sm:items-center">
      <StatusBadge :value="task.status" />
      <StatusBadge :value="task.effectivePriority" kind="priority" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ApiConfirmation, ApiTask } from '../../core/composables/useCriticalData'
import { deadlineSection } from '../../core/composables/taskState'
const props = defineProps<{ genre?: string; tasks: ApiTask[]; confirmations?: ApiConfirmation[]; advice?: string }>()
const sort = ref<'score'|'priority'|'deadline'>('score')
const priorityRank: Record<string, number> = { HIGH: 3, MEDIUM: 2, LOW: 1 }
const sorted = computed(() => props.tasks.filter(task => task.recommended !== false && task.status !== 'COMPLETED').sort((a, b) => {
  if (sort.value === 'priority') return (priorityRank[b.effectivePriority] || 0) - (priorityRank[a.effectivePriority] || 0) || a.title.localeCompare(b.title)
  if (sort.value === 'deadline') return (a.endAt ? Date.parse(a.endAt) : Number.MAX_SAFE_INTEGER) - (b.endAt ? Date.parse(b.endAt) : Number.MAX_SAFE_INTEGER) || a.title.localeCompare(b.title)
  return (b.taskScore || 0) - (a.taskScore || 0) || (a.endAt ? Date.parse(a.endAt) : Number.MAX_SAFE_INTEGER) - (b.endAt ? Date.parse(b.endAt) : Number.MAX_SAFE_INTEGER) || a.title.localeCompare(b.title)
}))
const progress = computed(() => props.tasks.length ? Math.floor(props.tasks.reduce((sum, task) => sum + (task.progress ?? 0), 0) / props.tasks.length) : 0)
const overdue = computed(() => props.tasks.filter(task => task.showFire).length)
const sectionLabels = { overdue: '期限切れ', today: '今日まで', tomorrow: '明日まで', week: '1週間以内', month: '1か月以内', later: 'それ以降' }
const deadlineGroups = computed(() => Object.entries(sectionLabels).map(([key, label]) => ({ key, label, tasks: props.tasks.filter(task => task.endAt && deadlineSection(task.endAt.slice(0, 10)) === key) })))
</script>
<template><div><div class="grid"><article class="card"><span class="eyebrow">AI Advice</span><h2>今日の焦点</h2><p>{{ advice || 'アドバイスはまだ生成されていません。' }}</p></article><article class="card"><span class="eyebrow">Progress</span><div class="metric">{{ progress }}%</div><p class="muted">{{ genre || '全ジャンル' }}</p></article><article class="card"><span class="eyebrow">Overdue</span><div class="metric fire">🔥 {{ overdue }}</div><p class="muted">子孫の期限切れを含む</p></article></div><section class="card"><div class="actions"><h2>おすすめ着手順</h2><label>並び順 <select v-model="sort"><option value="score">TaskScore順</option><option value="priority">優先度順</option><option value="deadline">期限順</option></select></label></div><div class="table-wrap"><table><thead><tr><th>Task</th><th>理由</th><th>状態</th><th>優先度</th></tr></thead><tbody><tr v-for="task in sorted" :key="task.id"><td><NuxtLink :to="`/tasks/${task.id}`">{{ task.title }}</NuxtLink></td><td class="muted">{{ task.recommendationReasons?.join('・') || '総合評価' }}</td><td>{{ task.status }}</td><td>{{ task.effectivePriority }}</td></tr></tbody></table></div></section><div class="grid two"><section class="card"><h2>期限区分</h2><div v-for="group in deadlineGroups" :key="group.key"><h3>{{ group.label }} <span class="muted">{{ group.tasks.length }}</span></h3><ul><li v-for="task in group.tasks" :key="task.id"><NuxtLink :to="`/tasks/${task.id}`">{{ task.title }}</NuxtLink></li></ul></div></section><section class="card"><h2>確認事項</h2><p v-if="!confirmations?.length" class="muted">未完了の確認事項はありません。</p><ol v-else><li v-for="item in confirmations" :key="item.id"><strong>{{ item.content }}</strong><br><span class="muted">{{ item.taskTitle }} / {{ item.targetName }} / {{ item.status }}</span></li></ol></section></div></div></template>

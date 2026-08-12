<script setup lang="ts">
import { computed } from 'vue'
import type { ApiTask } from '../../core/composables/useCriticalData'
const props = defineProps<{ genre?: string; tasks: ApiTask[]; advice?: string }>()
const sort = ref<'score'|'priority'|'deadline'>('score')
const sorted = computed(() => [...props.tasks].sort((a, b) => sort.value === 'priority' ? a.effectivePriority.localeCompare(b.effectivePriority) : a.title.localeCompare(b.title)))
const progress = computed(() => props.tasks.length ? Math.floor(props.tasks.reduce((sum, task) => sum + (task.progress ?? 0), 0) / props.tasks.length) : 0)
const overdue = computed(() => props.tasks.filter(task => task.showFire).length)
</script>
<template><div><div class="grid"><article class="card"><span class="eyebrow">AI Advice</span><h2>今日の焦点</h2><p>{{ advice || 'アドバイスはまだ生成されていません。' }}</p></article><article class="card"><span class="eyebrow">Progress</span><div class="metric">{{ progress }}%</div><p class="muted">{{ genre || '全ジャンル' }}</p></article><article class="card"><span class="eyebrow">Overdue</span><div class="metric fire">🔥 {{ overdue }}</div><p class="muted">子孫の期限切れを含む</p></article></div><section class="card"><div class="actions"><h2>おすすめ着手順</h2><label>並び順 <select v-model="sort"><option value="score">TaskScore順</option><option value="priority">優先度順</option><option value="deadline">期限順</option></select></label></div><div class="table-wrap"><table><thead><tr><th>Task</th><th>状態</th><th>優先度</th></tr></thead><tbody><tr v-for="task in sorted" :key="task.id"><td><NuxtLink :to="`/tasks/${task.id}`">{{ task.title }}</NuxtLink></td><td>{{ task.status }}</td><td>{{ task.effectivePriority }}</td></tr></tbody></table></div></section></div></template>

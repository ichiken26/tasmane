<script setup lang="ts">
defineProps<{ tasks: Array<{ id: string; title: string; progress: number; overdue?: boolean }> }>()
const collapsed = ref(new Set<string>())
</script>
<template><section class="card"><h2>Hierarchy Tree</h2><ul class="tree"><li v-for="task in tasks" :key="task.id"><button type="button" :aria-expanded="!collapsed.has(task.id)" @click="collapsed.has(task.id) ? collapsed.delete(task.id) : collapsed.add(task.id)">▾</button><NuxtLink :to="`/tasks/${task.id}`" target="_blank">{{ task.title }}</NuxtLink><span>{{ task.progress }}%</span><span v-if="task.overdue" aria-label="期限切れ">🔥</span></li></ul></section></template>

<script setup lang="ts">
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
const route = useRoute()
const genreId = String(route.params.genreId)
const api = useTasmaneApi()
const { data, status, error } = useCriticalData(`dashboard-genre-${genreId}`, () => api.dashboard(genreId))
useSeoMeta({ title: `Genre | タスマネ` })
</script>
<template><div class="page"><span class="eyebrow">Genre Dashboard</span><h1>Genre Dashboard</h1><p v-if="status === 'pending'" role="status">Dashboardを読み込み中…</p><p v-else-if="error" role="alert">Dashboardを取得できませんでした。</p><template v-else><DashboardView :genre="genreId" :tasks="data?.tasks || []"/><section class="card"><h2>Gantt</h2><div class="gantt"><div v-for="task in (data?.tasks || []).filter(item => item.startAt || item.endAt)" :key="task.id" class="gantt-row"><span>{{ task.title }}</span><div class="gantt-bar">{{ task.startAt || '未設定' }} — {{ task.endAt || '未設定' }}</div></div></div></section></template></div></template>

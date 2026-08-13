<script setup lang="ts">
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
const route = useRoute()
const genreId = String(route.params.genreId)
const api = useTasmaneApi()
const { data, status, error } = useCriticalData(`dashboard-genre-${genreId}`, () => api.dashboard(genreId))
useSeoMeta({ title: `Genre | タスマネ` })
function updateTask(task: ApiTask) {
  const index = data.value?.tasks.findIndex(item => item.id === task.id) ?? -1
  if (data.value && index >= 0) data.value.tasks[index] = task
}
</script>
<template><div class="page"><span class="eyebrow">Genre Dashboard</span><h1>Genre Dashboard</h1><p v-if="status === 'pending'" role="status">Dashboardを読み込み中…</p><p v-else-if="error" role="alert">Dashboardを取得できませんでした。</p><template v-else><DashboardView :genre="genreId" :tasks="data?.tasks || []" :confirmations="data?.confirmations || []" :advice="data?.advice?.content"/><GanttEditor :tasks="data?.tasks || []" @updated="updateTask" /></template></div></template>

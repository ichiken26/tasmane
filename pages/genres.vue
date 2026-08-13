<script setup lang="ts">
import { ref, watch } from 'vue'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
useSeoMeta({ title: 'ジャンル一覧 | タスマネ' })
const api = useTasmaneApi()
const { data: genres, status, error } = useCriticalData('genres', api.genres)
const roots = ref<Record<string, Awaited<ReturnType<typeof api.dashboard>>['tasks']>>({})
watch(genres, async (values) => {
  if (!values) return
  const entries = await Promise.all(values.map(async genre => [genre.id, (await api.dashboard(genre.id)).tasks.filter(task => !task.parentTaskId)] as const))
  roots.value = Object.fromEntries(entries)
}, { immediate: true })
</script>
<template><div class="page"><span class="eyebrow">Genres</span><h1>ジャンル一覧</h1><p v-if="status === 'pending'" role="status">ジャンルを読み込み中…</p><p v-else-if="error" role="alert">ジャンルを取得できませんでした。</p><div v-else class="grid"><article v-for="genre in genres || []" :key="genre.id" class="card"><h2>{{ genre.name }}</h2><p v-if="!roots[genre.id]?.length" class="muted">Root Taskなし</p><ul v-else><li v-for="task in roots[genre.id]" :key="task.id"><NuxtLink :to="`/tasks/${task.id}`">{{ task.title }}</NuxtLink> — {{ task.progress ?? 0 }}%</li></ul><NuxtLink :to="`/dashboard/genres/${genre.id}`">Dashboardへ</NuxtLink></article></div></div></template>

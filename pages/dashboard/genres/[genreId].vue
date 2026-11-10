<script setup lang="ts">
import { computed, ref } from 'vue'
import { definePageMeta, useRoute, useSeoMeta } from '#imports'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'

// ジャンル切替時にページを再生成（同一ルートでのstale表示を防ぐ）
definePageMeta({ key: route => route.path })
const route = useRoute()
const genreId = String(route.params.genreId)
const api = useTasmaneApi()
const { nameOf } = useGenreList()
const editing = ref(false)
const typesOpen = ref(false)
// ジャンルDashboardとジャンル名の取得
const { data, status, error, refresh } = useCriticalData(`dashboard-genre-${genreId}`, () => api.dashboard(genreId))
const { data: genre } = useCriticalData(`genre-${genreId}`, () => api.genre(genreId).catch(() => undefined))
const genreName = computed(() => nameOf(genreId) || genre.value?.name || 'Genre')
useSeoMeta({ title: () => `${genreName.value} | タスマネ` })

// Gantt更新後のタスク差し替え（日付伝播で他タスクも変わるため再取得）
function updateTask(task: ApiTask) {
  const index = data.value?.tasks.findIndex(item => item.id === task.id) ?? -1
  if (data.value && index >= 0) data.value.tasks[index] = task
  void refresh()
}
</script>

<template>
  <!-- ジャンルDashboard -->
  <div class="mx-auto max-w-7xl">
    <UBreadcrumb :items="[{ label: 'Dashboard', to: '/dashboard', icon: 'i-lucide-layout-dashboard' }, { label: genreName }]" class="mb-3" />
    <PageHeader eyebrow="Genre Dashboard" :title="genreName">
      <template #actions>
        <UButton icon="i-lucide-tags" color="neutral" variant="outline" @click="typesOpen = true">タスクタイプ</UButton>
        <UButton icon="i-lucide-pencil" color="neutral" variant="outline" aria-label="ジャンル名を変更" @click="editing = true" />
        <UButton :to="{ path: '/tasks/new', query: { genreId } }" icon="i-lucide-plus">このジャンルに追加</UButton>
      </template>
    </PageHeader>
    <DashboardSkeleton v-if="status === 'pending' && !data" />
    <ErrorState v-else-if="error" :error="error" fallback="Dashboardを取得できませんでした。" @retry="refresh()" />
    <div v-else class="space-y-6">
      <!-- Gantt編集 -->
      <GanttEditor :tasks="data?.tasks || []" @updated="updateTask" />
      <DashboardView :genre="genreId" :tasks="data?.tasks || []" :confirmations="data?.confirmations || []" :advice="data?.advice" />
    </div>
    <GenreFormModal v-if="genre" v-model:open="editing" :genre="genre" @saved="value => genre = value" />
    <TaskTypeManager v-if="genre" v-model:open="typesOpen" :genre="genre" />
  </div>
</template>

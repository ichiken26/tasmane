<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { ApiGenre } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'

const route = useRoute()
const api = useTasmaneApi()
const genres = ref<ApiGenre[]>([])
const loadError = ref('')

onMounted(async () => {
  try {
    genres.value = await api.genres()
  } catch {
    loadError.value = 'ジャンルを取得できませんでした。'
  }
})

function genreActive(id: string) {
  return route.path === `/dashboard/genres/${id}` || route.path.startsWith(`/dashboard/genres/${id}/`)
}
</script>

<template>
  <aside class="sidebar" aria-label="サイドバー">
    <NuxtLink to="/dashboard">Dashboard</NuxtLink>
    <strong>Genres</strong>
    <p v-if="loadError" class="muted" role="alert">{{ loadError }}</p>
    <p v-else-if="!genres.length" class="muted">ジャンルはまだありません</p>
    <NuxtLink
      v-for="genre in genres"
      :key="genre.id"
      :to="`/dashboard/genres/${genre.id}`"
      :aria-current="genreActive(genre.id) ? 'page' : undefined"
      :class="{ 'router-link-active': genreActive(genre.id) }"
    >
      {{ genre.name }}
    </NuxtLink>
  </aside>
</template>

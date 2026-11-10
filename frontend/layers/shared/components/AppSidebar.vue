<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { NavigationMenuItem } from '@nuxt/ui'
import { useRoute } from '#imports'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useGenreList } from '../../core/composables/useGenreList'

const route = useRoute()
const api = useTasmaneApi()
const { genres, loaded } = useGenreList()
const loadError = ref('')
const creating = ref(false)

// ジャンル一覧の取得（共有stateへ格納）
async function load() {
  loadError.value = ''
  try {
    genres.value = await api.genres()
    loaded.value = true
  } catch {
    loadError.value = 'ジャンルを取得できませんでした。'
  }
}
onMounted(() => { if (!loaded.value) void load() })

// ジャンルDashboardの選択判定（現在表示中Genreを必ずhighlight）
function genreActive(id: string) {
  if (route.path === `/dashboard/genres/${id}` || route.path.startsWith(`/dashboard/genres/${id}/`)) return true
  return route.query.genreId === id
}

const topItems = computed<NavigationMenuItem[]>(() => [
  { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', to: '/dashboard', active: route.path === '/dashboard' },
  { label: 'ジャンル一覧', icon: 'i-lucide-folders', to: '/genres', active: route.path === '/genres' },
])
const genreItems = computed<NavigationMenuItem[]>(() => genres.value.map(genre => ({
  label: genre.name,
  icon: 'i-lucide-folder',
  to: `/dashboard/genres/${genre.id}`,
  active: genreActive(genre.id),
  'aria-current': genreActive(genre.id) ? 'page' : undefined,
})))
</script>

<template>
  <div class="flex flex-col gap-4 px-3 py-5" aria-label="サイドバー">
    <UNavigationMenu :items="topItems" orientation="vertical" />
    <div>
      <div class="mb-1 flex items-center justify-between px-2.5">
        <span class="text-xs font-medium tracking-wide text-(--ui-text-muted) uppercase">Genres</span>
        <UTooltip text="ジャンルを追加">
          <UButton icon="i-lucide-plus" size="xs" color="neutral" variant="ghost" aria-label="ジャンルを追加" @click="creating = true" />
        </UTooltip>
      </div>
      <!-- ジャンルナビゲーション -->
      <div v-if="loadError" class="px-2.5" role="alert">
        <p class="text-sm text-(--ui-error)">{{ loadError }}</p>
        <UButton size="xs" variant="link" class="px-0" @click="load">再読み込み</UButton>
      </div>
      <div v-else-if="!loaded" class="space-y-2 px-2.5 py-1">
        <USkeleton v-for="index in 3" :key="index" class="h-6 w-full" />
      </div>
      <p v-else-if="!genres.length" class="px-2.5 text-sm text-(--ui-text-muted)">ジャンルはまだありません</p>
      <UNavigationMenu v-else :items="genreItems" orientation="vertical" />
    </div>
    <GenreFormModal v-model:open="creating" />
  </div>
</template>

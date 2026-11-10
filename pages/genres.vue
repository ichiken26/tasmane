<script setup lang="ts">
import { computed, ref } from 'vue'
import type { DropdownMenuItem } from '@nuxt/ui'
import { useSeoMeta } from '#imports'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiGenre, ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'
import { apiErrorStatus } from '~/frontend/layers/core/composables/apiError'
import { useNotify } from '~/frontend/layers/shared/composables/useNotify'

useSeoMeta({ title: 'ジャンル一覧 | タスマネ' })
const api = useTasmaneApi()
const notify = useNotify()
const { genres: sharedGenres, loaded, remove: removeGenre } = useGenreList()
// ジャンル一覧と全タスク（1リクエストで集計し、ジャンル数分の取得を避ける）
const { data, status, error, refresh } = useCriticalData('genres-page', async () => {
  const [genres, dashboard] = await Promise.all([api.genres(), api.dashboard()])
  sharedGenres.value = genres
  loaded.value = true
  return { tasks: dashboard.tasks }
})
const genres = computed(() => sharedGenres.value)
const tasksByGenre = computed(() => {
  const map = new Map<string, ApiTask[]>()
  for (const task of data.value?.tasks ?? []) map.set(task.genreId, [...(map.get(task.genreId) ?? []), task])
  return map
})
// Root Taskと集計
const summary = (genre: ApiGenre) => {
  const tasks = tasksByGenre.value.get(genre.id) ?? []
  const roots = tasks.filter(task => !task.parentTaskId)
  return { roots, total: tasks.length, done: tasks.filter(task => task.status === 'COMPLETED').length, fire: roots.filter(task => task.showFire).length }
}

const creating = ref(false)
const editing = ref<ApiGenre>()
const editOpen = ref(false)
const typesGenre = ref<ApiGenre>()
const typesOpen = ref(false)
const deleting = ref<ApiGenre>()
const deleteOpen = ref(false)
const deleteBusy = ref(false)
const menu = (genre: ApiGenre): DropdownMenuItem[][] => [
  [
    { label: '名前を変更', icon: 'i-lucide-pencil', onSelect: () => { editing.value = genre; editOpen.value = true } },
    { label: 'タスクタイプ', icon: 'i-lucide-tags', onSelect: () => { typesGenre.value = genre; typesOpen.value = true } },
  ],
  [{ label: '削除', icon: 'i-lucide-trash-2', color: 'error', disabled: summary(genre).total > 0, onSelect: () => { deleting.value = genre; deleteOpen.value = true } }],
]

// ジャンル削除（タスクがあるジャンルは削除不可）
async function confirmDelete() {
  if (!deleting.value) return
  deleteBusy.value = true
  try {
    await api.deleteGenre(deleting.value.id)
    removeGenre(deleting.value.id)
    notify.success('ジャンルを削除しました')
    deleteOpen.value = false
  } catch (cause) {
    notify.error(cause, apiErrorStatus(cause) === 409 ? 'タスクが残っているため削除できません。' : '削除できませんでした。')
  } finally { deleteBusy.value = false }
}
</script>

<template>
  <!-- ジャンルカード一覧 -->
  <div class="mx-auto max-w-7xl">
    <PageHeader eyebrow="Genres" title="ジャンル一覧" description="ジャンルごとのRoot Taskと進捗を確認します。">
      <template #actions><UButton icon="i-lucide-folder-plus" @click="creating = true">ジャンルを追加</UButton></template>
    </PageHeader>
    <div v-if="status === 'pending' && !data" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3"><USkeleton v-for="index in 6" :key="index" class="h-56 rounded-xl" /></div>
    <ErrorState v-else-if="error" :error="error" fallback="ジャンルを取得できませんでした。" @retry="refresh()" />
    <EmptyState v-else-if="!genres.length" icon="i-lucide-folders" title="ジャンルがまだありません" description="「仕事」「家事」など、タスクをまとめるジャンルを作成しましょう。">
      <UButton icon="i-lucide-folder-plus" @click="creating = true">ジャンルを追加</UButton>
    </EmptyState>
    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <UCard v-for="genre in genres" :key="genre.id" :ui="{ root: 'flex flex-col', body: 'flex-1 p-0 sm:p-0', footer: 'py-3' }">
        <template #header>
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <h2 class="truncate text-lg font-semibold text-(--ui-text-highlighted)"><NuxtLink :to="`/dashboard/genres/${genre.id}`" class="hover:underline">{{ genre.name }}</NuxtLink></h2>
              <p class="text-xs text-(--ui-text-muted)">タスク {{ summary(genre).total }} 件・完了 {{ summary(genre).done }} 件<span v-if="summary(genre).fire" class="ml-1 inline-flex items-center gap-0.5 text-(--ui-error)">・<UIcon name="i-lucide-flame" />期限切れ {{ summary(genre).fire }}</span></p>
            </div>
            <UDropdownMenu :items="menu(genre)" :content="{ align: 'end' }">
              <UButton icon="i-lucide-ellipsis" color="neutral" variant="ghost" size="sm" :aria-label="`${genre.name}の操作`" />
            </UDropdownMenu>
          </div>
        </template>
        <p v-if="!summary(genre).roots.length" class="px-4 py-6 text-center text-sm text-(--ui-text-muted)">Root Taskなし</p>
        <ul v-else class="divide-y divide-(--ui-border-muted)">
          <li v-for="task in summary(genre).roots.slice(0, 6)" :key="task.id" class="flex items-center gap-3 px-4 py-2.5">
            <NuxtLink :to="`/tasks/${task.id}`" class="min-w-0 flex-1 truncate text-sm hover:underline">{{ task.title }}</NuxtLink>
            <FireMark v-if="task.showFire" />
            <UProgress :model-value="task.progress ?? 0" size="xs" class="w-20" :color="task.status === 'COMPLETED' ? 'success' : 'primary'" />
            <span class="w-9 text-right font-mono text-xs text-(--ui-text-muted) tabular-nums">{{ task.progress ?? 0 }}%</span>
          </li>
          <li v-if="summary(genre).roots.length > 6" class="px-4 py-2 text-xs text-(--ui-text-muted)">他 {{ summary(genre).roots.length - 6 }} 件</li>
        </ul>
        <template #footer>
          <div class="flex items-center justify-between">
            <UButton :to="{ path: '/tasks/new', query: { genreId: genre.id } }" icon="i-lucide-plus" size="sm" color="neutral" variant="ghost">追加</UButton>
            <UButton :to="`/dashboard/genres/${genre.id}`" trailing-icon="i-lucide-arrow-right" size="sm" variant="link">Dashboardへ</UButton>
          </div>
        </template>
      </UCard>
    </div>

    <GenreFormModal v-model:open="creating" />
    <GenreFormModal v-model:open="editOpen" :genre="editing" />
    <TaskTypeManager v-if="typesGenre" v-model:open="typesOpen" :genre="typesGenre" />
    <ConfirmDialog v-model:open="deleteOpen" :title="`ジャンル「${deleting?.name}」を削除しますか？`" description="この操作は取り消せません。" confirm-label="削除する" :loading="deleteBusy" @confirm="confirmDelete" />
  </div>
</template>

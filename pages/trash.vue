<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useSeoMeta } from '#imports'
import type { DeletedTask } from '~/frontend/layers/core/composables/useCriticalData'
import { useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import { apiErrorStatus } from '~/frontend/layers/core/composables/apiError'
import { formatDate } from '~/frontend/layers/core/composables/dates'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'
import { useNotify } from '~/frontend/layers/shared/composables/useNotify'

// 削除済みタスク（30日以内）の一覧と復元
useSeoMeta({ title: 'ゴミ箱 | タスマネ' })
const api = useTasmaneApi()
const notify = useNotify()
const { nameOf, genres, loaded } = useGenreList()
const tasks = ref<DeletedTask[]>([])
const loading = ref(true)
const loadError = ref<unknown>()
const busyId = ref('')

// 同じ削除操作で消えたタスクをまとめて表示（復元も一括）
const groups = computed(() => {
  const map = new Map<string, DeletedTask[]>()
  for (const task of tasks.value) map.set(task.deletedAt, [...(map.get(task.deletedAt) ?? []), task])
  return [...map.entries()].map(([deletedAt, items]) => ({ deletedAt, items, purgeAt: items[0]?.purgeAt ?? deletedAt }))
})
const daysLeft = (purgeAt: string) => Math.max(0, Math.ceil((new Date(purgeAt).getTime() - Date.now()) / 86_400_000))

// 一覧の読込
async function load() {
  loading.value = true
  loadError.value = undefined
  try {
    tasks.value = await api.deletedTasks()
    if (!loaded.value) { genres.value = await api.genres(); loaded.value = true }
  } catch (error) { loadError.value = error }
  finally { loading.value = false }
}
onMounted(load)

// 削除操作単位での復元（Google Tasks側も新規作成し再紐付け）
async function restore(task: DeletedTask) {
  busyId.value = task.id
  try {
    const result = await api.restoreTask(task.id)
    tasks.value = tasks.value.filter(item => !result.restoredTaskIds.includes(item.id))
    notify.success(`${result.restoredTaskIds.length}件のタスクを復元しました`)
  } catch (error) {
    if (apiErrorStatus(error) === 422) notify.error(error, '親タスクが完了済みのため復元できません。親タスクを再開してから復元してください。')
    else if (apiErrorStatus(error) === 409) notify.error(error, 'ジャンルが削除されているため復元できません。')
    else notify.error(error, '復元できませんでした。')
  } finally { busyId.value = '' }
}
</script>

<template>
  <section class="mx-auto max-w-4xl">
    <PageHeader eyebrow="Trash" title="ゴミ箱" description="削除したタスクは30日後に完全に削除されます。それまでは復元できます。" />
    <div v-if="loading" class="space-y-3" role="status" aria-label="読み込み中"><USkeleton v-for="index in 3" :key="index" class="h-20 rounded-xl" /></div>
    <ErrorState v-else-if="loadError" :error="loadError" fallback="削除済みタスクを取得できませんでした。" @retry="load" />
    <EmptyState v-else-if="!groups.length" icon="i-lucide-trash" title="ゴミ箱は空です" description="削除したタスクはここに30日間保管されます。" />
    <div v-else class="space-y-4">
      <UCard v-for="group in groups" :key="group.deletedAt" :ui="{ body: 'p-0 sm:p-0' }">
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <p class="text-sm font-medium text-(--ui-text-highlighted)">{{ formatDate(group.deletedAt, true) }} に削除</p>
              <p class="text-xs text-(--ui-text-muted)">あと{{ daysLeft(group.purgeAt) }}日で完全に削除 ・ {{ group.items.length }}件</p>
            </div>
            <UButton icon="i-lucide-undo-2" size="sm" :loading="busyId === group.items[0]?.id" @click="group.items[0] && restore(group.items[0])">まとめて復元</UButton>
          </div>
        </template>
        <ul class="divide-y divide-(--ui-border)">
          <li v-for="task in group.items" :key="task.id" class="flex items-center gap-3 px-4 py-2.5">
            <StatusBadge :value="task.status" />
            <span class="min-w-0 flex-1 truncate">{{ task.title }}</span>
            <span class="text-xs text-(--ui-text-muted)">{{ nameOf(task.genreId) || '' }}</span>
          </li>
        </ul>
      </UCard>
    </div>
  </section>
</template>

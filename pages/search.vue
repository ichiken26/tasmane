<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type { TableColumn } from '@nuxt/ui'
import { useRoute, useRouter, useSeoMeta } from '#imports'
import { encodeSearch, parseSearch, toSearchApiQuery } from '../frontend/layers/core/composables/taskState'
import { useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'
import { formatDate, relativeDeadline } from '~/frontend/layers/core/composables/dates'

useSeoMeta({ title: '検索 | タスマネ' })
const route = useRoute()
const router = useRouter()
const api = useTasmaneApi()
const { genres, nameOf } = useGenreList()
const ALL = '__all__'
const state = reactive(parseSearch(route.query))
const keyword = ref(state.q)
const results = ref<ApiTask[]>([])
const loading = ref(false)
const failed = ref<unknown>()
let request = 0

// フリーワードはdebounceして反映
let timer: ReturnType<typeof setTimeout> | undefined
watch(keyword, (value) => {
  clearTimeout(timer)
  timer = setTimeout(() => { state.q = value.trim() }, 300)
})

// 検索条件のURL反映と結果取得
async function run() {
  const query = encodeSearch(state)
  if (JSON.stringify(query) !== JSON.stringify(route.query)) await router.replace({ query })
  const current = ++request
  loading.value = true
  failed.value = undefined
  try {
    const found = await api.search(toSearchApiQuery(state))
    if (current === request) results.value = found
  } catch (error) {
    if (current === request) failed.value = error
  } finally {
    if (current === request) loading.value = false
  }
}
watch(state, run, { deep: true, immediate: true })
// URLクエリからの検索条件再同期（戻る・進む）
watch(() => route.query, (query) => {
  const restored = parseSearch(query)
  if (JSON.stringify(restored) !== JSON.stringify({ ...state })) { Object.assign(state, restored); keyword.value = restored.q }
})

// 選択肢（USelectは空文字を値にできないためセンチネルで「すべて」を表現）
const genreItems = computed(() => genres.value.map(genre => ({ label: genre.name, value: genre.id })))
const priority = computed({ get: () => state.priority || undefined, set: (value: string | undefined) => { state.priority = !value || value === ALL ? '' : value } })
const deadline = computed({ get: () => state.deadline || undefined, set: (value: string | undefined) => { state.deadline = !value || value === ALL ? '' : value } })
const priorityItems = [{ label: 'すべての優先度', value: ALL }, { label: '高', value: 'high' }, { label: '中', value: 'medium' }, { label: '低', value: 'low' }]
const sort = computed({ get: () => state.sort || undefined, set: (value: string | undefined) => { state.sort = !value || value === 'deadline' ? '' : value } })
const sortItems = [{ label: '期限が近い順', value: 'deadline' }, { label: '優先度順', value: 'priority' }, { label: 'おすすめ順（TaskScore）', value: 'taskScore' }]
const deadlineItems = [{ label: 'すべての期限', value: ALL }, { label: '期限切れ', value: 'overdue' }, { label: '今日まで', value: 'today' }, { label: '明日まで', value: 'tomorrow' }, { label: '1週間以内', value: 'week' }, { label: '1か月以内', value: 'month' }, { label: 'それ以上', value: 'later' }]
const hasFilter = computed(() => Boolean(state.q || state.genres.length || state.priority || state.deadline))
function clearAll() { keyword.value = ''; Object.assign(state, { q: '', genres: [], priority: '', deadline: '', sort: '' }) }

// 検索結果テーブル
const columns: TableColumn<ApiTask>[] = [
  { accessorKey: 'title', header: 'Task' },
  { accessorKey: 'genreId', header: 'ジャンル' },
  { accessorKey: 'status', header: '状態' },
  { accessorKey: 'effectivePriority', header: '優先度' },
  { accessorKey: 'endAt', header: '期限' },
  { accessorKey: 'progress', header: '進捗' },
]
</script>

<template>
  <!-- 検索画面 -->
  <div class="mx-auto max-w-7xl">
    <PageHeader eyebrow="Search" title="検索" description="条件はURLに保存されるため、ブックマークや共有ができます。" />
    <!-- 検索条件 -->
    <UCard class="mb-6">
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,2fr)_minmax(0,1.4fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.2fr)]">
        <UInput v-model="keyword" type="search" icon="i-lucide-search" placeholder="タイトル・メモを検索" aria-label="フリーワード" size="lg" :loading="loading" autofocus />
        <USelectMenu v-model="state.genres" :items="genreItems" value-key="value" multiple placeholder="すべてのジャンル（複数はOR）" aria-label="ジャンル" size="lg" />
        <USelect v-model="priority" :items="priorityItems" placeholder="すべての優先度" aria-label="優先度" size="lg" />
        <USelect v-model="deadline" :items="deadlineItems" placeholder="すべての期限" aria-label="期限" size="lg" />
        <USelect v-model="sort" :items="sortItems" placeholder="期限が近い順" aria-label="並び順" size="lg" icon="i-lucide-arrow-up-down" />
      </div>
      <div class="mt-3 flex min-h-7 items-center justify-between gap-2 text-sm text-(--ui-text-muted)">
        <span role="status">{{ loading ? '検索中…' : `${results.length} 件` }}</span>
        <UButton v-if="hasFilter" icon="i-lucide-x" size="xs" color="neutral" variant="ghost" @click="clearAll">条件をクリア</UButton>
      </div>
    </UCard>

    <ErrorState v-if="failed" :error="failed" fallback="検索できませんでした。" @retry="run" />
    <!-- 検索結果一覧 -->
    <UCard v-else :ui="{ body: 'p-0 sm:p-0' }">
      <UTable :data="results" :columns="columns" :loading="loading" class="w-full" :ui="{ td: 'py-2.5', th: 'text-xs' }">
        <template #title-cell="{ row }">
          <div class="flex max-w-md items-center gap-1.5">
            <NuxtLink :to="`/tasks/${row.original.id}`" class="truncate font-medium text-(--ui-text-highlighted) hover:underline">{{ row.original.title }}</NuxtLink>
            <FireMark v-if="row.original.showFire" />
            <UTooltip text="別タブで開く"><UButton :to="`/tasks/${row.original.id}`" target="_blank" icon="i-lucide-external-link" size="xs" color="neutral" variant="ghost" aria-label="別タブで開く" /></UTooltip>
          </div>
          <p v-if="row.original.body" class="max-w-md truncate text-xs text-(--ui-text-muted)">{{ row.original.body }}</p>
        </template>
        <template #genreId-cell="{ row }"><span class="text-sm whitespace-nowrap">{{ nameOf(row.original.genreId) || '—' }}</span></template>
        <template #status-cell="{ row }"><StatusBadge :value="row.original.status" /></template>
        <template #effectivePriority-cell="{ row }"><StatusBadge :value="row.original.effectivePriority" kind="priority" /></template>
        <template #endAt-cell="{ row }">
          <span v-if="row.original.endAt" class="text-sm whitespace-nowrap" :class="{ 'text-(--ui-error)': row.original.isOverdue }">{{ formatDate(row.original.endAt) }}<span v-if="row.original.status !== 'COMPLETED'" class="ml-1 text-xs text-(--ui-text-muted)">{{ relativeDeadline(row.original.endAt) }}</span></span>
          <span v-else class="text-(--ui-text-dimmed)">—</span>
        </template>
        <template #progress-cell="{ row }">
          <div class="flex items-center gap-2"><UProgress :model-value="row.original.progress ?? 0" size="xs" class="w-16" /><span class="font-mono text-xs tabular-nums">{{ row.original.progress ?? 0 }}%</span></div>
        </template>
        <template #empty>
          <EmptyState class="m-4 border-0" icon="i-lucide-search-x" :title="hasFilter ? '条件に一致するタスクはありません' : 'タスクがありません'" :description="hasFilter ? '条件を変えるか、クリアして再検索してください。' : undefined" />
        </template>
      </UTable>
    </UCard>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useSeoMeta } from '#imports'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'
import { deadlineSection } from '~/frontend/layers/core/composables/taskState'
import { useNotify } from '~/frontend/layers/shared/composables/useNotify'

useSeoMeta({ title: 'トップ | タスマネ', description: '次にやるべきタスクをすぐ把握' })
const api = useTasmaneApi()
const router = useRouter()
const notify = useNotify()
const { genres } = useGenreList()
// トップ用ダッシュボードデータの取得
const { data, status, error, refresh } = useCriticalData('top-dashboard', () => api.dashboard())

// 次タスク（TaskScore順の上位）と今日の状況
const open = computed(() => (data.value?.tasks ?? []).filter(task => task.status !== 'COMPLETED'))
const nextTasks = computed(() => [...open.value].filter(task => task.recommended !== false).sort((a, b) => (b.taskScore || 0) - (a.taskScore || 0)).slice(0, 5))
const overdue = computed(() => open.value.filter(task => task.showFire).length)
const today = computed(() => open.value.filter(task => task.endAt && deadlineSection(task.endAt.slice(0, 10)) === 'today').length)
const inProgress = computed(() => open.value.filter(task => task.status === 'IN_PROGRESS').length)
const greeting = computed(() => {
  const hour = new Date().getHours()
  return hour < 11 ? 'Good Morning' : hour < 18 ? 'Hi' : 'Take care'
})
const todayLabel = new Intl.DateTimeFormat('ja-JP', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date())

// クイック追加
const quickTitle = ref('')
const quickGenre = ref<string>()
const adding = ref(false)
const genreItems = computed(() => genres.value.map(genre => ({ label: genre.name, value: genre.id })))
async function quickAdd() {
  const title = quickTitle.value.trim()
  const genreId = quickGenre.value ?? genres.value[0]?.id
  if (!title) return
  if (!genreId) { await router.push('/tasks/new'); return }
  adding.value = true
  try {
    const task = await api.createTask({ title, genreId, isAllDay: true })
    quickTitle.value = ''
    notify.success('タスクを追加しました', task.title)
    await refresh()
  } catch (cause) { notify.error(cause, 'タスクを追加できませんでした。') }
  finally { adding.value = false }
}
</script>

<template>
  <div class="mx-auto max-w-5xl space-y-8">
    <!-- ヒーロー -->
    <section>
      <p class="font-mono text-xs tracking-wider text-(--ui-text-muted) uppercase">{{ todayLabel }}</p>
      <h1 class="mt-1 font-serif text-4xl leading-tight text-(--ui-text-highlighted) sm:text-5xl">{{ greeting }}, <br class="sm:hidden">NOW TASKS BELOW</h1>
    </section>

    <!-- クイック追加 -->
    <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="quickAdd">
      <UInput v-model="quickTitle" size="lg" icon="i-lucide-plus" placeholder="タスクをすばやく追加（Enterで登録）" class="flex-1" aria-label="追加するタスクのタイトル" />
      <USelectMenu v-if="genres.length" v-model="quickGenre" :items="genreItems" value-key="value" size="lg" :placeholder="genres[0]?.name" class="sm:w-48" aria-label="ジャンル" />
      <UButton type="submit" size="lg" :loading="adding" :disabled="!quickTitle.trim()">追加</UButton>
    </form>

    <ErrorState v-if="error" :error="error" fallback="データを取得できませんでした。" @retry="refresh()" />
    <template v-else>
      <!-- 焦点サマリー -->
      <div class="grid grid-cols-3 gap-3">
        <StatCard label="期限切れ" :value="status === 'pending' ? '—' : overdue" icon="i-lucide-flame" :tone="overdue ? 'error' : 'neutral'" />
        <StatCard label="今日が期限" :value="status === 'pending' ? '—' : today" icon="i-lucide-calendar-check" :tone="today ? 'warning' : 'neutral'" />
        <StatCard label="進行中" :value="status === 'pending' ? '—' : inProgress" icon="i-lucide-circle-dot" tone="info" />
      </div>

      <!-- 次にやること -->
      <UCard :ui="{ body: 'p-0 sm:p-0' }">
        <template #header>
          <div class="flex items-center justify-between">
            <h2 class="font-semibold text-(--ui-text-highlighted)">次にやること</h2>
            <UButton to="/dashboard" variant="link" trailing-icon="i-lucide-arrow-right" size="sm">Dashboardで全て見る</UButton>
          </div>
        </template>
        <div v-if="status === 'pending'" class="space-y-3 p-4"><USkeleton v-for="index in 4" :key="index" class="h-12 w-full" /></div>
        <ol v-else-if="nextTasks.length" class="divide-y divide-(--ui-border)">
          <li v-for="(task, index) in nextTasks" :key="task.id"><TaskRow :task="task" :rank="index + 1" reasons /></li>
        </ol>
        <div v-else class="p-4">
          <EmptyState icon="i-lucide-coffee" title="着手候補はありません" description="上の入力欄からタスクを追加してみましょう。" />
        </div>
      </UCard>
    </template>
  </div>
</template>

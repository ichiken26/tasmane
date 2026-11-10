<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { definePageMeta, useRoute, useSeoMeta } from '#imports'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiDependency, ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
import { useEntityChannel } from '~/frontend/layers/core/composables/useEntityChannel'
import { useGenreList } from '~/frontend/layers/core/composables/useGenreList'
import { formatDate, relativeDeadline } from '~/frontend/layers/core/composables/dates'
import { useNotify } from '~/frontend/layers/shared/composables/useNotify'
import { useMemoIntegration } from '~/frontend/layers/memo/composables/useMemoIntegration'
import { useWideScreen } from '~/frontend/layers/memo/composables/useWideScreen'
import { useMemoPanelWidth } from '~/frontend/layers/memo/composables/useMemoPanelWidth'

// タスク間の遷移でページを再生成
definePageMeta({ key: route => route.path })
const route = useRoute()
const api = useTasmaneApi()
const { nameOf } = useGenreList()
const notify = useNotify()
// メモ表示中は右カラムをメモに切り替え（狭い画面はスライドオーバー）
const { openedMemo, closeMemo } = useMemoIntegration()
const wide = useWideScreen()
const memoWidth = useMemoPanelWidth()
const memoBeside = computed(() => Boolean(openedMemo.value) && wide.value)
onBeforeUnmount(() => closeMemo())
const showGoogleConflict = computed(() => route.query.googleConflict === '1')
const taskId = String(route.params.taskId)
const tree = ref<ApiTask[]>([])
const dependencies = ref<ApiDependency[]>([])
const relationsLoading = ref(true)
// タスク本体（Critical）→ 描画後にツリー・依存をPrefetch
const { data: task, status, error, refresh } = useCriticalData(`task-${taskId}`, () => api.task(taskId), () => reloadRelations())
useSeoMeta({ title: () => `${task.value?.title || 'Task'} | タスマネ` })

// 他タブとのエンティティ共有（versionで新旧比較）
const channel = useEntityChannel('task', task as unknown as import('vue').Ref<ApiTask | undefined>)
let loadedOnce = false
watch(task, (value) => {
  if (value) { loadedOnce = true; return }
  if (loadedOnce) void refresh()
}, { immediate: true })

// 階層表示用行の整形
const treeRows = computed(() => tree.value.map(item => ({
  id: item.id, title: item.title, progress: item.progress ?? 0, overdue: item.showFire, parentTaskId: item.parentTaskId, status: item.status,
})))
// パンくず用の祖先
const ancestors = computed(() => {
  const byId = new Map(tree.value.map(item => [item.id, item]))
  const result: ApiTask[] = []
  let current = task.value?.parentTaskId ? byId.get(task.value.parentTaskId) : undefined
  while (current && result.length < 20) { result.unshift(current); current = current.parentTaskId ? byId.get(current.parentTaskId) : undefined }
  return result
})
const breadcrumb = computed(() => [
  ...(task.value ? [{ label: nameOf(task.value.genreId) || 'ジャンル', icon: 'i-lucide-folder', to: `/dashboard/genres/${task.value.genreId}` }] : []),
  ...ancestors.value.map(item => ({ label: item.title, to: `/tasks/${item.id}` })),
  { label: task.value?.title || '' },
])
// 全子孫完了時の「完了可能」表示
const descendants = computed(() => {
  const result: ApiTask[] = []
  const walk = (id: string) => { for (const child of tree.value.filter(item => item.parentTaskId === id)) { result.push(child); walk(child.id) } }
  if (task.value) walk(task.value.id)
  return result
})
// サーバー算出（未完了子孫・未確認の確認事項なし）を優先
const completable = computed(() => task.value?.canComplete ?? (task.value?.status !== 'COMPLETED' && descendants.value.length > 0 && descendants.value.every(item => item.status === 'COMPLETED')))
const isParent = computed(() => descendants.value.length > 0 || task.value?.isLeaf === false)

// ツリーと依存関係の再取得
async function reloadRelations() {
  relationsLoading.value = true
  try { [tree.value, dependencies.value] = await Promise.all([api.tree(taskId), api.dependencies(taskId)]) }
  catch { tree.value = []; dependencies.value = [] }
  finally { relationsLoading.value = false }
  if (task.value) channel.publish(task.value)
}
// 保存後のタスク差し替え
function replaceTask(saved: ApiTask) {
  // 依存関係と日程の矛盾は保存した上で警告
  if (saved.warnings?.length) notify.warning('依存関係と日程が矛盾しています', saved.warnings.join(' '))
  task.value = saved
  channel.publish(saved)
  void reloadRelations()
}
</script>

<template>
  <div class="mx-auto max-w-7xl">
    <div v-if="status === 'pending' && !task" class="space-y-4" role="status" aria-label="Taskを読み込み中">
      <USkeleton class="h-5 w-64" /><USkeleton class="h-9 w-96 max-w-full" />
      <div class="grid gap-6 lg:grid-cols-[1fr_400px]"><USkeleton class="h-[480px] rounded-xl" /><USkeleton class="h-[480px] rounded-xl" /></div>
    </div>
    <ErrorState v-else-if="error" :error="error" fallback="Taskを取得できませんでした。" @retry="refresh()" />
    <template v-else-if="task">
      <!-- タスク詳細と編集 -->
      <UBreadcrumb :items="breadcrumb" class="mb-3" />
      <PageHeader :title="task.title">
        <template #title>{{ task.title }} <FireMark v-if="task.showFire" /></template>
        <template #meta>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <StatusBadge :value="task.status" size="md" />
            <StatusBadge :value="task.effectivePriority" kind="priority" size="md" />
            <UBadge v-if="!task.manualPriority" label="優先度は自動" color="neutral" variant="outline" size="md" />
            <UBadge v-if="task.endAt" :label="`期限 ${formatDate(task.endAt, !task.isAllDay)}（${relativeDeadline(task.endAt)}）`" :color="task.isOverdue ? 'error' : 'neutral'" variant="subtle" icon="i-lucide-calendar-clock" size="md" />
            <LeafProgressControl v-if="task.status === 'IN_PROGRESS' && task.isLeaf !== false && !isParent" :task="task" @updated="replaceTask" />
            <div v-else class="flex items-center gap-2 text-sm text-(--ui-text-muted)">
              <UProgress :model-value="task.progress ?? 0" size="sm" class="w-28" :color="task.status === 'COMPLETED' ? 'success' : 'primary'" />
              <span class="tabular-nums">{{ task.progress ?? 0 }}%</span>
              <span v-if="isParent" class="text-xs">（子タスクから自動計算）</span>
            </div>
          </div>
        </template>
        <template #actions>
          <UButton v-if="task.status !== 'COMPLETED'" :to="{ path: '/tasks/new', query: { genreId: task.genreId, parentTaskId: task.id } }" icon="i-lucide-list-plus" color="neutral" variant="outline">子タスクを追加</UButton>
          <TaskLifecycle :task="task" @updated="replaceTask" />
        </template>
      </PageHeader>

      <GoogleConflictResolver v-if="showGoogleConflict" :task="task" @resolved="replaceTask" />
      <UAlert v-if="completable && task.status === 'IN_PROGRESS'" class="mb-6" color="success" variant="subtle" icon="i-lucide-party-popper" title="完了可能" :description="isParent ? 'すべての子タスクが完了しました。内容を確認して、このタスクを完了にできます。' : '未確認の確認事項はありません。作業が終わっていれば完了にできます。'" />

      <div class="grid gap-6" :class="memoBeside ? '' : 'lg:grid-cols-[minmax(0,1fr)_400px]'" :style="memoBeside ? { gridTemplateColumns: `minmax(0,1fr) ${memoWidth.size.value}px` } : undefined">
        <div class="min-w-0 space-y-6">
          <TaskEditor :task="task" @saved="replaceTask" />
          <ConfirmationPanel :task-id="task.id" :completed="task.status === 'COMPLETED'" />
        </div>
        <!-- メモ表示中はメモ、それ以外は階層と依存グラフ -->
        <MemoSidePanel v-if="memoBeside" resizable class="sticky top-20 h-[calc(100dvh-8.75rem)]" />
        <div v-else class="min-w-0 space-y-6">
          <HierarchyTree :tasks="treeRows" :current-id="task.id" :loading="relationsLoading && !tree.length" />
          <DependencyGraph :task="task" :dependencies="dependencies" @changed="replaceTask" />
        </div>
      </div>
      <USlideover :open="Boolean(openedMemo) && !wide" title="メモ" side="right" :ui="{ content: 'max-w-full sm:max-w-2xl' }" @update:open="(value: boolean) => { if (!value) closeMemo() }">
        <template #body><MemoSidePanel class="h-full" /></template>
      </USlideover>
    </template>
  </div>
</template>

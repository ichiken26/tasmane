<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { ApiDependency, ApiTask } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useNotify } from '../../shared/composables/useNotify'

// Dependency Graph: task_dependenciesによる作業前提（Hierarchyとは別概念）
const props = defineProps<{ task: ApiTask; dependencies: ApiDependency[] }>()
const emit = defineEmits<{ changed: [task: ApiTask] }>()
const api = useTasmaneApi()
const notify = useNotify()
const genreTasks = ref<ApiTask[]>([])
const dependsOnTaskId = ref<string>()
const busy = ref(false)

// 同一ジャンルのタスク（依存は同一Genre内のみ）
async function loadCandidates() {
  try { genreTasks.value = await api.search({ genreIds: props.task.genreId }) }
  catch { genreTasks.value = [] }
}
onMounted(loadCandidates)
watch(() => props.task.genreId, loadCandidates)

const byId = computed(() => new Map(genreTasks.value.map(task => [task.id, task])))
const multiple = computed(() => props.dependencies.length > 1)
const candidates = computed(() => genreTasks.value
  .filter(task => task.id !== props.task.id && !props.dependencies.some(edge => edge.dependsOnTaskId === task.id))
  .map(task => ({ label: task.title, value: task.id, description: task.status === 'COMPLETED' ? '完了済み' : undefined })))
const blocking = computed(() => props.dependencies.filter(edge => byId.value.get(edge.dependsOnTaskId)?.status !== 'COMPLETED').length)

// 依存関係の追加
async function addDependency(id?: string) {
  if (!id || busy.value) return
  busy.value = true
  try {
    await api.addDependency(props.task.id, id, props.task.version)
    dependsOnTaskId.value = undefined
    emit('changed', await api.task(props.task.id))
  } catch (error) {
    dependsOnTaskId.value = undefined
    notify.error(error, '依存を追加できませんでした。循環になっていないか確認してください。')
  } finally {
    busy.value = false
  }
}

// 依存関係の削除
async function removeDependency(dependsOn: string) {
  if (busy.value) return
  busy.value = true
  try {
    await api.removeDependency(props.task.id, dependsOn, props.task.version)
    emit('changed', await api.task(props.task.id))
  } catch (error) {
    notify.error(error, '依存を削除できませんでした。')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UCard :ui="{ body: 'p-2 sm:p-2' }">
    <template #header>
      <div class="flex items-center justify-between gap-2">
        <div>
          <h2 class="flex items-center gap-2 font-semibold text-(--ui-text-highlighted)"><UIcon name="i-lucide-workflow" class="size-5 text-(--ui-text-muted)" />Dependency Graph</h2>
          <p class="text-xs text-(--ui-text-muted)">このタスクの前に終わらせたいタスク（作業前提）</p>
        </div>
        <UBadge v-if="blocking" :label="`未完了の前提 ${blocking}`" color="warning" variant="subtle" icon="i-lucide-octagon-alert" />
      </div>
    </template>
    <UAlert v-if="multiple" color="warning" variant="subtle" icon="i-lucide-git-merge" title="複数のタスクに依存しています" class="mb-2" />
    <!-- 依存一覧 -->
    <ul v-if="dependencies.length" class="space-y-1">
      <li
        v-for="edge in dependencies"
        :key="`${edge.taskId}:${edge.dependsOnTaskId}`"
        class="flex items-center gap-2 rounded-md px-2 py-1.5"
        :class="{ 'multi-dependency': multiple }"
      >
        <UIcon name="i-lucide-arrow-left-to-line" class="size-4 shrink-0 text-(--ui-text-dimmed)" aria-label="前提" />
        <NuxtLink :to="`/tasks/${edge.dependsOnTaskId}`" class="min-w-0 flex-1 truncate text-sm hover:underline">
          {{ byId.get(edge.dependsOnTaskId)?.title || '読み込み中…' }}
        </NuxtLink>
        <StatusBadge :value="byId.get(edge.dependsOnTaskId)?.status" />
        <UTooltip text="依存を解除">
          <UButton icon="i-lucide-unlink" size="xs" color="error" variant="ghost" :disabled="busy" aria-label="依存を解除" @click="removeDependency(edge.dependsOnTaskId)" />
        </UTooltip>
      </li>
    </ul>
    <p v-else class="px-2 py-3 text-sm text-(--ui-text-muted)">前提となるタスクはありません。</p>
    <template #footer>
      <!-- 依存追加 -->
      <USelectMenu
        v-model="dependsOnTaskId"
        :items="candidates"
        value-key="value"
        :loading="busy"
        icon="i-lucide-plus"
        placeholder="前提タスクを追加…"
        :search-input="{ placeholder: '同じジャンルのタスクを検索…' }"
        class="w-full"
        aria-label="前提タスクを追加"
        @update:model-value="(value: string | undefined) => addDependency(value)"
      />
    </template>
  </UCard>
</template>

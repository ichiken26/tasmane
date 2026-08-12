<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiDependency, ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
const route = useRoute()
const api = useTasmaneApi()
const taskId = computed(() => String(route.params.taskId))
const tree = ref<ApiTask[]>([])
const dependencies = ref<ApiDependency[]>([])
const { data: task, status, error } = useCriticalData(`task-${taskId.value}`, () => api.task(taskId.value), async () => {
  [tree.value, dependencies.value] = await Promise.all([api.tree(taskId.value), api.dependencies(taskId.value)])
})
useSeoMeta({ title: () => `${task.value?.title || 'Task'} | タスマネ` })
const treeRows = computed(() => tree.value.map(item => ({ id: item.id, title: item.title, progress: item.progress ?? 0, overdue: item.showFire })))
const dependencyRows = computed(() => task.value ? [{ id: task.value.id, title: task.value.title, dependencies: dependencies.value.map(item => item.dependsOnTaskId) }] : [])
</script>
<template><div class="page"><p v-if="status === 'pending'" role="status">Taskを読み込み中…</p><p v-else-if="error" role="alert">Taskを取得できませんでした。</p><template v-else-if="task"><span class="eyebrow">Task / version {{ task.version }}</span><h1>{{ task.title }}</h1><TaskEditor :task="{ title: task.title, body: task.body || undefined, genre: task.genreId, version: task.version }"/><div class="grid two"><HierarchyTree :tasks="treeRows"/><DependencyGraph :tasks="dependencyRows"/></div></template></div></template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ApiDependency, ApiTask } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'

const props = defineProps<{ task: ApiTask; dependencies: ApiDependency[] }>()
const emit = defineEmits<{ changed: [task: ApiTask] }>()
const api = useTasmaneApi()
const dependsOnTaskId = ref('')
const errorMessage = ref('')
const busy = ref(false)

async function addDependency() {
  if (!dependsOnTaskId.value || busy.value) return
  busy.value = true
  errorMessage.value = ''
  try {
    await api.addDependency(props.task.id, dependsOnTaskId.value, props.task.version)
    const saved = await api.task(props.task.id)
    dependsOnTaskId.value = ''
    emit('changed', saved)
  } catch {
    errorMessage.value = '依存を追加できませんでした。循環やジャンル境界を確認してください。'
  } finally {
    busy.value = false
  }
}

async function removeDependency(dependsOn: string) {
  if (busy.value) return
  busy.value = true
  errorMessage.value = ''
  try {
    await api.removeDependency(props.task.id, dependsOn, props.task.version)
    emit('changed', await api.task(props.task.id))
  } catch {
    errorMessage.value = '依存を削除できませんでした。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card">
    <h2>Dependency Graph</h2>
    <p class="muted">Hierarchyとは異なる作業前提の参照です。</p>
    <p v-if="errorMessage" role="alert">{{ errorMessage }}</p>
    <ul>
      <li
        v-for="edge in dependencies"
        :key="`${edge.taskId}:${edge.dependsOnTaskId}`"
        :class="{ 'multi-dependency': dependencies.length > 1 }"
      >
        <strong>{{ task.title }}</strong>
        <span> ← {{ edge.dependsOnTaskId }}</span>
        <button type="button" class="danger" :disabled="busy" @click="removeDependency(edge.dependsOnTaskId)">削除</button>
      </li>
      <li v-if="!dependencies.length">前提なし</li>
    </ul>
    <form class="form compact" @submit.prevent="addDependency">
      <label>
        依存先 Task ID
        <input v-model="dependsOnTaskId" required placeholder="uuid" />
      </label>
      <button class="primary" type="submit" :disabled="busy">依存を追加</button>
    </form>
  </section>
</template>

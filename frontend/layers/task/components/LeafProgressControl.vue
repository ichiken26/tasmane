<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import type { ApiTask } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useNotify } from '../../shared/composables/useNotify'

// 葉タスクの進捗率（進行中のみ 0〜95%・5刻み。完了で100%、親は子から自動算出）
const props = defineProps<{ task: ApiTask }>()
const emit = defineEmits<{ updated: [task: ApiTask] }>()
const api = useTasmaneApi()
const notify = useNotify()
const value = ref(props.task.progress ?? 0)
const saving = ref(false)
const container = ref<HTMLElement>()
// Nuxt UI Slider はつまみ（role=slider）へラベルを渡さないため、支援技術向けに付与
onMounted(() => { container.value?.querySelector('[role=slider]')?.setAttribute('aria-label', '進捗率') })
watch(() => props.task.progress, (progress) => { value.value = progress ?? 0 })
// 操作が止まってから保存（ドラッグ中の連続送信を避ける）
let timer: ReturnType<typeof setTimeout> | undefined
watch(value, (next) => {
  clearTimeout(timer)
  timer = setTimeout(() => { void commit(next) }, 500)
})

// 確定値の保存（ドラッグ終了時）
async function commit(progress: number) {
  if (progress === (props.task.progress ?? 0)) return
  saving.value = true
  try {
    emit('updated', await api.setProgress(props.task.id, progress, props.task.version))
  } catch (error) {
    value.value = props.task.progress ?? 0
    notify.error(error, '進捗を更新できませんでした。最新の状態を読み込み直してください。')
  } finally { saving.value = false }
}
</script>

<template>
  <div ref="container" class="flex items-center gap-3">
    <USlider
      v-model="value"
      :min="0"
      :max="95"
      :step="5"
      :disabled="saving"
      class="w-40"
      aria-label="進捗率"
    />
    <span class="w-10 text-sm tabular-nums text-(--ui-text-muted)">{{ value }}%</span>
    <UIcon v-if="saving" name="i-lucide-loader-circle" class="size-4 animate-spin text-(--ui-text-muted)" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { confirmationMeta, priorityMeta, statusMeta } from '../../core/composables/labels'

// 状態・優先度・確認状態の共通バッジ
const props = withDefaults(defineProps<{ value?: string | null; kind?: 'status' | 'priority' | 'confirmation'; size?: 'sm' | 'md' }>(), { value: '', kind: 'status', size: 'sm' })
const meta = computed(() => {
  if (!props.value) return undefined
  if (props.kind === 'priority') return { ...priorityMeta(props.value), label: `優先度 ${priorityMeta(props.value).label}` }
  if (props.kind === 'confirmation') return confirmationMeta(props.value)
  return statusMeta(props.value)
})
</script>

<template>
  <UBadge v-if="meta" :label="meta.label" :icon="meta.icon" :color="meta.color" variant="subtle" :size="size" class="whitespace-nowrap" />
</template>

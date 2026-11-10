<script setup lang="ts">
import { computed, ref } from 'vue'
import { parseDate } from '@internationalized/date'
import type { DateValue } from '@internationalized/date'
import { inputDatePart, withPickedDate } from '../../core/composables/dates'

// 日付／日時入力（自由入力＋カレンダーから選択）
const props = withDefaults(defineProps<{ withTime?: boolean; defaultTime?: string; ariaLabel?: string }>(), { withTime: false, defaultTime: '09:00', ariaLabel: undefined })
const model = defineModel<string>({ default: '' })
const open = ref(false)

// カレンダーの選択値（入力値の日付部分）
const picked = computed(() => {
  const part = inputDatePart(model.value)
  return part ? parseDate(part) : undefined
})

// カレンダーで選んだ日付を反映（時刻は保持）
function pick(value: DateValue | DateValue[] | { start?: DateValue; end?: DateValue } | null | undefined) {
  if (!value || Array.isArray(value) || !('calendar' in value)) return
  model.value = withPickedDate(model.value, value.toString(), props.withTime, props.defaultTime)
  open.value = false
}
</script>

<template>
  <UInput v-model="model" :type="withTime ? 'datetime-local' : 'date'" :aria-label="ariaLabel" class="date-field w-full" :ui="{ trailing: 'pe-1' }">
    <template #trailing>
      <UPopover v-model:open="open">
        <UButton color="neutral" variant="link" size="sm" icon="i-lucide-calendar" aria-label="カレンダーから選ぶ" />
        <template #content>
          <UCalendar :model-value="picked" :default-placeholder="picked" class="p-2" @update:model-value="pick" />
        </template>
      </UPopover>
    </template>
  </UInput>
</template>

<style scoped>
/* ブラウザ標準のピッカーアイコンはカレンダーボタンと重複するため隠す */
.date-field :deep(input::-webkit-calendar-picker-indicator) {
  display: none;
}
</style>

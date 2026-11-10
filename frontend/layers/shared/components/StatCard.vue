<script setup lang="ts">
// 指標カード
withDefaults(defineProps<{ label: string; value: string | number; icon: string; hint?: string; tone?: 'primary' | 'error' | 'warning' | 'success' | 'info' | 'neutral' }>(), { hint: undefined, tone: 'neutral' })
const toneClass: Record<string, string> = {
  primary: 'bg-(--ui-primary)/12 text-(--ui-primary)', error: 'bg-(--ui-error)/12 text-(--ui-error)', warning: 'bg-(--ui-warning)/12 text-(--ui-warning)',
  success: 'bg-(--ui-success)/12 text-(--ui-success)', info: 'bg-(--ui-info)/12 text-(--ui-info)', neutral: 'bg-(--ui-bg-accented) text-(--ui-text-toned)',
}
</script>

<template>
  <UCard :ui="{ body: 'p-4 sm:p-5' }">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="text-sm text-(--ui-text-muted)">{{ label }}</p>
        <p class="mt-1 text-3xl font-semibold tracking-tight text-(--ui-text-highlighted) tabular-nums">{{ value }}</p>
      </div>
      <span class="grid size-10 shrink-0 place-items-center rounded-lg" :class="toneClass[tone]"><UIcon :name="icon" class="size-5" /></span>
    </div>
    <div v-if="hint || $slots.default" class="mt-2 text-xs text-(--ui-text-muted)"><slot>{{ hint }}</slot></div>
  </UCard>
</template>

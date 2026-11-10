<script setup lang="ts">
// ページ見出し
withDefaults(defineProps<{ eyebrow?: string; title: string; description?: string }>(), { eyebrow: undefined, description: undefined })
</script>

<template>
  <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
    <div class="min-w-0">
      <p v-if="eyebrow" class="font-mono text-xs tracking-wider text-(--ui-text-muted) uppercase">{{ eyebrow }}</p>
      <h1 class="mt-1 text-2xl font-semibold tracking-tight text-pretty text-(--ui-text-highlighted) sm:text-3xl">
        <slot name="title">{{ title }}</slot>
      </h1>
      <p v-if="description" class="mt-1 text-(--ui-text-muted)">{{ description }}</p>
      <slot name="meta" />
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 flex-wrap items-center gap-2"><slot name="actions" /></div>
  </div>
</template>

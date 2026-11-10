<script setup lang="ts">
// 空状態の表示
withDefaults(defineProps<{ icon?: string; title: string; description?: string }>(), { icon: 'i-lucide-inbox', description: undefined })
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-(--ui-border) px-6 py-10 text-center">
    <UIcon :name="icon" class="size-8 text-(--ui-text-dimmed)" />
    <p class="font-medium text-(--ui-text-highlighted)">{{ title }}</p>
    <p v-if="description" class="max-w-sm text-sm text-(--ui-text-muted)">{{ description }}</p>
    <div v-if="$slots.default" class="mt-2"><slot /></div>
  </div>
</template>

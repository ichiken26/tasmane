<script setup lang="ts">
import { onBeforeUnmount } from 'vue'
import { useMemoIntegration } from '../composables/useMemoIntegration'
import { useWideScreen } from '../composables/useWideScreen'
import { useMemoPanelWidth } from '../composables/useMemoPanelWidth'

// メモ表示中はメイン領域を左へ寄せ、右側にメモを表示（狭い画面ではスライドオーバー）
withDefaults(defineProps<{ narrowClass?: string }>(), { narrowClass: 'mx-auto max-w-3xl' })
const { openedMemo, closeMemo } = useMemoIntegration()
const wide = useWideScreen()
const width = useMemoPanelWidth()
onBeforeUnmount(() => closeMemo())
</script>

<template>
  <div v-if="openedMemo && wide" class="grid gap-6" :style="{ gridTemplateColumns: `minmax(0,1fr) ${width.size.value}px` }">
    <div class="min-w-0"><slot /></div>
    <MemoSidePanel resizable class="sticky top-20 h-[calc(100dvh-8.75rem)]" />
  </div>
  <div v-else :class="narrowClass">
    <slot />
    <USlideover :open="Boolean(openedMemo) && !wide" title="メモ" side="right" :ui="{ content: 'max-w-full sm:max-w-2xl' }" @update:open="(value: boolean) => { if (!value) closeMemo() }">
      <template #body><MemoSidePanel class="h-full" /></template>
    </USlideover>
  </div>
</template>

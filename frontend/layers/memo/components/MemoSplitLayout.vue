<script setup lang="ts">
import { computed, onBeforeUnmount } from 'vue'
import { useMemoIntegration } from '../composables/useMemoIntegration'
import { useWideScreen } from '../composables/useWideScreen'
import { useMemoPanelWidth } from '../composables/useMemoPanelWidth'

// メモ表示中はメイン領域を左へ寄せ、右側にメモを表示（狭い画面ではスライドオーバー）
withDefaults(defineProps<{ narrowClass?: string }>(), { narrowClass: 'mx-auto max-w-3xl' })
const { openedMemo, closeMemo } = useMemoIntegration()
const wide = useWideScreen()
const width = useMemoPanelWidth()
// 広い画面でメモを開いているときは左右分割
const split = computed(() => Boolean(openedMemo.value) && wide.value)
onBeforeUnmount(() => closeMemo())
</script>

<template>
  <!-- スロット（作成・編集フォーム）は常に同じ位置に置き、メモの開閉で作り直さない（未保存の入力・紐付けを保つ） -->
  <div :class="split ? 'grid gap-6' : narrowClass" :style="split ? { gridTemplateColumns: `minmax(0,1fr) ${width.size.value}px` } : undefined">
    <div class="min-w-0"><slot /></div>
    <MemoSidePanel v-if="split" resizable class="sticky top-20 h-[calc(100dvh-8.75rem)]" />
    <USlideover :open="Boolean(openedMemo) && !wide" title="メモ" side="right" :ui="{ content: 'max-w-full sm:max-w-2xl' }" @update:open="(value: boolean) => { if (!value) closeMemo() }">
      <template #body><MemoSidePanel class="h-full" /></template>
    </USlideover>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { memoEmbedUrl } from '../composables/memoBridge'
import { useMemoIntegration } from '../composables/useMemoIntegration'
import { useMemoPanelWidth } from '../composables/useMemoPanelWidth'

// 右側に表示するメモ（チップをクリックしたとき）
const { openedMemo, closeMemo, memoOrigin, reportSaved } = useMemoIntegration()
withDefaults(defineProps<{ resizable?: boolean }>(), { resizable: false })
const width = useMemoPanelWidth()
const src = computed(() => openedMemo.value ? memoEmbedUrl(openedMemo.value.url, memoOrigin.value) : null)
</script>

<template>
  <aside v-if="openedMemo" class="relative flex h-full min-h-0 flex-col gap-3" aria-label="メモ">
    <!-- 幅変更ハンドル（左端をドラッグ／矢印キー） -->
    <div
      v-if="resizable"
      role="separator"
      aria-orientation="vertical"
      aria-label="メモ表示枠の幅を変更"
      :aria-valuenow="width.size.value"
      :aria-valuemin="width.min"
      tabindex="0"
      class="group absolute inset-y-0 -left-4 z-10 flex w-3 cursor-col-resize touch-none items-center justify-center rounded focus-visible:outline-2 focus-visible:outline-(--ui-primary)"
      @pointerdown="width.onPointerDown"
      @keydown="width.onKeydown"
    >
      <span class="h-10 w-1 rounded-full bg-(--ui-border-accented) group-hover:bg-(--ui-primary)" />
    </div>
    <div class="flex items-center gap-2">
      <UIcon name="i-lucide-sticky-note" class="size-4 shrink-0 text-(--ui-text-muted)" />
      <h2 class="min-w-0 flex-1 truncate font-semibold text-(--ui-text-highlighted)">{{ openedMemo.title }}</h2>
      <UButton :to="openedMemo.url" target="_blank" external icon="i-lucide-external-link" color="neutral" variant="ghost" size="sm" aria-label="別タブで開く" />
      <UButton icon="i-lucide-x" color="neutral" variant="ghost" size="sm" aria-label="メモを閉じる" @click="closeMemo()" />
    </div>
    <MemoAppFrame v-if="src" :key="src" :src="src" :title="openedMemo.title" height="100%" class="min-h-0 flex-1" @saved="reportSaved($event); openedMemo = { url: $event.url, title: $event.title }" />
    <UAlert v-else color="neutral" variant="subtle" icon="i-lucide-info" title="このリンクはアプリ内で表示できません" :actions="[{ label: '別タブで開く', to: openedMemo.url, target: '_blank' }]" />
  </aside>
</template>

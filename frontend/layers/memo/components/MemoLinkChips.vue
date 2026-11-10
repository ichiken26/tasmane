<script setup lang="ts">
import { opensInNewTab } from '../composables/memoBridge'
import type { MemoLinkDraft } from '../composables/memoBridge'

// 紐付けたメモのリンクチップ
// - クリック: アプリ内（右側パネル）で表示
// - Ctrl/⌘/Shift/中クリック: ブラウザ標準どおり別タブ
const props = defineProps<{ links: MemoLinkDraft[]; activeUrl?: string | null; removable?: boolean }>()
const emit = defineEmits<{ open: [link: MemoLinkDraft]; remove: [link: MemoLinkDraft] }>()

function onClick(event: MouseEvent, link: MemoLinkDraft) {
  if (opensInNewTab(event)) return
  event.preventDefault()
  emit('open', link)
}
</script>

<template>
  <ul v-if="props.links.length" class="flex flex-wrap gap-2" aria-label="紐付けたメモ">
    <li v-for="link in props.links" :key="link.url">
      <span
        class="inline-flex max-w-72 items-center gap-1 rounded-full border py-1 ps-2.5 pe-1 text-sm transition-colors"
        :class="link.url === activeUrl ? 'border-(--ui-primary) bg-(--ui-primary)/10 text-(--ui-text-highlighted)' : 'border-(--ui-border) bg-(--ui-bg-elevated) hover:border-(--ui-border-accented)'"
      >
        <UIcon name="i-lucide-sticky-note" class="size-3.5 shrink-0 text-(--ui-text-muted)" />
        <a :href="link.url" target="_blank" rel="noopener noreferrer" class="truncate focus-visible:outline-2" :title="`${link.title}（Ctrl+クリックで別タブ）`" @click="onClick($event, link)">{{ link.title }}</a>
        <UButton v-if="removable" icon="i-lucide-x" color="neutral" variant="ghost" size="xs" class="rounded-full" :aria-label="`「${link.title}」の紐付けを外す`" @click="emit('remove', link)" />
      </span>
    </li>
  </ul>
</template>

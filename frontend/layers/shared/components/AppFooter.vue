<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from '#imports'
import { DATA_CHANGED_EVENT, useTasmaneApi } from '../../core/composables/useCriticalData'
import { progressSummary, type ProgressSummary } from '../../core/composables/progressSummary'

// 全体の進捗概要（未着手・進行中・期限・確認待ち）を常に下端に表示
const api = useTasmaneApi()
const route = useRoute()
const summary = ref<ProgressSummary>()
let timer: ReturnType<typeof globalThis.setInterval> | undefined
let channel: InstanceType<typeof globalThis.BroadcastChannel> | undefined
let loading = false
let lastLoaded = 0

// 概要の再取得（連続した呼び出しはまとめる）
async function load(force = false) {
  if (loading || (!force && Date.now() - lastLoaded < 3000)) return
  loading = true
  try {
    const dashboard = await api.dashboard()
    summary.value = progressSummary(dashboard.tasks, dashboard.confirmations)
    lastLoaded = Date.now()
  } catch {
    // 未ログイン・API停止時は表示しない（エラー表示は各画面に任せる）
  } finally {
    loading = false
  }
}
const onChanged = () => { void load(true) }
const onVisible = () => { if (globalThis.document.visibilityState === 'visible') void load() }

onMounted(() => {
  void load(true)
  // 画面遷移・タブ復帰・このタブ／他タブでの変更・1分ごとに更新
  timer = globalThis.setInterval(() => { if (globalThis.document.visibilityState === 'visible') void load(true) }, 60_000)
  globalThis.document.addEventListener('visibilitychange', onVisible)
  globalThis.addEventListener(DATA_CHANGED_EVENT, onChanged)
  if (typeof globalThis.BroadcastChannel !== 'undefined') {
    channel = new globalThis.BroadcastChannel('tasmane-entities')
    channel.onmessage = () => { setTimeout(() => void load(true), 300) }
  }
})
onBeforeUnmount(() => {
  if (timer) globalThis.clearInterval(timer)
  globalThis.document.removeEventListener('visibilitychange', onVisible)
  globalThis.removeEventListener(DATA_CHANGED_EVENT, onChanged)
  channel?.close()
})
watch(() => route.fullPath, () => { void load() })

// 表示項目（件数が要注意なら色付け）
const items = computed(() => {
  const value = summary.value
  if (!value) return []
  return [
    { label: '未着手', count: value.notStarted, icon: 'i-lucide-circle', to: '/dashboard', tone: 'text-(--ui-text-toned)' },
    { label: '進行中', count: value.inProgress, icon: 'i-lucide-circle-dot', to: '/dashboard', tone: 'text-(--ui-info)' },
    { label: '期限切れ', count: value.overdue, icon: 'i-lucide-flame', to: '/search?deadline=overdue', tone: value.overdue ? 'text-(--ui-error)' : 'text-(--ui-text-toned)' },
    { label: '今日まで', count: value.dueToday, icon: 'i-lucide-calendar-check', to: '/search?deadline=today', tone: value.dueToday ? 'text-(--ui-warning)' : 'text-(--ui-text-toned)' },
    { label: '確認待ち', count: value.openConfirmations, icon: 'i-lucide-message-circle-question', to: '/dashboard', tone: value.openConfirmations ? 'text-(--ui-info)' : 'text-(--ui-text-toned)' },
  ]
})
</script>

<template>
  <footer v-if="summary" class="sticky bottom-0 z-30 border-t border-(--ui-border) bg-(--ui-bg)/90 backdrop-blur" aria-label="進捗の概要">
    <div class="mx-auto flex h-9 max-w-[1600px] items-center gap-4 overflow-x-auto px-4 text-xs whitespace-nowrap sm:px-6">
      <NuxtLink v-for="item in items" :key="item.label" :to="item.to" class="flex items-center gap-1 rounded hover:text-(--ui-text-highlighted) focus-visible:outline-2 focus-visible:outline-(--ui-primary)" :class="item.tone">
        <UIcon :name="item.icon" class="size-3.5" aria-hidden="true" />
        <span>{{ item.label }}</span>
        <span class="font-mono font-medium tabular-nums">{{ item.count }}</span>
      </NuxtLink>
      <div class="order-first flex items-center gap-2 text-(--ui-text-muted) sm:order-none sm:ml-auto">
        <span>未完了 <span class="font-mono tabular-nums">{{ summary.open }}</span> / 全 <span class="font-mono tabular-nums">{{ summary.total }}</span> 件</span>
        <UProgress :model-value="summary.progress" size="xs" class="hidden w-20 sm:block" aria-label="全体の進捗" />
        <span class="font-mono tabular-nums">{{ summary.progress }}%</span>
      </div>
    </div>
  </footer>
</template>

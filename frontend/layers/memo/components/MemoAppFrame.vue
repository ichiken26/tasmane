<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from '#imports'
import { parseMemoMessage } from '../composables/memoBridge'
import { useMemoIntegration } from '../composables/useMemoIntegration'
import { useResizable } from '../composables/useResizable'

// メモアプリの埋め込み表示
// - メモアプリが未ログインなら、タスマネのGoogleログイン（IDトークン）を引き継いでログインさせる
// - メモ保存の通知を受けて saved を発火（タスクへの紐付けは呼び出し側）
const props = withDefaults(defineProps<{ src: string; title?: string; height?: string; resizable?: boolean }>(), { title: 'メモアプリ', height: '560px', resizable: false })
// 下端ドラッグでの高さ変更（ブラウザごとに記憶）
const resize = useResizable({ storageKey: 'tasmane-memo-frame-height-v2', initial: 680, min: 320, max: () => Math.max(320, (globalThis.innerHeight || 900) - 120), axis: 'y', direction: 1 })
const frameHeight = computed(() => props.resizable ? `${resize.size.value}px` : props.height)
const emit = defineEmits<{ saved: [memo: { id: string; title: string; url: string }] }>()
const route = useRoute()
const { memoOrigin } = useMemoIntegration()

const frame = ref<HTMLIFrameElement>()
type Phase = 'loading' | 'signing-in' | 'ready' | 'reauth' | 'failed' | 'unsupported'
const phase = ref<Phase>('loading')
const memoEmail = ref<string | null>(null)
const failure = ref('')
let handoffSent = false
let readyTimer: ReturnType<typeof setTimeout> | undefined
// 保存要求の応答待ち（要求ID → 解決関数）
type SavedMemo = { id: string; title: string; url: string }
const pendingSaves = new Map<string, (memo: SavedMemo | null) => void>()

// 書きかけのメモをメモアプリに保存させる（保存したメモ／保存しなかったら null）
function requestSave(timeoutMs = 8000): Promise<SavedMemo | null> {
  const target = frame.value?.contentWindow
  if (!target || phase.value !== 'ready') return Promise.resolve(null)
  const requestId = globalThis.crypto.randomUUID()
  return new Promise((resolve) => {
    const timer = setTimeout(() => { pendingSaves.delete(requestId); resolve(null) }, timeoutMs)
    pendingSaves.set(requestId, (memo) => { clearTimeout(timer); pendingSaves.delete(requestId); resolve(memo) })
    target.postMessage({ source: 'tasmane', type: 'save-request', requestId }, memoOrigin.value)
  })
}
defineExpose({ requestSave })

// 再認証（Googleログインをやり直して新しいIDトークンを受け取る）
const reauthUrl = computed(() => `/auth/login?returnTo=${encodeURIComponent(route.fullPath)}`)

// タスマネのGoogleログインをメモアプリへ引き継ぐ
async function handOff() {
  if (handoffSent) return
  handoffSent = true
  phase.value = 'signing-in'
  try {
    const token = await $fetch<{ idToken: string; rawNonce: string }>('/api/memo/handoff', { headers: { accept: 'application/json' } })
    frame.value?.contentWindow?.postMessage({ source: 'tasmane', type: 'auth', idToken: token.idToken, rawNonce: token.rawNonce }, memoOrigin.value)
  } catch {
    phase.value = 'reauth'
  }
}

// メモアプリからのメッセージ処理
function onMessage(event: MessageEvent) {
  const message = parseMemoMessage(event, frame.value?.contentWindow, memoOrigin.value)
  if (!message) return
  if (readyTimer) { clearTimeout(readyTimer); readyTimer = undefined }
  if (message.type === 'ready') {
    memoEmail.value = message.email
    if (message.signedIn) phase.value = 'ready'
    else void handOff()
  } else if (message.type === 'auth-result') {
    if (message.ok) phase.value = 'ready'
    else { phase.value = 'failed'; failure.value = message.error || '' }
  } else if (message.type === 'memo-saved') {
    emit('saved', message.memo)
    if (message.requestId) pendingSaves.get(message.requestId)?.(message.memo)
  } else if (message.type === 'save-skipped') {
    pendingSaves.get(message.requestId)?.(null)
  }
}

// 読み込みごとに連携状態をリセット（メモアプリ未対応なら一定時間で案内）
function onLoad() {
  if (phase.value === 'ready') return
  if (readyTimer) clearTimeout(readyTimer)
  readyTimer = setTimeout(() => { if (phase.value === 'loading') phase.value = 'unsupported' }, 10_000)
}
watch(() => props.src, () => { phase.value = 'loading'; handoffSent = false })

onMounted(() => globalThis.addEventListener('message', onMessage))
onBeforeUnmount(() => {
  globalThis.removeEventListener('message', onMessage)
  if (readyTimer) clearTimeout(readyTimer)
  for (const resolve of pendingSaves.values()) resolve(null)
})
</script>

<template>
  <div class="relative">
    <div class="relative overflow-hidden rounded-lg border border-(--ui-border) bg-(--ui-bg)" :class="resizable ? 'rounded-b-none' : ''" :style="{ height: frameHeight }">
      <iframe
        ref="frame"
        :src="src"
        :title="title"
        class="size-full"
        allow="clipboard-read; clipboard-write"
        referrerpolicy="strict-origin-when-cross-origin"
        @load="onLoad"
      />
      <!-- 連携状態のオーバーレイ -->
      <div v-if="phase !== 'ready'" class="absolute inset-0 flex items-center justify-center bg-(--ui-bg)/95 p-6 text-center" role="status">
        <div v-if="phase === 'loading' || phase === 'signing-in'" class="space-y-2 text-sm text-(--ui-text-muted)">
          <UIcon name="i-lucide-loader-circle" class="size-6 animate-spin" />
          <p>{{ phase === 'loading' ? 'メモアプリを読み込んでいます…' : 'タスマネのGoogleログインでメモアプリにログインしています…' }}</p>
        </div>
        <div v-else class="max-w-sm space-y-3">
          <UIcon name="i-lucide-circle-alert" class="size-6 text-(--ui-warning)" />
          <p v-if="phase === 'reauth'" class="text-sm">メモアプリにログインするため、Googleでの再認証が必要です（ログイン情報の有効期限切れ）。</p>
          <p v-else-if="phase === 'failed'" class="text-sm">
            メモアプリにログインできませんでした{{ failure ? `（${failure}）` : '' }}。
            Firebase の Google ログイン設定でタスマネの OAuth クライアントIDが許可されているか確認してください。
          </p>
          <p v-else class="text-sm">メモアプリから応答がありません。メモアプリが連携に対応していないか、通信できません。</p>
          <div class="flex flex-wrap justify-center gap-2">
            <UButton v-if="phase === 'reauth'" :to="reauthUrl" external icon="i-lucide-log-in" size="sm">Googleで再認証</UButton>
            <UButton :to="src" target="_blank" external color="neutral" variant="outline" icon="i-lucide-external-link" size="sm">別タブで開く</UButton>
            <UButton v-if="phase !== 'reauth'" color="neutral" variant="ghost" size="sm" @click="phase = 'ready'">このまま表示</UButton>
          </div>
        </div>
      </div>
    </div>
    <!-- 高さ変更ハンドル（ドラッグ／矢印キー） -->
    <div
      v-if="resizable"
      role="separator"
      aria-orientation="horizontal"
      aria-label="メモ表示枠の高さを変更"
      :aria-valuenow="resize.size.value"
      :aria-valuemin="resize.min"
      tabindex="0"
      class="group flex h-3 cursor-row-resize touch-none items-center justify-center rounded-b-lg border border-t-0 border-(--ui-border) bg-(--ui-bg-elevated) hover:bg-(--ui-bg-accented) focus-visible:outline-2 focus-visible:outline-(--ui-primary)"
      @pointerdown="resize.onPointerDown"
      @keydown="resize.onKeydown"
    >
      <span class="h-1 w-10 rounded-full bg-(--ui-border-accented) group-hover:bg-(--ui-primary)" />
    </div>
  </div>
</template>

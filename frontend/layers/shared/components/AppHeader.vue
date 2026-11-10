<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { DropdownMenuItem, NavigationMenuItem } from '@nuxt/ui'
import { useColorMode, useRoute } from '#imports'

interface AuthUser { email: string, name?: string, picture?: string }

const open = ref(false)
const user = ref<AuthUser>()
const sessionChecked = ref(false)
const route = useRoute()
const colorMode = useColorMode()
const loginHref = computed(() => `/auth/login?returnTo=${encodeURIComponent(route.fullPath)}`)
const isDark = computed({
  get: () => colorMode.value === 'dark',
  set: (value: boolean) => { colorMode.preference = value ? 'dark' : 'light' },
})

// 画面遷移でモバイルメニューを閉じる
watch(() => route.fullPath, () => { open.value = false })

// メインナビゲーション（仕様: Top / Dashboard / 新規タスク / ジャンル一覧 / 検索）
const links = computed<NavigationMenuItem[]>(() => [
  { label: 'Top', icon: 'i-lucide-house', to: '/', active: route.path === '/' },
  { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', to: '/dashboard', active: route.path.startsWith('/dashboard') },
  { label: 'ジャンル一覧', icon: 'i-lucide-folders', to: '/genres' },
  { label: '検索', icon: 'i-lucide-search', to: '/search' },
])

// プロフィールメニュー
const profileItems = computed<DropdownMenuItem[][]>(() => [
  [{ label: user.value?.name || user.value?.email || '', description: user.value?.name ? user.value.email : undefined, type: 'label' }],
  [{ label: 'ユーザー設定', icon: 'i-lucide-settings', to: '/settings' }, { label: 'ゴミ箱（30日以内に復元）', icon: 'i-lucide-trash', to: '/trash' }],
  [{ label: 'ログアウト', icon: 'i-lucide-log-out', color: 'error', onSelect: () => { void logout() } }],
])

// セッションからのユーザー取得
onMounted(async () => {
  try {
    const response = await globalThis.fetch('/auth/session', { headers: { accept: 'application/json' } })
    if (response.ok) user.value = ((await response.json()) as { user: AuthUser }).user
  } catch {
    user.value = undefined
  } finally {
    sessionChecked.value = true
  }
})

// ログアウト処理
async function logout(): Promise<void> {
  const response = await globalThis.fetch('/auth/logout', { method: 'POST', headers: { accept: 'application/json' } })
  if (response.ok) globalThis.location.assign('/auth/login')
}
</script>

<template>
  <!-- グローバルヘッダー -->
  <header class="sticky top-0 z-40 h-16 border-b border-(--ui-border) bg-(--ui-bg)/85 backdrop-blur">
    <div class="mx-auto flex h-full max-w-[1600px] items-center gap-3 px-4 sm:px-6">
      <UButton class="lg:hidden" icon="i-lucide-menu" color="neutral" variant="ghost" aria-label="メニューを開く" :aria-expanded="open" aria-controls="mobile-nav" @click="open = true" />
      <NuxtLink to="/" class="flex items-center gap-2 font-semibold text-(--ui-text-highlighted)" aria-label="タスマネ トップ">
        <AppLogo class="size-8" />
        <span class="text-lg tracking-tight">タスマネ</span>
      </NuxtLink>

      <!-- メインナビゲーション -->
      <UNavigationMenu :items="links" class="ml-6 hidden lg:flex" aria-label="メインナビゲーション" />

      <div class="ml-auto flex items-center gap-1.5">
        <UTooltip text="検索">
          <UButton class="lg:hidden" to="/search" icon="i-lucide-search" color="neutral" variant="ghost" aria-label="検索" />
        </UTooltip>
        <UButton to="/tasks/new" icon="i-lucide-plus" class="hidden sm:inline-flex">新規タスク</UButton>
        <UButton to="/tasks/new" icon="i-lucide-plus" class="sm:hidden" aria-label="新規タスク" />
        <ClientOnly>
          <UTooltip :text="isDark ? 'ライトモードへ' : 'ダークモードへ'">
            <UButton :icon="isDark ? 'i-lucide-sun' : 'i-lucide-moon'" color="neutral" variant="ghost" :aria-label="isDark ? 'ライトモードへ切替' : 'ダークモードへ切替'" @click="isDark = !isDark" />
          </UTooltip>
          <template #fallback><span class="size-9" /></template>
        </ClientOnly>
        <UDropdownMenu v-if="user" :items="profileItems" :content="{ align: 'end' }">
          <UButton color="neutral" variant="ghost" square class="rounded-full p-0.5" aria-label="Googleプロフィールメニュー">
            <UAvatar :src="user.picture" :alt="user.name || user.email" size="sm" />
          </UButton>
        </UDropdownMenu>
        <UButton v-else-if="sessionChecked" :href="loginHref" external icon="i-lucide-log-in" color="neutral" variant="outline">ログイン</UButton>
      </div>
    </div>

    <!-- モバイル用ナビゲーション -->
    <USlideover v-model:open="open" side="left" title="メニュー" :ui="{ content: 'max-w-xs', body: 'p-0 sm:p-0' }">
      <template #body>
        <nav id="mobile-nav" aria-label="モバイルナビゲーション" class="flex flex-col gap-4 py-3">
          <UNavigationMenu :items="links" orientation="vertical" class="px-3" />
          <USeparator />
          <AppSidebar />
        </nav>
      </template>
    </USlideover>
  </header>
</template>

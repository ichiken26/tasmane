<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

interface AuthUser { email: string, name?: string, picture?: string }

const open = ref(false)
const user = ref<AuthUser>()
const route = useRoute()
const loginHref = computed(() => `/auth/login?returnTo=${encodeURIComponent(route.fullPath)}`)

onMounted(async () => {
  try {
    const response = await globalThis.fetch('/auth/session', { headers: { accept: 'application/json' } })
    if (response.ok) user.value = ((await response.json()) as { user: AuthUser }).user
  } catch {
    user.value = undefined
  }
})

async function logout(): Promise<void> {
  const response = await globalThis.fetch('/auth/logout', { method: 'POST', headers: { accept: 'application/json' } })
  if (response.ok) globalThis.location.assign('/auth/login')
}
</script>
<template>
  <header class="header">
    <NuxtLink class="logo" to="/" aria-label="タスマネ トップ"><svg aria-hidden="true" viewBox="0 0 40 40"><path d="M7 12h26v20H7zM12 7v10m16-10v10M13 23h14" /></svg><span>タスマネ</span></NuxtLink>
    <a class="skip-link" href="#main-content">本文へ移動</a>
    <button class="menu-button" type="button" :aria-expanded="open" aria-controls="global-nav" @click="open = !open">メニュー</button>
    <nav id="global-nav" :class="{ open }" aria-label="メインナビゲーション">
      <NuxtLink to="/">Top</NuxtLink><NuxtLink to="/dashboard">Dashboard</NuxtLink><NuxtLink to="/tasks/new">新規タスク</NuxtLink><NuxtLink to="/genres">ジャンル一覧</NuxtLink><NuxtLink to="/search">検索</NuxtLink>
      <details v-if="user" class="profile-menu">
        <summary aria-label="Googleプロフィールメニュー">
          <img v-if="user.picture" :src="user.picture" alt="" referrerpolicy="no-referrer">
          <span v-else aria-hidden="true">{{ (user.name || user.email).slice(0, 1).toUpperCase() }}</span>
        </summary>
        <div class="profile-popover"><p>{{ user.name || user.email }}</p><NuxtLink to="/settings">ユーザー設定</NuxtLink><button type="button" @click="logout">ログアウト</button></div>
      </details>
      <a v-else :href="loginHref">Googleでログイン</a>
    </nav>
  </header>
</template>

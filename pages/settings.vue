<script setup lang="ts">
import { onMounted, ref } from 'vue'

interface AuthUser { email: string, name?: string, picture?: string }

const user = ref<AuthUser>()
const loading = ref(true)
const syncMessage = ref('')

onMounted(async () => {
  try {
    const response = await globalThis.fetch('/auth/session', { headers: { accept: 'application/json' } })
    if (response.status === 401) {
      globalThis.location.assign('/auth/login?returnTo=%2Fsettings')
      return
    }
    if (!response.ok) throw new Error('profile request failed')
    user.value = ((await response.json()) as { user: AuthUser }).user
  } finally {
    loading.value = false
  }
})

async function requestGoogleSync(): Promise<void> {
  syncMessage.value = '同期を受け付けています…'
  const response = await globalThis.fetch('/v1/google/sync', { method: 'POST', headers: { accept: 'application/json' } })
  syncMessage.value = response.ok ? 'Google Tasks同期を受け付けました。' : '同期を開始できませんでした。時間をおいて再試行してください。'
}

useSeoMeta({ title: 'ユーザー設定 | タスマネ' })
</script>

<template>
  <section class="page">
    <p class="eyebrow">Account</p>
    <h1>ユーザー設定</h1>
    <p v-if="loading" role="status">プロフィールを読み込んでいます…</p>
    <div v-else-if="user" class="grid two">
      <article class="card profile-card">
        <img v-if="user.picture" :src="user.picture" alt="" referrerpolicy="no-referrer">
        <div><h2>{{ user.name || 'Googleアカウント' }}</h2><p class="muted">{{ user.email }}</p></div>
      </article>
      <article class="card">
        <h2>Google Tasks</h2>
        <p>接続済みのGoogle Tasksとタスマネの同期を開始します。</p>
        <button class="primary" type="button" @click="requestGoogleSync">今すぐ同期</button>
        <p v-if="syncMessage" role="status">{{ syncMessage }}</p>
      </article>
    </div>
  </section>
</template>

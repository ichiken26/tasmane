<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { GoogleConflict, GoogleImportCandidate, UserSettings } from '~/frontend/layers/core/composables/useCriticalData'
import { useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'

interface AuthUser { email: string; name?: string; picture?: string }

const api = useTasmaneApi()
const user = ref<AuthUser>()
const loading = ref(true)
const syncMessage = ref('')
const settingsMessage = ref('')
const settings = reactive<UserSettings>({ timezone: 'Asia/Tokyo', googleSyncIntervalMinutes: 10 })
const conflicts = ref<GoogleConflict[]>([])
const imports = ref<GoogleImportCandidate[]>([])

onMounted(async () => {
  try {
    const response = await globalThis.fetch('/auth/session', { headers: { accept: 'application/json' } })
    if (response.status === 401) {
      globalThis.location.assign('/auth/login?returnTo=%2Fsettings')
      return
    }
    if (!response.ok) throw new Error('profile request failed')
    user.value = ((await response.json()) as { user: AuthUser }).user
    const [current, conflictRows, importRows] = await Promise.all([
      api.settings(),
      api.googleConflicts(),
      api.googleImportCandidates(),
    ])
    settings.timezone = current.timezone
    settings.googleSyncIntervalMinutes = current.googleSyncIntervalMinutes
    conflicts.value = conflictRows
    imports.value = importRows
  } finally {
    loading.value = false
  }
})

async function saveSettings() {
  settingsMessage.value = ''
  try {
    const saved = await api.updateSettings({
      timezone: settings.timezone,
      googleSyncIntervalMinutes: settings.googleSyncIntervalMinutes,
    })
    settings.timezone = saved.timezone
    settings.googleSyncIntervalMinutes = saved.googleSyncIntervalMinutes
    settingsMessage.value = '設定を保存しました。'
  } catch {
    settingsMessage.value = '設定を保存できませんでした。'
  }
}

async function requestGoogleSync() {
  syncMessage.value = '同期を受け付けています…'
  const response = await globalThis.fetch('/v1/google/sync', { method: 'POST', headers: { accept: 'application/json' } })
  syncMessage.value = response.ok
    ? 'Google Tasks同期を受け付けました。'
    : '同期を開始できませんでした。時間をおいて再試行してください。'
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
        <div>
          <h2>{{ user.name || 'Googleアカウント' }}</h2>
          <p class="muted">{{ user.email }}</p>
        </div>
      </article>
      <article class="card">
        <h2>サーバー設定</h2>
        <form class="form" @submit.prevent="saveSettings">
          <label>タイムゾーン (IANA)<input v-model="settings.timezone" required placeholder="Asia/Tokyo" /></label>
          <label>Google同期間隔（分）<input v-model.number="settings.googleSyncIntervalMinutes" type="number" min="1" max="1440" required /></label>
          <button class="primary" type="submit">設定を保存</button>
          <p v-if="settingsMessage" role="status">{{ settingsMessage }}</p>
        </form>
      </article>
      <article class="card">
        <h2>Google Tasks</h2>
        <p>接続済みのGoogle Tasksとタスマネの同期を開始します。</p>
        <button class="primary" type="button" @click="requestGoogleSync">今すぐ同期</button>
        <p v-if="syncMessage" role="status">{{ syncMessage }}</p>
      </article>
      <article class="card">
        <h2>同期コンフリクト</h2>
        <ul v-if="conflicts.length">
          <li v-for="item in conflicts" :key="`${item.taskId}:${item.code}`">
            <NuxtLink :to="`/tasks/${item.taskId}`">{{ item.taskId }}</NuxtLink>
            — {{ item.code }}: {{ item.message }}
          </li>
        </ul>
        <p v-else class="muted">未解決のコンフリクトはありません。</p>
        <h3>Import候補</h3>
        <ul v-if="imports.length">
          <li v-for="item in imports" :key="`${item.googleTaskListId}:${item.googleTaskId}`">
            {{ item.googleTaskListId }} / {{ item.googleTaskId }}
          </li>
        </ul>
        <p v-else class="muted">Import候補はありません。</p>
      </article>
    </div>
  </section>
</template>

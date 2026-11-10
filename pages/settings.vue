<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useColorMode, useSeoMeta } from '#imports'
import { useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import { useNotify } from '~/frontend/layers/shared/composables/useNotify'
import { useMemoIntegration } from '~/frontend/layers/memo/composables/useMemoIntegration'

interface AuthUser { email: string; name?: string; picture?: string }

useSeoMeta({ title: 'ユーザー設定 | タスマネ' })
const api = useTasmaneApi()
const notify = useNotify()
const colorMode = useColorMode()
const { enabled: memoIntegration, memoOrigin } = useMemoIntegration()
const user = ref<AuthUser>()
const loading = ref(true)
const loadError = ref<unknown>()
const saving = ref(false)
const settings = reactive({ timezone: 'Asia/Tokyo', googleSyncIntervalMinutes: 10, memoIntegrationEnabled: false })
let savedSnapshot = JSON.stringify(settings)
const dirty = computed(() => JSON.stringify(settings) !== savedSnapshot)

// IANAタイムゾーン一覧（ブラウザ非対応時は主要なもの）
const timezones = (() => {
  try { return (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone') }
  catch { return ['Asia/Tokyo', 'UTC', 'America/Los_Angeles', 'America/New_York', 'Europe/London'] }
})()
const colorModes = [
  { label: 'OSに合わせる', value: 'system', icon: 'i-lucide-monitor' },
  { label: 'ライト', value: 'light', icon: 'i-lucide-sun' },
  { label: 'ダーク', value: 'dark', icon: 'i-lucide-moon' },
]
const colorPreference = computed({ get: () => colorMode.preference, set: (value: string) => { colorMode.preference = value } })

// セッションと設定・同期候補の初期読込
async function load() {
  loading.value = true
  loadError.value = undefined
  try {
    const response = await globalThis.fetch('/auth/session', { headers: { accept: 'application/json' } })
    if (response.status === 401) {
      globalThis.location.assign('/auth/login?returnTo=%2Fsettings')
      return
    }
    if (!response.ok) throw new Error('profile request failed')
    user.value = ((await response.json()) as { user: AuthUser }).user
    const current = await api.settings()
    Object.assign(settings, { timezone: current.timezone, googleSyncIntervalMinutes: current.googleSyncIntervalMinutes, memoIntegrationEnabled: current.memoIntegrationEnabled })
    savedSnapshot = JSON.stringify(settings)
  } catch (error) {
    loadError.value = error
  } finally {
    loading.value = false
  }
}
onMounted(load)

// ユーザー設定の保存
async function saveSettings() {
  saving.value = true
  try {
    const saved = await api.updateSettings({ ...settings })
    Object.assign(settings, { timezone: saved.timezone, googleSyncIntervalMinutes: saved.googleSyncIntervalMinutes, memoIntegrationEnabled: saved.memoIntegrationEnabled })
    memoIntegration.value = saved.memoIntegrationEnabled
    savedSnapshot = JSON.stringify(settings)
    notify.success('設定を保存しました')
  } catch (error) {
    notify.error(error, '設定を保存できませんでした。')
  } finally { saving.value = false }
}

</script>

<template>
  <section class="mx-auto max-w-4xl">
    <PageHeader eyebrow="Account" title="ユーザー設定" />
    <div v-if="loading" class="space-y-4" role="status" aria-label="読み込み中"><USkeleton v-for="index in 3" :key="index" class="h-40 rounded-xl" /></div>
    <ErrorState v-else-if="loadError" :error="loadError" fallback="設定を取得できませんでした。" @retry="load" />
    <div v-else-if="user" class="space-y-6">
      <!-- プロフィール -->
      <UCard>
        <div class="flex items-center gap-4">
          <UAvatar :src="user.picture" :alt="user.name || user.email" size="3xl" />
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-(--ui-text-highlighted)">{{ user.name || 'Googleアカウント' }}</h2>
            <p class="truncate text-(--ui-text-muted)">{{ user.email }}</p>
          </div>
        </div>
      </UCard>

      <!-- 表示設定 -->
      <UCard>
        <template #header><h2 class="font-semibold text-(--ui-text-highlighted)">表示</h2></template>
        <UFormField label="テーマ" help="この設定はこのブラウザに保存されます。">
          <ClientOnly><UTabs v-model="colorPreference" :items="colorModes" :content="false" color="neutral" class="w-full sm:w-auto" /></ClientOnly>
        </UFormField>
      </UCard>

      <!-- サーバー設定 -->
      <UCard>
        <template #header>
          <div>
            <h2 class="font-semibold text-(--ui-text-highlighted)">サーバー設定</h2>
            <p class="text-xs text-(--ui-text-muted)">期限判定・AIアドバイス生成・Google同期に使われます。</p>
          </div>
        </template>
        <form class="grid gap-4 sm:grid-cols-2" @submit.prevent="saveSettings">
          <UFormField label="タイムゾーン" required>
            <USelectMenu v-model="settings.timezone" :items="timezones" class="w-full" :search-input="{ placeholder: '例: Tokyo' }" />
          </UFormField>
          <UFormField label="Google同期間隔" required help="1〜1440分">
            <UInputNumber v-model="settings.googleSyncIntervalMinutes" :min="1" :max="1440" class="w-full" />
          </UFormField>
          <div class="rounded-lg border border-(--ui-border) p-4 sm:col-span-2">
            <USwitch v-model="settings.memoIntegrationEnabled" label="メモアプリと連携する" :description="`タスク画面に ${memoOrigin.replace('https://', '')} を表示し、作成したメモをタスクに紐付けます。メモアプリへはタスマネのGoogleログインで自動ログインします。`" />
          </div>
          <div class="flex justify-end sm:col-span-2">
            <UButton type="submit" :loading="saving" :disabled="!dirty" icon="i-lucide-check">設定を保存</UButton>
          </div>
        </form>
      </UCard>

      <!-- 確認先プリセット -->
      <ConfirmationTargetManager />

      <!-- Google Tasks同期 -->
      <GoogleSyncPanel />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { NuxtError } from '#app'
import { clearError, useSeoMeta } from '#imports'

// アプリ全体のエラーページ（404など）
const props = defineProps<{ error: NuxtError }>()
const notFound = computed(() => props.error.statusCode === 404)
useSeoMeta({ title: () => `${notFound.value ? 'ページが見つかりません' : 'エラー'} | タスマネ` })
</script>

<template>
  <UApp>
    <div class="grid min-h-dvh place-items-center bg-(--ui-bg) px-4">
      <div class="max-w-md text-center">
        <p class="font-mono text-sm text-(--ui-primary)">{{ error.statusCode }}</p>
        <h1 class="mt-2 text-2xl font-semibold text-(--ui-text-highlighted)">{{ notFound ? 'ページが見つかりません' : '問題が発生しました' }}</h1>
        <p class="mt-2 text-(--ui-text-muted)">{{ notFound ? 'URLが間違っているか、タスクが削除された可能性があります。' : '時間をおいて再度お試しください。' }}</p>
        <div class="mt-6 flex justify-center gap-2">
          <UButton icon="i-lucide-house" @click="clearError({ redirect: '/' })">トップへ戻る</UButton>
          <UButton color="neutral" variant="outline" icon="i-lucide-layout-dashboard" @click="clearError({ redirect: '/dashboard' })">Dashboard</UButton>
        </div>
      </div>
    </div>
  </UApp>
</template>

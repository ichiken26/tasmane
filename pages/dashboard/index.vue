<script setup lang="ts">
import { useSeoMeta } from '#imports'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'

useSeoMeta({ title: 'Dashboard | タスマネ' })
const api = useTasmaneApi()
// 全ジャンルDashboardの取得
const { data, status, error, refresh } = useCriticalData('dashboard', () => api.dashboard())
</script>

<template>
  <!-- Dashboard本体 -->
  <div class="mx-auto max-w-7xl">
    <PageHeader eyebrow="All genres" title="Dashboard" description="全ジャンル横断で、今やるべきことを確認します。">
      <template #actions>
        <UButton icon="i-lucide-refresh-cw" color="neutral" variant="outline" :loading="status === 'pending'" aria-label="再読み込み" @click="refresh()" />
      </template>
    </PageHeader>
    <DashboardSkeleton v-if="status === 'pending' && !data" />
    <ErrorState v-else-if="error" :error="error" fallback="Dashboardを取得できませんでした。" @retry="refresh()" />
    <DashboardView v-else :tasks="data?.tasks || []" :confirmations="data?.confirmations || []" :advice="data?.advice" />
  </div>
</template>

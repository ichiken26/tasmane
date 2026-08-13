<script setup lang="ts">
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
useSeoMeta({ title: 'Dashboard | タスマネ' })
const api = useTasmaneApi()
const { data, status, error } = useCriticalData('dashboard', () => api.dashboard())
</script>
<template><div class="page"><span class="eyebrow">All genres</span><h1>Dashboard</h1><p v-if="status === 'pending'" role="status">Dashboardを読み込み中…</p><p v-else-if="error" role="alert">Dashboardを取得できませんでした。</p><DashboardView v-else :tasks="data?.tasks || []" :confirmations="data?.confirmations || []" :advice="data?.advice?.content" /></div></template>

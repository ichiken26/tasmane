<script setup lang="ts">
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
useSeoMeta({ title: 'ジャンル一覧 | タスマネ' })
const api = useTasmaneApi()
const { data: genres, status, error } = useCriticalData('genres', api.genres)
</script>
<template><div class="page"><span class="eyebrow">Genres</span><h1>ジャンル一覧</h1><p v-if="status === 'pending'" role="status">ジャンルを読み込み中…</p><p v-else-if="error" role="alert">ジャンルを取得できませんでした。</p><div v-else class="grid"><article v-for="genre in genres || []" :key="genre.id" class="card"><h2>{{ genre.name }}</h2><NuxtLink :to="`/dashboard/genres/${genre.id}`">Dashboardへ</NuxtLink></article></div></div></template>

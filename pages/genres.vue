<script setup lang="ts">
useSeoMeta({ title: 'ジャンル一覧 | タスマネ' })
const genres = [{ name: '開発', roots: [['認証基盤', 65], ['同期', 40]] }, { name: '運用', roots: [['リリース', 25]] }]
</script>
<template><div class="page"><span class="eyebrow">Genres</span><h1>ジャンル一覧</h1><div class="grid"><article v-for="genre in genres" :key="genre.name" class="card"><h2>{{ genre.name }}</h2><ul><li v-for="root in genre.roots" :key="String(root[0])"><span>{{ root[0] }}</span> — <strong>{{ root[1] }}%</strong></li></ul><NuxtLink :to="`/dashboard/genres/${genre.name}`">Dashboardへ</NuxtLink></article></div></div></template>

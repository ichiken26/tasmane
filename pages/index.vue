<script setup lang="ts">
import { computed } from 'vue'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
const api=useTasmaneApi();const {data}=useCriticalData('top-dashboard',()=>api.dashboard())
const nextTask=computed(()=>data.value?.tasks[0]);const overdue=computed(()=>data.value?.tasks.filter(task=>task.showFire).length||0)
useSeoMeta({title:'トップ | タスマネ',description:'次にやるべきタスクをすぐ把握'})
</script>
<template><div class="page hero"><span class="eyebrow">Today / Focus</span><h1>大量のタスクから、次にやることを迷わない。</h1><p class="lead">期限・進捗・依存関係をまとめて捉え、今日の焦点を静かに示します。</p><div class="actions"><NuxtLink class="primary" to="/dashboard">Dashboardを見る</NuxtLink><NuxtLink to="/tasks/new">タスクを追加</NuxtLink></div><div class="grid"><article class="card"><span class="eyebrow">Next</span><h2>{{ nextTask?.title || '着手候補なし' }}</h2><p>{{ nextTask?.effectivePriority || '—' }}</p></article><article class="card"><span class="eyebrow">Tasks</span><h2>{{ data?.tasks.length || 0 }}件</h2><p>現在の対象タスク</p></article><article class="card"><span class="eyebrow">Fire</span><h2>🔥 {{ overdue }}件</h2><p>折りたたみ中の子タスクを含む</p></article></div></div></template>

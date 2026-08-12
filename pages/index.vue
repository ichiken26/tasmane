<script setup lang="ts">
useSeoMeta({ title: 'トップ | タスマネ', description: '次にやるべきタスクをすぐ把握' })
</script>
<template><div class="page hero"><span class="eyebrow">Today / Focus</span><h1>大量のタスクから、次にやることを迷わない。</h1><p class="lead">期限・進捗・依存関係をまとめて捉え、今日の焦点を静かに示します。</p><div class="actions"><NuxtLink class="primary" to="/dashboard">Dashboardを見る</NuxtLink><NuxtLink to="/tasks/new">タスクを追加</NuxtLink></div><div class="grid"><article class="card"><span class="eyebrow">Next</span><h2>認証フローを完成</h2><p>期限: 今日 / HIGH</p></article><article class="card"><span class="eyebrow">Checking</span><h2>確認事項 2件</h2><p>期限切れの確認を優先</p></article><article class="card"><span class="eyebrow">Fire</span><h2>🔥 期限切れ 2件</h2><p>折りたたみ中の子タスクを含む</p></article></div></div></template>

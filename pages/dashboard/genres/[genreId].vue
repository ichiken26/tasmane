<script setup lang="ts">
const route = useRoute()
const genre = String(route.params.genreId)
useSeoMeta({ title: `${genre} | タスマネ` })
</script>
<template><div class="page"><span class="eyebrow">Genre Dashboard</span><h1>{{ genre }}</h1><DashboardView :genre="genre"/><section class="card"><h2>Gantt</h2><div class="gantt"><div class="gantt-row"><span>認証基盤</span><div class="gantt-bar" style="width:75%">8/12 — 8/19</div></div><div class="gantt-row"><span>同期</span><div class="gantt-bar" style="width:45%;margin-left:25%">8/14 — 8/18</div></div></div></section></div></template>

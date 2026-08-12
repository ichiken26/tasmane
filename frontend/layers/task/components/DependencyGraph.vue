<script setup lang="ts">
defineProps<{ tasks: Array<{ id: string; title: string; dependencies: string[] }> }>()
</script>
<template><section class="card"><h2>Dependency Graph</h2><p class="muted">Hierarchyとは異なる作業前提の参照です。</p><ul><li v-for="task in tasks" :key="task.id" :class="{ 'multi-dependency': task.dependencies.length > 1 }"><strong>{{ task.title }}</strong><span> ← {{ task.dependencies.join(' / ') || '前提なし' }}</span></li></ul></section></template>

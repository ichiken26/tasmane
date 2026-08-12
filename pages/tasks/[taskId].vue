<script setup lang="ts">
const route = useRoute()
const task = { title: `Task ${route.params.taskId}`, body: '認証済みUser Contextを検証する', genre: '開発', version: 3 }
useSeoMeta({ title: `${task.title} | タスマネ` })
const tree = [{ id: 'root', title: '認証基盤', progress: 65, overdue: true }, { id: String(route.params.taskId), title: task.title, progress: 40 }]
const deps = [{ id: 'a', title: 'API接続', dependencies: ['Schema', 'Authentication'] }, { id: 'b', title: 'UI', dependencies: ['API接続'] }]
</script>
<template><div class="page"><span class="eyebrow">Task / version {{ task.version }}</span><h1>{{ task.title }}</h1><TaskEditor :task="task"/><div class="grid two"><HierarchyTree :tasks="tree"/><DependencyGraph :tasks="deps"/></div></div></template>

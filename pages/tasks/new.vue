<script setup lang="ts">
import { computed } from 'vue'
import { definePageMeta, useRoute, useSeoMeta } from '#imports'

useSeoMeta({ title: '新規タスク | タスマネ' })
// 子タスク追加・ジャンル指定の初期値（クエリから）
definePageMeta({ key: route => route.fullPath })
const route = useRoute()
const defaults = computed(() => ({
  genreId: typeof route.query.genreId === 'string' ? route.query.genreId : undefined,
  parentTaskId: typeof route.query.parentTaskId === 'string' ? route.query.parentTaskId : undefined,
}))
</script>

<template>
  <!-- 新規タスク編集 -->
  <MemoSplitLayout>
    <PageHeader eyebrow="Create" title="新規タスク" :description="defaults.parentTaskId ? '子タスクとして追加します。親タスクの進捗は子タスクから自動で計算されます。' : 'タイトルとジャンルだけで作成できます。'" />
    <TaskEditor :defaults="defaults" />
  </MemoSplitLayout>
</template>

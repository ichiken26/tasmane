<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { encodeSearch, parseSearch } from '../frontend/layers/core/composables/taskState'
import { useCriticalData, useTasmaneApi } from '~/frontend/layers/core/composables/useCriticalData'
import type { ApiTask } from '~/frontend/layers/core/composables/useCriticalData'
const route=useRoute(), router=useRouter(), api=useTasmaneApi()
const state=reactive(parseSearch(route.query)), results=ref<ApiTask[]>([])
const { data: genres }=useCriticalData('search-genres',api.genres)
let request=0
watch(state,async value=>{await router.replace({query:encodeSearch(value)});const current=++request;const query=encodeSearch(value) as Record<string,string>;const found=await api.search(query);if(current===request)results.value=found},{deep:true,immediate:true})
watch(() => route.query, (query) => {
  const restored = parseSearch(query)
  if (JSON.stringify(restored) !== JSON.stringify({ ...state })) Object.assign(state, restored)
})
useSeoMeta({title:'検索 | タスマネ'})
</script>
<template><div class="page"><span class="eyebrow">Search</span><h1>検索</h1><div class="search-controls card"><label>フリーワード<input v-model="state.q" type="search" placeholder="title・bodyを検索" /></label><fieldset><legend>ジャンル（OR）</legend><label v-for="genre in genres || []" :key="genre.id"><input v-model="state.genres" type="checkbox" :value="genre.id" /> {{ genre.name }}</label></fieldset><label>優先度<select v-model="state.priority"><option value="">すべて</option><option value="high">HIGH</option><option value="medium">MEDIUM</option><option value="low">LOW</option></select></label><label>期限<select v-model="state.deadline"><option value="">すべて</option><option value="today">今日まで</option><option value="tomorrow">明日まで</option><option value="week">1週間以内</option><option value="month">1か月以内</option><option value="later">それ以上</option></select></label></div><div class="table-wrap card"><table><thead><tr><th>Task</th><th>本文</th><th>Priority</th><th>Status</th></tr></thead><tbody><tr v-for="task in results" :key="task.id"><td><NuxtLink :to="`/tasks/${task.id}`" target="_blank">{{ task.title }}</NuxtLink></td><td>{{ task.body }}</td><td>{{ task.effectivePriority }}</td><td>{{ task.status }}</td></tr></tbody></table></div></div></template>

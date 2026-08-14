<script setup lang="ts">
import { computed, ref } from 'vue'

export interface HierarchyNode {
  id: string
  title: string
  progress: number
  overdue?: boolean
  parentTaskId?: string | null
}

const props = defineProps<{ tasks: HierarchyNode[] }>()
const collapsed = ref(new Set<string>())

const byParent = computed(() => {
  const map = new Map<string | null, HierarchyNode[]>()
  for (const task of props.tasks) {
    const key = task.parentTaskId ?? null
    const list = map.get(key) ?? []
    list.push(task)
    map.set(key, list)
  }
  return map
})

const visible = computed(() => {
  const ids = new Set(props.tasks.map(task => task.id))
  const roots = props.tasks.filter(task => !task.parentTaskId || !ids.has(task.parentTaskId))
  const rows: Array<HierarchyNode & { depth: number; hasChildren: boolean }> = []
  const stack = roots.map(node => ({ node, depth: 0 })).reverse()
  while (stack.length) {
    const current = stack.pop()!
    const children = byParent.value.get(current.node.id) ?? []
    rows.push({ ...current.node, depth: current.depth, hasChildren: children.length > 0 })
    if (!children.length || collapsed.value.has(current.node.id)) continue
    for (let index = children.length - 1; index >= 0; index -= 1) {
      const child = children[index]
      if (!child) continue
      stack.push({ node: child, depth: current.depth + 1 })
    }
  }
  return rows
})

function toggle(id: string) {
  const next = new Set(collapsed.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  collapsed.value = next
}
</script>

<template>
  <section class="card">
    <h2>Hierarchy Tree</h2>
    <ul v-if="visible.length" class="tree">
      <li v-for="task in visible" :key="task.id" :style="{ paddingLeft: `${task.depth * 16}px` }">
        <button
          v-if="task.hasChildren"
          type="button"
          :aria-expanded="!collapsed.has(task.id)"
          :aria-label="collapsed.has(task.id) ? '子タスクを展開' : '子タスクを折りたたむ'"
          @click="toggle(task.id)"
        >
          {{ collapsed.has(task.id) ? '▸' : '▾' }}
        </button>
        <span v-else class="tree-leaf" aria-hidden="true">•</span>
        <NuxtLink :to="`/tasks/${task.id}`" target="_blank">{{ task.title }}</NuxtLink>
        <span>{{ task.progress }}%</span>
        <span v-if="task.overdue" aria-label="期限切れ">🔥</span>
      </li>
    </ul>
    <p v-else class="muted">表示できる階層がありません。</p>
  </section>
</template>

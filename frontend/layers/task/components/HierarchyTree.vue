<script setup lang="ts">
import { computed, ref } from 'vue'
import { statusMeta } from '../../core/composables/labels'

export interface HierarchyNode {
  id: string
  title: string
  progress: number
  overdue?: boolean
  parentTaskId?: string | null
  status?: string
}

const props = withDefaults(defineProps<{ tasks: HierarchyNode[]; currentId?: string; loading?: boolean }>(), { currentId: undefined })
const collapsed = ref(new Set<string>())

// 親IDごとの子タスク分類
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

// 折りたたみ状態を反映した表示行の構築
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

// 子タスクの折りたたみ切替
function toggle(id: string) {
  const next = new Set(collapsed.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  collapsed.value = next
}

// すべて展開・折りたたみ
const parentIds = computed(() => props.tasks.filter(task => byParent.value.has(task.id)).map(task => task.id))
const collapseAll = () => { collapsed.value = new Set(parentIds.value) }
const expandAll = () => { collapsed.value = new Set() }
</script>

<template>
  <UCard :ui="{ body: 'p-2 sm:p-2' }">
    <template #header>
      <div class="flex items-center justify-between gap-2">
        <div>
          <h2 class="flex items-center gap-2 font-semibold text-(--ui-text-highlighted)"><UIcon name="i-lucide-list-tree" class="size-5 text-(--ui-text-muted)" />Hierarchy Tree</h2>
          <p class="text-xs text-(--ui-text-muted)">親子（分解・包含）の構造</p>
        </div>
        <div v-if="parentIds.length" class="flex gap-1">
          <UTooltip text="すべて展開"><UButton icon="i-lucide-chevrons-up-down" size="xs" color="neutral" variant="ghost" aria-label="すべて展開" @click="expandAll" /></UTooltip>
          <UTooltip text="すべて折りたたむ"><UButton icon="i-lucide-chevrons-down-up" size="xs" color="neutral" variant="ghost" aria-label="すべて折りたたむ" @click="collapseAll" /></UTooltip>
        </div>
      </div>
    </template>
    <!-- 階層ツリー一覧 -->
    <div v-if="loading" class="space-y-2 p-2"><USkeleton v-for="index in 3" :key="index" class="h-8 w-full" /></div>
    <ul v-else-if="visible.length" role="tree" aria-label="Hierarchy Tree" class="space-y-0.5">
      <li
        v-for="task in visible"
        :key="task.id"
        role="treeitem"
        :aria-expanded="task.hasChildren ? !collapsed.has(task.id) : undefined"
        :aria-current="task.id === currentId ? 'page' : undefined"
        class="group flex items-center gap-2 rounded-md py-1.5 pr-2 hover:bg-(--ui-bg-muted)"
        :class="{ 'bg-(--ui-primary)/10 hover:bg-(--ui-primary)/15': task.id === currentId }"
        :style="{ paddingLeft: `${task.depth * 20 + 6}px` }"
      >
        <UButton
          v-if="task.hasChildren"
          :icon="collapsed.has(task.id) ? 'i-lucide-chevron-right' : 'i-lucide-chevron-down'"
          size="xs" color="neutral" variant="ghost" square
          :aria-label="collapsed.has(task.id) ? '子タスクを展開' : '子タスクを折りたたむ'"
          @click="toggle(task.id)"
        />
        <span v-else class="grid size-6 shrink-0 place-items-center" aria-hidden="true"><span class="size-1.5 rounded-full bg-(--ui-border-accented)" /></span>
        <UIcon :name="statusMeta(task.status || '').icon" class="size-4 shrink-0" :class="task.status === 'COMPLETED' ? 'text-(--ui-success)' : task.status === 'IN_PROGRESS' ? 'text-(--ui-info)' : 'text-(--ui-text-dimmed)'" />
        <NuxtLink :to="`/tasks/${task.id}`" class="min-w-0 flex-1 truncate text-sm hover:underline" :class="task.id === currentId ? 'font-semibold text-(--ui-text-highlighted)' : ''">{{ task.title }}</NuxtLink>
        <FireMark v-if="task.overdue" />
        <UProgress :model-value="task.progress" size="xs" class="hidden w-16 sm:block" :color="task.progress >= 100 ? 'success' : 'primary'" />
        <span class="w-9 text-right font-mono text-xs text-(--ui-text-muted) tabular-nums">{{ task.progress }}%</span>
        <UTooltip text="別タブで開く">
          <UButton :to="`/tasks/${task.id}`" target="_blank" icon="i-lucide-external-link" size="xs" color="neutral" variant="ghost" class="opacity-0 group-hover:opacity-100 focus-visible:opacity-100" aria-label="別タブで開く" />
        </UTooltip>
      </li>
    </ul>
    <p v-else class="p-4 text-sm text-(--ui-text-muted)">表示できる階層がありません。</p>
  </UCard>
</template>

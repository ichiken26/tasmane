<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useConfirmationTargets } from '../composables/useConfirmationTargets'
import { useNotify } from '../../shared/composables/useNotify'

// 確認先プリセット（ユーザー個人）の管理
const notify = useNotify()
const { targets, load, create, remove } = useConfirmationTargets()
const name = ref('')
const busy = ref(false)
const loadError = ref<unknown>()

onMounted(async () => {
  try { await load() }
  catch (error) { loadError.value = error }
})

// プリセット追加
async function add() {
  if (!name.value.trim()) return
  busy.value = true
  try {
    await create(name.value)
    name.value = ''
  } catch (error) { notify.error(error, '追加できませんでした。同じ名前がないか確認してください。') }
  finally { busy.value = false }
}

// プリセット削除（既存の確認事項の確認先は変わらない）
async function drop(id: string) {
  try { await remove(id) }
  catch (error) { notify.error(error, '削除できませんでした。') }
}
</script>

<template>
  <UCard>
    <template #header>
      <div>
        <h2 class="font-semibold text-(--ui-text-highlighted)">確認先プリセット</h2>
        <p class="text-xs text-(--ui-text-muted)">確認事項を追加するときに選べる、よく使う確認先（人・部署）です。</p>
      </div>
    </template>
    <ErrorState v-if="loadError" :error="loadError" fallback="プリセットを取得できませんでした。" @retry="load" />
    <div v-else class="space-y-3">
      <div v-if="targets.length" class="flex flex-wrap gap-2">
        <UBadge v-for="target in targets" :key="target.id" color="neutral" variant="subtle" size="lg" class="gap-1">
          <UIcon name="i-lucide-user" class="size-3.5" />{{ target.name }}
          <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" :aria-label="`${target.name} を削除`" class="-mr-1 p-0" @click="drop(target.id)" />
        </UBadge>
      </div>
      <p v-else class="text-sm text-(--ui-text-muted)">まだプリセットはありません。</p>
      <form class="flex gap-2" @submit.prevent="add">
        <UInput v-model="name" placeholder="例: 経理部 田中さん" aria-label="確認先の名前" class="flex-1" />
        <UButton type="submit" icon="i-lucide-plus" :loading="busy" color="neutral" variant="soft">追加</UButton>
      </form>
    </div>
  </UCard>
</template>

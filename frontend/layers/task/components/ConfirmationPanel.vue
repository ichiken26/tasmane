<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { ApiConfirmation } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { formatDate, fromInputValue, relativeDeadline } from '../../core/composables/dates'
import { confirmationMeta } from '../../core/composables/labels'
import { apiErrorStatus } from '../../core/composables/apiError'
import { useNotify } from '../../shared/composables/useNotify'
import { useConfirmationTargets } from '../composables/useConfirmationTargets'

// タスクの確認事項（未確認があるとタスクは完了できない）
const props = defineProps<{ taskId: string; completed?: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const api = useTasmaneApi()
const notify = useNotify()
const confirmations = ref<ApiConfirmation[]>([])
const loading = ref(true)
const loadError = ref<unknown>()
const adding = ref(false)
const busyId = ref('')
const form = reactive({ content: '', targetName: '', dueAt: '' })
const { targets, load: loadTargets, create: createTarget } = useConfirmationTargets()
// 確認先候補（プリセット＋入力中の自由記述）
const targetItems = computed(() => {
  const names = targets.value.map(target => target.name)
  return form.targetName && !names.includes(form.targetName) ? [form.targetName, ...names] : names
})
const savePreset = ref(false)
const isPreset = computed(() => targets.value.some(target => target.name.toLowerCase() === form.targetName.trim().toLowerCase()))

const openCount = computed(() => confirmations.value.filter(item => item.status !== 'CONFIRMED').length)

// 確認事項の取得
async function load() {
  loading.value = true
  loadError.value = undefined
  try { confirmations.value = await api.confirmations(props.taskId) }
  catch (error) { loadError.value = error }
  finally { loading.value = false }
}
onMounted(() => { void load(); void loadTargets().catch(() => undefined) })

// 確認事項の遷移先一覧
const nextStatuses = (status: string) => {
  if (status === 'UNCONFIRMED') return ['CHECKING']
  if (status === 'CHECKING') return ['CONFIRMED', 'UNCONFIRMED']
  return ['CHECKING']
}
const nextLabel = (status: string) => ({ CHECKING: '確認中にする', CONFIRMED: '確認済みにする', UNCONFIRMED: '未確認に戻す' } as Record<string, string>)[status] ?? status

// 確認事項の追加
async function add() {
  if (!form.content.trim() || !form.targetName.trim()) return
  adding.value = true
  try {
    confirmations.value.push(await api.createConfirmation(props.taskId, {
      content: form.content.trim(), targetName: form.targetName.trim(), dueAt: fromInputValue(form.dueAt, true, 'end'),
    }))
    // 確認先をプリセットへ保存（任意）
    if (savePreset.value && !isPreset.value) await createTarget(form.targetName).catch(() => undefined)
    Object.assign(form, { content: '', targetName: '', dueAt: '' })
    savePreset.value = false
    emit('changed')
  } catch (error) {
    if (apiErrorStatus(error) === 422) notify.error(error, '完了済みのタスクには未確認の確認事項を追加できません。先にタスクを再開してください。')
    else notify.error(error, '確認事項を追加できませんでした。')
  }
  finally { adding.value = false }
}

// 確認事項の状態更新
async function setStatus(item: ApiConfirmation, status: string) {
  busyId.value = item.id
  try {
    const saved = await api.updateConfirmation(item.id, status)
    confirmations.value.splice(confirmations.value.findIndex(value => value.id === item.id), 1, { ...item, ...saved })
    emit('changed')
  } catch (error) {
    if (apiErrorStatus(error) === 422) notify.error(error, '完了済みのタスクの確認事項は「確認済み」から戻せません。先にタスクを再開してください。')
    else notify.error(error, '確認状態を変更できませんでした。')
  }
  finally { busyId.value = '' }
}
</script>

<template>
  <UCard :ui="{ body: 'p-0 sm:p-0' }">
    <template #header>
      <div class="flex items-center justify-between gap-2">
        <h2 class="flex items-center gap-2 font-semibold text-(--ui-text-highlighted)"><UIcon name="i-lucide-message-circle-question" class="size-5 text-(--ui-text-muted)" />確認事項</h2>
        <UBadge v-if="openCount" :label="`未完了 ${openCount}`" color="warning" variant="subtle" />
      </div>
    </template>
    <div v-if="loading" class="space-y-3 p-4"><USkeleton v-for="index in 2" :key="index" class="h-12 w-full" /></div>
    <div v-else-if="loadError" class="p-4"><ErrorState :error="loadError" fallback="確認事項を取得できませんでした。" @retry="load" /></div>
    <ul v-else-if="confirmations.length" class="divide-y divide-(--ui-border)">
      <li v-for="item in confirmations" :key="item.id" class="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center">
        <div class="min-w-0 flex-1">
          <p class="font-medium text-(--ui-text-highlighted)" :class="{ 'line-through opacity-60': item.status === 'CONFIRMED' }">{{ item.content }}</p>
          <p class="mt-0.5 flex flex-wrap items-center gap-x-3 text-xs text-(--ui-text-muted)">
            <span class="inline-flex items-center gap-1"><UIcon name="i-lucide-user" />{{ item.targetName }}</span>
            <span v-if="item.dueAt" class="inline-flex items-center gap-1"><UIcon name="i-lucide-calendar" />{{ formatDate(item.dueAt) }}（{{ relativeDeadline(item.dueAt) }}）</span>
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-1.5">
          <StatusBadge :value="item.status" kind="confirmation" />
          <UButton v-for="status in nextStatuses(item.status)" :key="status" size="xs" :color="status === 'CONFIRMED' ? 'success' : 'neutral'" :variant="status === 'CONFIRMED' ? 'soft' : 'ghost'" :icon="confirmationMeta(status).icon" :loading="busyId === item.id" @click="setStatus(item, status)">{{ nextLabel(status) }}</UButton>
        </div>
      </li>
    </ul>
    <p v-else class="px-4 py-6 text-center text-sm text-(--ui-text-muted)">確認事項はありません。誰かへの確認待ちがあれば追加しましょう。</p>
    <template #footer>
      <p v-if="completed" class="text-xs text-(--ui-text-muted)">完了済みのタスクには確認事項を追加できません。</p>
      <form v-else class="space-y-2" @submit.prevent="add">
        <div class="grid gap-2 sm:grid-cols-[1fr_12rem_10rem_auto]">
          <UInput v-model="form.content" required placeholder="確認する内容" aria-label="確認する内容" />
          <USelectMenu
            v-model="form.targetName"
            :items="targetItems"
            create-item
            placeholder="確認先（人・部署）"
            aria-label="確認先"
            :search-input="{ placeholder: '名前を入力 / プリセットから選択' }"
            @create="(value: string) => { form.targetName = value.trim() }"
          />
          <DateField v-model="form.dueAt" aria-label="確認期限" />
          <UButton type="submit" icon="i-lucide-plus" :loading="adding" :disabled="!form.content.trim() || !form.targetName.trim()" color="neutral" variant="soft">追加</UButton>
        </div>
        <UCheckbox v-if="form.targetName.trim() && !isPreset" v-model="savePreset" label="この確認先をプリセットに保存" size="sm" />
      </form>
    </template>
  </UCard>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ApiGenre } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useNotify } from '../../shared/composables/useNotify'

// ジャンルで利用できるタスクタイプの管理（既存タイプの紐付け・新規作成）
const props = defineProps<{ genre: ApiGenre }>()
const open = defineModel<boolean>('open', { default: false })
const api = useTasmaneApi()
const notify = useNotify()
const attached = ref<ApiGenre[]>([])
const all = ref<ApiGenre[]>([])
const loading = ref(false)
const busy = ref(false)
const selected = ref<string>()

const candidates = computed(() => all.value.filter(type => !attached.value.some(item => item.id === type.id)).map(type => ({ label: type.name, value: type.id })))

// 一覧の読込
async function load() {
  loading.value = true
  try { [attached.value, all.value] = await Promise.all([api.taskTypes(props.genre.id), api.allTaskTypes()]) }
  catch (error) { notify.error(error, 'タスクタイプを取得できませんでした。') }
  finally { loading.value = false }
}
watch(open, (value) => { if (value) void load() })

// 既存タイプの紐付け
async function attach(taskTypeId: string) {
  busy.value = true
  try {
    await api.attachTaskType(props.genre.id, taskTypeId)
    const type = all.value.find(item => item.id === taskTypeId)
    if (type) attached.value.push(type)
    selected.value = undefined
  } catch (error) { notify.error(error, '紐付けできませんでした。') }
  finally { busy.value = false }
}

// ジャンルからの解除（このジャンルのタスクで使用中なら不可）
async function detach(taskTypeId: string) {
  busy.value = true
  try {
    await api.detachTaskType(props.genre.id, taskTypeId)
    attached.value = attached.value.filter(item => item.id !== taskTypeId)
  } catch (error) { notify.error(error, 'このジャンルのタスクで使用中のため外せません。') }
  finally { busy.value = false }
}

// 新規作成して紐付け
async function create(name: string) {
  const value = name.trim()
  if (!value) return
  busy.value = true
  try {
    const type = await api.createTaskType(value)
    all.value.push(type)
    await api.attachTaskType(props.genre.id, type.id)
    attached.value.push(type)
    selected.value = undefined
    notify.success(`タスクタイプ「${type.name}」を追加しました`)
  } catch (error) { notify.error(error, 'タスクタイプを作成できませんでした。同名のタイプがないか確認してください。') }
  finally { busy.value = false }
}
</script>

<template>
  <UModal v-model:open="open" :title="`${genre.name} のタスクタイプ`" description="このジャンルのタスクで選択できるタイプを管理します。">
    <template #body>
      <div class="space-y-4">
        <div v-if="loading" class="flex flex-wrap gap-2"><USkeleton v-for="index in 4" :key="index" class="h-6 w-20" /></div>
        <div v-else-if="attached.length" class="flex flex-wrap gap-2">
          <UBadge v-for="type in attached" :key="type.id" color="neutral" variant="subtle" size="lg" class="gap-1">
            <UIcon name="i-lucide-tag" class="size-3.5" />{{ type.name }}
            <UButton icon="i-lucide-x" size="xs" color="neutral" variant="link" class="-mr-1 p-0" :disabled="busy" :aria-label="`${type.name} をこのジャンルから外す`" @click="detach(type.id)" />
          </UBadge>
        </div>
        <p v-else class="text-sm text-(--ui-text-muted)">まだタスクタイプがありません。</p>
        <UFormField label="タイプを追加" help="既存のタイプを選ぶか、名前を入力して新規作成できます。">
          <USelectMenu
            v-model="selected"
            :items="candidates"
            value-key="value"
            create-item
            :loading="busy"
            placeholder="タイプを選択 / 作成"
            class="w-full"
            @update:model-value="(value: string | undefined) => value && attach(value)"
            @create="create"
          />
        </UFormField>
      </div>
    </template>
  </UModal>
</template>

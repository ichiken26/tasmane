<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import type { ApiTask, ImpactPlan, TaskInput } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { impactLines, impactPlanFrom } from '../../core/composables/impact'

const props = defineProps<{ task?: ApiTask }>()
const emit = defineEmits<{ saved: [task: ApiTask] }>()
const api = useTasmaneApi()
const router = useRouter()
const genres = ref<{ id: string; name: string }[]>([])
const taskTypes = ref<{ id: string; name: string }[]>([])
const possibleParents = ref<ApiTask[]>([])
const busy = ref(false)
const errorMessage = ref('')
const impact = ref<ImpactPlan>()
const form = reactive({
  title: props.task?.title ?? '', body: props.task?.body ?? '', genreId: props.task?.genreId ?? '',
  parentTaskId: props.task?.parentTaskId ?? '', taskTypeId: props.task?.taskTypeId ?? '',
  priority: props.task?.manualPriority ?? '', startAt: toLocalInput(props.task?.startAt),
  endAt: toLocalInput(props.task?.endAt), isAllDay: props.task?.isAllDay ?? true,
})

onMounted(async () => {
  try { genres.value = await api.genres() } catch { errorMessage.value = 'ジャンルを取得できませんでした。' }
})
watch(() => form.genreId, async (genreId) => {
  form.parentTaskId = props.task?.genreId === genreId ? form.parentTaskId : ''
  form.taskTypeId = props.task?.genreId === genreId ? form.taskTypeId : ''
  if (!genreId) { taskTypes.value = []; possibleParents.value = []; return }
  try {
    const [types, tasks] = await Promise.all([api.taskTypes(genreId), api.search({ genres: genreId })])
    taskTypes.value = types
    possibleParents.value = tasks.filter(candidate => candidate.id !== props.task?.id && candidate.status !== 'COMPLETED')
  } catch { errorMessage.value = 'ジャンル関連データを取得できませんでした。' }
}, { immediate: true })

function toLocalInput(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}
const iso = (value: string) => value ? new Date(value).toISOString() : null
function requestBody(token?: string): TaskInput {
  return {
    title: form.title, body: form.body || null, genreId: form.genreId,
    taskTypeId: form.taskTypeId || null, parentTaskId: form.parentTaskId || null,
    manualPriority: form.priority || null, startAt: iso(form.startAt), endAt: iso(form.endAt),
    isAllDay: form.isAllDay, ...(token ? { impactToken: token } : {}),
  }
}
async function save(token?: string) {
  busy.value = true
  errorMessage.value = ''
  try {
    const body = requestBody(token)
    const saved = props.task
      ? await api.updateTask(props.task.id, { ...body, expectedVersion: props.task.version })
      : await api.createTask(body)
    impact.value = undefined
    emit('saved', saved)
    if (!props.task) await router.push(`/tasks/${saved.id}`)
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) impact.value = plan
    else errorMessage.value = '保存できませんでした。入力内容または最新versionを確認してください。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="card form" @submit.prevent="save()">
    <p v-if="errorMessage" role="alert">{{ errorMessage }}</p>
    <label>タイトル<span aria-hidden="true"> *</span><input v-model="form.title" required maxlength="240" /></label>
    <label>本文<textarea v-model="form.body" /></label>
    <label>ジャンル<select v-model="form.genreId" required><option value="">選択してください</option><option v-for="genre in genres" :key="genre.id" :value="genre.id">{{ genre.name }}</option></select></label>
    <div class="form-grid">
      <label>親タスク<select v-model="form.parentTaskId"><option value="">Root</option><option v-for="parent in possibleParents" :key="parent.id" :value="parent.id">{{ parent.title }}</option></select></label>
      <label>タスクタイプ<select v-model="form.taskTypeId"><option value="">指定なし</option><option v-for="type in taskTypes" :key="type.id" :value="type.id">{{ type.name }}</option></select></label>
      <label>優先度<select v-model="form.priority"><option value="">自動</option><option value="HIGH">HIGH</option><option value="MEDIUM">MEDIUM</option><option value="LOW">LOW</option></select></label>
      <label>開始<input v-model="form.startAt" type="datetime-local" /></label>
      <label>期限<input v-model="form.endAt" type="datetime-local" /></label>
      <label><span>終日</span><input v-model="form.isAllDay" type="checkbox" /></label>
    </div>
    <button class="primary" type="submit" :disabled="busy">{{ busy ? '保存中…' : task ? '変更を保存' : 'タスクを作成' }}</button>
    <ImpactModal :open="Boolean(impact)" title="変更の影響を確認" :impacts="impactLines(impact)" @cancel="impact=undefined" @confirm="impact && save(impact.impactToken)" />
  </form>
</template>

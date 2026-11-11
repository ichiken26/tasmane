<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { defineShortcuts, onBeforeRouteLeave, useRouter } from '#imports'
import type { ApiTask, ImpactPlan, TaskInput } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useGenreList } from '../../core/composables/useGenreList'
import { impactPlanFrom } from '../../core/composables/impact'
import { fromInputValue, toDateInput, toDateTimeInput } from '../../core/composables/dates'
import { priorityOptions } from '../../core/composables/labels'
import { ensureGenreTaskType } from '../../core/composables/taskTypes'
import { memoEmbedUrl, memoTitleFromUrl, persistPendingMemoLinks, upsertMemoLink } from '../../memo/composables/memoBridge'
import type { MemoLinkDraft } from '../../memo/composables/memoBridge'
import { useMemoIntegration } from '../../memo/composables/useMemoIntegration'
import { useNotify } from '../../shared/composables/useNotify'

// create/editで共通のTaskEditor
const props = withDefaults(defineProps<{ task?: ApiTask; defaults?: { genreId?: string; parentTaskId?: string } }>(), { task: undefined, defaults: () => ({}) })
const emit = defineEmits<{ saved: [task: ApiTask] }>()
const api = useTasmaneApi()
const router = useRouter()
const notify = useNotify()
const { genres, loaded: genresLoaded, upsert: upsertGenre } = useGenreList()

const ROOT = '__root__'
const NONE = '__none__'
const AUTO = '__auto__'
const taskTypes = ref<{ id: string; name: string }[]>([])
const genreTasks = ref<ApiTask[]>([])
const relatedLoading = ref(false)
const busy = ref(false)
const errorMessage = ref('')
const impact = ref<ImpactPlan>()
const remoteChanged = ref(false)
const saved = ref(false)
// メモ紐付けの失敗時も作成済みタスクのIDを保持し、再送で重複作成しない
const createdTask = ref<ApiTask>()

// メモアプリ連携（紐付けたメモ・埋め込みでの新規作成）
type LinkedMemo = MemoLinkDraft & { id?: string }
const { enabled: memoEnabled, setEnabled: setMemoEnabled, ensureLoaded: loadMemoSetting, memoOrigin, openedMemo, openMemo, closeMemo, savedMemo } = useMemoIntegration()
const memoLinks = ref<LinkedMemo[]>([])
const memoTab = ref<'app' | 'text'>('app')
const composerOpen = ref(!props.task)
const composerKey = ref(0)
const composer = ref<{ requestSave: () => Promise<{ title: string; url: string } | null> }>()
const newMemoSrc = computed(() => memoEmbedUrl(null, memoOrigin.value))
const memoUrlInput = ref('')
const memoTabs = [{ label: 'メモアプリ', value: 'app', icon: 'i-lucide-sticky-note' }, { label: 'テキスト', value: 'text', icon: 'i-lucide-pencil' }]

// 作成画面からの連携オン/オフ（ユーザー設定に保存）
const memoToggleBusy = ref(false)
async function toggleMemoIntegration(value: boolean) {
  memoToggleBusy.value = true
  try {
    await setMemoEnabled(value)
    if (value) { memoTab.value = 'app'; composerOpen.value = true }
    notify.success(value ? 'メモアプリとの連携をオンにしました' : 'メモアプリとの連携をオフにしました')
  } catch (error) { notify.error(error, '連携設定を保存できませんでした。') }
  finally { memoToggleBusy.value = false }
}

// 紐付けの保存（タスク作成前は下書き。保存済みタスクではAPI成功後に表示）
async function persistMemoLink(link: LinkedMemo): Promise<boolean> {
  const taskId = props.task?.id ?? createdTask.value?.id
  if (!taskId) {
    memoLinks.value = upsertMemoLink(memoLinks.value, link)
    return true
  }
  try {
    const stored = await api.addMemoLink(taskId, { url: link.url, title: link.title })
    memoLinks.value = upsertMemoLink(memoLinks.value, { ...link, id: stored.id, url: stored.url, title: stored.title })
    return true
  } catch (error) {
    notify.error(error, 'メモを紐付けられませんでした。')
    return false
  }
}
// 埋め込みメモアプリでの保存通知
function onMemoSaved(memo: { title: string; url: string }) { void persistMemoLink({ url: memo.url, title: memo.title }) }
// 新しいメモの作成欄を開き直す
function newComposer() { composerOpen.value = true; composerKey.value++ }
// URL貼り付けでの紐付け
async function addMemoUrl() {
  const url = memoUrlInput.value.trim()
  if (!url) return true
  const title = memoTitleFromUrl(url)
  if (!title) { notify.error(undefined, 'メモのURLの形式が正しくありません。'); return false }
  const persisted = await persistMemoLink({ url, title })
  if (persisted) memoUrlInput.value = ''
  return persisted
}

// タスク保存の前に、書きかけのメモと入力途中のURLを紐付けに反映
async function flushMemoDrafts() {
  if (!(await addMemoUrl())) return false
  if (memoEnabled.value && memoTab.value === 'app' && composerOpen.value && composer.value) {
    const memo = await composer.value.requestSave()
    if (memo) await persistMemoLink({ url: memo.url, title: memo.title })
  }
  return true
}
// 紐付けの解除（メモ自体は残る）
async function removeMemoLink(link: LinkedMemo) {
  const taskId = props.task?.id ?? createdTask.value?.id
  if (taskId && link.id) {
    try { await api.deleteMemoLink(taskId, link.id) }
    catch (error) { notify.error(error, '紐付けを外せませんでした。'); return }
  }
  memoLinks.value = memoLinks.value.filter(item => item.url !== link.url)
  if (openedMemo.value?.url === link.url) closeMemo()
}
// 右側パネルで保存されたメモのタイトル反映
watch(savedMemo, (value) => {
  if (value && memoLinks.value.some(item => item.url === value.url)) void persistMemoLink({ url: value.url, title: value.title })
})
onMounted(async () => {
  await loadMemoSetting()
  if (!props.task) return
  try { memoLinks.value = await api.memoLinks(props.task.id) }
  catch (error) {
    notify.error(error, 'メモ一覧を取得できませんでした。')
    errorMessage.value = 'メモ一覧を取得できませんでした。再読み込みするか、Go APIのデプロイ状況を確認してください。'
  }
})

// フォーム初期値
function initialState(task?: ApiTask) {
  const isAllDay = task?.isAllDay ?? true
  const toInput = isAllDay ? toDateInput : toDateTimeInput
  return {
    title: task?.title ?? '', body: task?.body ?? '', genreId: task?.genreId ?? props.defaults.genreId ?? '',
    parentTaskId: task?.parentTaskId ?? props.defaults.parentTaskId ?? ROOT, taskTypeId: task?.taskTypeId ?? NONE,
    priority: task?.manualPriority ?? AUTO, startAt: toInput(task?.startAt), endAt: toInput(task?.endAt), isAllDay,
  }
}
const form = reactive(initialState(props.task))
let baseline = JSON.stringify(form)
const dirty = computed(() => JSON.stringify(form) !== baseline)

// フォームを最新タスクへ同期
function reset(task = props.task) {
  Object.assign(form, initialState(task))
  baseline = JSON.stringify(form)
  remoteChanged.value = false
}

// 他タブ・他操作でversionが進んだときの同期（編集中なら通知のみ）
watch(() => props.task?.version, (version, previous) => {
  if (!version || !previous || version === previous) return
  if (dirty.value && !saved.value) remoteChanged.value = true
  else reset()
  saved.value = false
})

// ジャンル一覧（共有stateが未取得なら取得）
onMounted(async () => {
  if (genresLoaded.value) return
  try { genres.value = await api.genres(); genresLoaded.value = true } catch { errorMessage.value = 'ジャンルを取得できませんでした。' }
})

// ジャンル連動の親タスクとタイプ取得
watch(() => form.genreId, async (genreId, previous) => {
  if (previous !== undefined && genreId !== props.task?.genreId) { form.parentTaskId = ROOT; form.taskTypeId = NONE }
  if (!genreId) { taskTypes.value = []; genreTasks.value = []; return }
  relatedLoading.value = true
  try {
    const [types, tasks] = await Promise.all([api.taskTypes(genreId), api.search({ genreIds: genreId })])
    if (genreId !== form.genreId) return
    taskTypes.value = types
    genreTasks.value = tasks
  } catch { errorMessage.value = 'ジャンル関連データを取得できませんでした。' }
  finally { relatedLoading.value = false }
}, { immediate: true })

// 終日切替時に入力形式を変換
watch(() => form.isAllDay, (isAllDay) => {
  const convert = (value: string) => {
    if (!value) return ''
    if (isAllDay) return value.slice(0, 10)
    return value.length === 10 ? `${value}T09:00` : value
  }
  form.startAt = convert(form.startAt)
  form.endAt = convert(form.endAt)
})

const genreItems = computed(() => genres.value.map(genre => ({ label: genre.name, value: genre.id })))

// 親タスク候補（自身と子孫は循環になるため除外、完了済みは子追加不可のため除外）
const parentItems = computed(() => {
  const byId = new Map(genreTasks.value.map(task => [task.id, task]))
  const path = (task: ApiTask) => {
    const names: string[] = []
    let current = task.parentTaskId ? byId.get(task.parentTaskId) : undefined
    while (current && names.length < 10) { names.unshift(current.title); current = current.parentTaskId ? byId.get(current.parentTaskId) : undefined }
    return names.join(' / ')
  }
  const isSelfOrDescendant = (task: ApiTask) => {
    let current: ApiTask | undefined = task
    let guard = 0
    while (current && guard++ < 1000) {
      if (current.id === props.task?.id) return true
      current = current.parentTaskId ? byId.get(current.parentTaskId) : undefined
    }
    return false
  }
  const candidates = genreTasks.value
    .filter(task => task.status !== 'COMPLETED' && !isSelfOrDescendant(task))
    .map(task => ({ label: task.title, value: task.id, description: path(task) || undefined }))
  return [{ label: 'なし（Root Task）', value: ROOT, icon: 'i-lucide-git-branch' }, ...candidates]
})
const typeItems = computed(() => [{ label: '指定なし', value: NONE }, ...taskTypes.value.map(type => ({ label: type.name, value: type.id }))])

// タスクタイプをその場で作成してジャンルへ紐付け・選択
const creatingType = ref(false)
async function createType(name: string) {
  const genreId = form.genreId
  if (!genreId || creatingType.value) return
  creatingType.value = true
  try {
    const type = await ensureGenreTaskType(api, genreId, name)
    if (genreId !== form.genreId) return
    if (!taskTypes.value.some(item => item.id === type.id)) taskTypes.value = [...taskTypes.value, type]
    form.taskTypeId = type.id
    notify.success(`タスクタイプ「${type.name}」を追加しました`)
  } catch (error) { notify.error(error, 'タスクタイプを作成できませんでした。') }
  finally { creatingType.value = false }
}
const priorityItems = [{ label: '自動（期限・進捗・依存から算出）', value: AUTO }, ...priorityOptions]

// 入力検証
const dateError = computed(() => {
  if (!form.startAt || !form.endAt) return ''
  const start = fromInputValue(form.startAt, form.isAllDay, 'start')!
  const end = fromInputValue(form.endAt, form.isAllDay, 'end')!
  return start > end ? '期限は開始以降の日時にしてください。' : ''
})
const canSubmit = computed(() => Boolean(form.title.trim() && form.genreId && !dateError.value && (!props.task || dirty.value)))

// ジャンルの新規作成（仕様: Genreは検索・新規作成可能）
async function createGenre(name: string) {
  const value = name.trim()
  if (!value) return
  try {
    const genre = await api.createGenre(value)
    upsertGenre(genre)
    form.genreId = genre.id
    notify.success(`ジャンル「${genre.name}」を作成しました`)
  } catch (error) { notify.error(error, 'ジャンルを作成できませんでした。同名のジャンルがないか確認してください。') }
}

// 保存リクエスト本文の組み立て
function requestBody(token?: string): TaskInput {
  return {
    title: form.title.trim(), body: form.body || null, genreId: form.genreId,
    taskTypeId: form.taskTypeId === NONE ? null : form.taskTypeId,
    parentTaskId: form.parentTaskId === ROOT ? null : form.parentTaskId,
    manualPriority: form.priority === AUTO ? null : form.priority,
    startAt: fromInputValue(form.startAt, form.isAllDay, 'start'), endAt: fromInputValue(form.endAt, form.isAllDay, 'end'),
    isAllDay: form.isAllDay, ...(token ? { impactToken: token } : {}),
  }
}

// タスクの作成または更新
async function save(token?: string) {
  if (busy.value || (!canSubmit.value && !token)) return
  busy.value = true
  errorMessage.value = ''
  try {
    if (!(await flushMemoDrafts())) return
    const body = requestBody(token)
    let result: ApiTask
    if (props.task) {
      result = await api.updateTask(props.task.id, { ...body, expectedVersion: props.task.version })
    } else if (createdTask.value) {
      // 以前のメモ保存が失敗している場合は同じタスクに再試行する
      result = dirty.value
        ? await api.updateTask(createdTask.value.id, { ...body, expectedVersion: createdTask.value.version })
        : createdTask.value
      createdTask.value = result
      baseline = JSON.stringify(form)
    } else {
      result = await api.createTask(body)
      createdTask.value = result
      baseline = JSON.stringify(form)
    }
    if (!props.task && memoLinks.value.length) {
      const persisted = await persistPendingMemoLinks(memoLinks.value, link =>
        api.addMemoLink(result.id, { url: link.url, title: link.title }))
      memoLinks.value = persisted.links
      if (persisted.failed.length) {
        errorMessage.value = `タスク「${result.title}」は作成済みですが、メモ${persisted.failed.length}件の紐付けに失敗しました。画面を離れず再試行してください（タスクは重複作成されません）。`
        notify.warning('メモの紐付けに失敗しました', 'タスク自体は作成済みです。Go APIのデプロイ状況も確認してください。')
        return
      }
      // GETでも保存が確認できるまで画面を遷移しない
      let verified: Awaited<ReturnType<typeof api.memoLinks>>
      try { verified = await api.memoLinks(result.id) }
      catch (error) {
        notify.error(error, '保存後のメモ紐付け確認に失敗しました。')
        errorMessage.value = 'タスクは作成済みですが、メモ一覧を確認できませんでした。再試行してください。'
        return
      }
      const byUrl = new Map(verified.map(link => [link.url, link]))
      memoLinks.value = memoLinks.value.map(link => {
        const item = byUrl.get(link.url)
        return item ? { ...link, id: item.id, title: item.title } : { ...link, id: undefined }
      })
      if (memoLinks.value.some(link => !link.id)) {
        errorMessage.value = 'タスクは作成済みですが、一部のメモの保存を確認できませんでした。再試行してください。'
        return
      }
    }
    impact.value = undefined
    saved.value = true
    baseline = JSON.stringify(form)
    notify.success(props.task ? '変更を保存しました' : 'タスクを作成しました', result.title)
    emit('saved', result)
    if (!props.task) await router.push(`/tasks/${result.id}`)
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) impact.value = plan
    else notify.error(error, '保存できませんでした。入力内容を確認してください。')
  } finally {
    busy.value = false
  }
}

// Ctrl/⌘ + Enterで保存
defineShortcuts({ meta_enter: { usingInput: true, handler: () => { void save() } } })

// 未保存の変更がある状態での離脱確認
const hasPendingMemoLinks = computed(() => memoLinks.value.some(link => !link.id))
onBeforeRouteLeave(() => {
  if ((dirty.value || hasPendingMemoLinks.value) && !saved.value && !busy.value)
    return globalThis.confirm('未保存の変更またはメモの紐付けがあります。破棄して移動しますか？')
  return true
})
const beforeUnload = (event: BeforeUnloadEvent) => {
  if ((dirty.value || hasPendingMemoLinks.value) && !saved.value) event.preventDefault()
}
onMounted(() => globalThis.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => globalThis.removeEventListener('beforeunload', beforeUnload))
</script>

<template>
  <!-- タスク入力フォーム -->
  <UCard>
    <template v-if="task" #header>
      <div class="flex items-center justify-between gap-2">
        <h2 class="font-semibold text-(--ui-text-highlighted)">タスクの内容</h2>
        <UBadge v-if="dirty" color="warning" variant="subtle" label="未保存の変更" icon="i-lucide-pencil" />
      </div>
    </template>
    <form class="space-y-5" @submit.prevent="save()">
      <UAlert v-if="errorMessage" color="error" variant="subtle" icon="i-lucide-circle-alert" :title="errorMessage" role="alert" />
      <UAlert
        v-if="remoteChanged"
        color="info" variant="subtle" icon="i-lucide-refresh-cw"
        title="このタスクは別の操作で更新されました"
        description="編集中の内容を破棄して最新の内容を読み込むか、このまま編集を続けてください（保存時に競合する場合があります）。"
        :actions="[{ label: '最新を読み込む', color: 'info', variant: 'outline', onClick: () => reset() }]"
      />

      <UFormField label="タイトル" required>
        <UInput v-model="form.title" required maxlength="240" size="xl" placeholder="例: 見積書を作成する" class="w-full" :autofocus="!task" />
      </UFormField>

      <!-- メモ（メモアプリ連携の切替はフォーム項目のラベルと分けて配置） -->
      <div role="group" aria-labelledby="memo-field-label">
        <div class="mb-1.5 flex items-center justify-between gap-2 text-sm">
          <span id="memo-field-label" class="font-medium text-(--ui-text)">メモ</span>
          <USwitch v-if="!task && newMemoSrc" :model-value="memoEnabled" :loading="memoToggleBusy" :disabled="memoToggleBusy" size="sm" label="メモアプリと連携する" @update:model-value="toggleMemoIntegration" />
          <span v-else class="text-(--ui-text-muted)">任意</span>
        </div>
        <div class="space-y-3">
          <!-- 紐付けたメモ（クリックで右側に表示・Ctrl+クリックで別タブ） -->
          <MemoLinkChips :links="memoLinks" :active-url="openedMemo?.url" removable @open="openMemo" @remove="removeMemoLink" />
          <template v-if="memoEnabled && newMemoSrc">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <UTabs v-model="memoTab" :items="memoTabs" :content="false" variant="link" size="sm" />
              <div v-if="memoTab === 'app'" class="flex items-center gap-1">
                <UPopover>
                  <UButton icon="i-lucide-link" color="neutral" variant="ghost" size="sm">URLで紐付け</UButton>
                  <template #content>
                    <form class="flex w-80 gap-2 p-2" @submit.prevent="void addMemoUrl()">
                      <UInput v-model="memoUrlInput" type="url" placeholder="https://memo.kokage-studio.com/memo/…" aria-label="メモのURL" class="flex-1" />
                      <UButton type="submit" icon="i-lucide-plus" aria-label="紐付ける" />
                    </form>
                  </template>
                </UPopover>
                <UButton icon="i-lucide-file-plus" color="neutral" variant="ghost" size="sm" @click="newComposer">新しいメモ</UButton>
              </div>
            </div>
            <div v-if="memoTab === 'app'">
              <MemoAppFrame v-if="composerOpen" ref="composer" :key="composerKey" :src="newMemoSrc" title="新しいメモ" resizable @saved="onMemoSaved" />
              <button v-else type="button" class="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-(--ui-border-accented) p-4 text-sm text-(--ui-text-muted) hover:bg-(--ui-bg-elevated)" @click="newComposer">
                <UIcon name="i-lucide-file-plus" class="size-4" />メモアプリで新しいメモを作成
              </button>
              <p class="mt-1 text-xs text-(--ui-text-dimmed)">メモアプリで保存すると、このタスクに自動で紐付きます。</p>
            </div>
            <UTextarea v-else v-model="form.body" :rows="3" autoresize :maxrows="12" placeholder="詳細・参考リンクなど（Google Tasks のメモに同期）" aria-label="メモ（テキスト）" class="w-full" />
          </template>
          <UTextarea v-else v-model="form.body" :rows="3" autoresize :maxrows="12" placeholder="詳細・参考リンクなど" aria-label="メモ" class="w-full" />
        </div>
      </div>

      <div class="grid gap-5 md:grid-cols-2">
        <UFormField label="ジャンル" required help="一覧にない名前を入力すると新規作成できます。">
          <USelectMenu v-model="form.genreId" :items="genreItems" value-key="value" create-item placeholder="ジャンルを選択" :search-input="{ placeholder: '検索 / 新規作成…' }" class="w-full" @create="createGenre" />
        </UFormField>
        <UFormField label="親タスク" :help="form.genreId ? 'このジャンルの階層から選択します。' : '先にジャンルを選択してください。'">
          <USelectMenu v-model="form.parentTaskId" :items="parentItems" value-key="value" :disabled="!form.genreId" :loading="relatedLoading" :search-input="{ placeholder: 'タスク名で検索…' }" class="w-full" />
        </UFormField>
        <UFormField label="タスクタイプ">
          <USelectMenu
            v-model="form.taskTypeId"
            :items="typeItems"
            value-key="value"
            :disabled="!form.genreId || creatingType"
            :loading="creatingType"
            create-item
            :search-input="{ placeholder: '検索 / 新しいタイプ名を入力' }"
            class="w-full"
            @create="createType"
          />
        </UFormField>
        <UFormField label="優先度">
          <USelect v-model="form.priority" :items="priorityItems" class="w-full" />
        </UFormField>
      </div>

      <div class="rounded-lg border border-(--ui-border) p-4">
        <div class="mb-4 flex items-center justify-between gap-3">
          <p class="text-sm font-medium text-(--ui-text-highlighted)">日程</p>
          <USwitch v-model="form.isAllDay" label="終日" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="開始">
            <DateField v-model="form.startAt" :with-time="!form.isAllDay" />
          </UFormField>
          <UFormField label="期限" :error="dateError || undefined">
            <DateField v-model="form.endAt" :with-time="!form.isAllDay" default-time="18:00" />
          </UFormField>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-end gap-2">
        <span class="mr-auto hidden items-center gap-1 text-xs text-(--ui-text-dimmed) sm:flex"><UKbd value="meta" /><UKbd value="Enter" /> で保存</span>
        <UButton v-if="task && dirty" color="neutral" variant="ghost" @click="reset()">変更を破棄</UButton>
        <UButton type="submit" :loading="busy" :disabled="!canSubmit" icon="i-lucide-check">{{ task ? '変更を保存' : createdTask ? '保存を再試行' : 'タスクを作成' }}</UButton>
      </div>
    </form>
    <!-- 変更影響の確認 -->
    <ImpactModal :open="Boolean(impact)" title="変更の影響を確認" :plan="impact" :loading="busy" @cancel="impact = undefined" @confirm="impact && save(impact.impactToken)" />
  </UCard>
</template>

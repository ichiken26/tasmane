<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from '#imports'
import type { GoogleConflict, GoogleImportCandidate, GoogleResolution, GoogleSnapshot, GoogleStatus, ImpactPlan } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { apiErrorStatus } from '../../core/composables/apiError'
import { deletionImpact, impactPlanFrom } from '../../core/composables/impact'
import { formatDate } from '../../core/composables/dates'
import { useGenreList } from '../../core/composables/useGenreList'
import { useNotify } from '../../shared/composables/useNotify'

// Google Tasks 同期状態・コンフリクト解決・取込候補の取り込み
const api = useTasmaneApi()
const router = useRouter()
const notify = useNotify()
const { genres, loaded: genresLoaded } = useGenreList()
const status = ref<GoogleStatus>()
const conflicts = ref<GoogleConflict[]>([])
const imports = ref<GoogleImportCandidate[]>([])
const loading = ref(true)
const loadError = ref<unknown>()
const syncing = ref(false)
const busyKey = ref('')
const importGenre = ref<Record<string, string>>({})
// 「タスマネ削除」は通常の削除（2段階確認）を通す
const deleting = ref<GoogleConflict>()
const deleteImpact = ref<ImpactPlan>()

// 削除確認の表示（操作対象自身は一覧から外し、名前として別に示す）
const deleteView = computed(() => deleteImpact.value && deleting.value ? deletionImpact(deleteImpact.value, deleting.value.taskId) : undefined)
const shownDeleteImpact = computed<ImpactPlan | undefined>(() => deleteImpact.value && deleteView.value
  ? { ...deleteImpact.value, affectedEntities: deleteView.value.together, warnings: deleteView.value.warnings }
  : undefined)

const genreItems = computed(() => genres.value.map(genre => ({ label: genre.name, value: genre.id })))
const codeLabel: Record<string, string> = {
  GOOGLE_DELETED: 'Google側で削除', FIELD_CONFLICT: '双方で変更', DUE_HIERARCHY: '期限が親子関係と矛盾', COMPLETE_BLOCKED: 'Googleで完了したが完了条件未達',
}
const fieldLabel: Record<string, string> = { title: 'タイトル', notes: 'メモ', due: '期限', status: '状態' }
const statusLabel = (value?: string) => value === 'completed' ? '完了' : value === 'needsAction' ? '未完了' : value || '—'
const fieldValue = (snapshot: GoogleSnapshot, key: string) => key === 'status' ? statusLabel(snapshot.status) : (snapshot as Record<string, string | undefined>)[key] || '—'
const changedFields = (item: GoogleConflict) => Object.keys(fieldLabel).filter(key => fieldValue(item.local, key) !== fieldValue(item.remote, key))
const candidateKey = (item: GoogleImportCandidate) => `${item.googleTaskListId}:${item.googleTaskId}`
const snapshotOf = (item: GoogleImportCandidate) => (item.snapshot ?? {}) as GoogleSnapshot

// 状態・コンフリクト・候補の読込
async function load() {
  loading.value = true
  loadError.value = undefined
  try {
    const [current, conflictRows, importRows] = await Promise.all([api.googleStatus(), api.googleConflicts(), api.googleImportCandidates()])
    status.value = current
    conflicts.value = conflictRows
    imports.value = importRows
    if (!genresLoaded.value) { genres.value = await api.genres(); genresLoaded.value = true }
  } catch (error) { loadError.value = error }
  finally { loading.value = false }
}
onMounted(load)

// 手動同期（最終FAILEDの再送を含む）
async function requestSync() {
  syncing.value = true
  try {
    await api.googleSync()
    notify.success('Google Tasks同期を受け付けました', '失敗していたタスクも再送します。完了まで少し時間がかかる場合があります。')
  } catch (error) {
    if (apiErrorStatus(error) === 409) notify.info('同期はすでに実行中です', '完了までしばらくお待ちください。')
    else notify.error(error, '同期を開始できませんでした。時間をおいて再試行してください。')
  } finally { syncing.value = false }
}

// 専用リストの作り直し（Google側で削除された場合の復旧）
const recreating = ref(false)
async function recreateList() {
  recreating.value = true
  try {
    await api.recreateGoogleTaskList()
    if (status.value) status.value.taskListMissing = false
    notify.success('専用リストを作り直しています', 'すべてのタスクをGoogle Tasksへ作成し直します。')
  } catch (error) { notify.error(error, '専用リストを作り直せませんでした。') }
  finally { recreating.value = false }
}

// コンフリクト解決
async function resolve(item: GoogleConflict, resolution: GoogleResolution, message: string) {
  busyKey.value = `${item.taskId}:${resolution}`
  try {
    await api.resolveGoogleConflict(item.taskId, resolution)
    conflicts.value = conflicts.value.filter(value => value.taskId !== item.taskId)
    notify.success(message)
  } catch (error) { notify.error(error, 'コンフリクトを解決できませんでした。最新の状態を読み込み直してください。') }
  finally { busyKey.value = '' }
}

// Google側の値をタスマネへ反映（通常の更新・影響確認を通すためタスク詳細で編集）
function adoptGoogle(item: GoogleConflict) {
  void router.push({ path: `/tasks/${item.taskId}`, query: { googleConflict: '1' } })
}

// 「タスマネ削除」: 通常の削除UseCase（影響範囲の確認つき）
async function removeTask(item: GoogleConflict, token?: string) {
  busyKey.value = `${item.taskId}:DELETE`
  try {
    const task = await api.task(item.taskId)
    await api.deleteTask(item.taskId, task.version, token)
    deleting.value = undefined
    deleteImpact.value = undefined
    conflicts.value = conflicts.value.filter(value => value.taskId !== item.taskId)
    notify.success('タスクを削除しました', '30日以内ならゴミ箱から復元できます。')
  } catch (error) {
    const plan = impactPlanFrom(error)
    if (plan) { deleteImpact.value = plan; return }
    notify.error(error, '削除できませんでした。')
  } finally { busyKey.value = '' }
}

// 取込候補の取り込み（ユーザーがGenreを選択）
async function importCandidate(item: GoogleImportCandidate) {
  const genreId = importGenre.value[candidateKey(item)]
  if (!genreId) return
  busyKey.value = candidateKey(item)
  try {
    const task = await api.importGoogleCandidate(item, genreId)
    imports.value = imports.value.filter(value => candidateKey(value) !== candidateKey(item))
    notify.success(`「${task.title}」を取り込みました`)
  } catch (error) { notify.error(error, '取り込めませんでした。同期後に再読み込みしてください。') }
  finally { busyKey.value = '' }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-semibold text-(--ui-text-highlighted)">Google Tasks</h2>
          <p class="text-xs text-(--ui-text-muted)">
            <template v-if="status?.connected">最終同期: {{ status.lastSyncFinishedAt ? formatDate(status.lastSyncFinishedAt, true) : '未同期' }}<span v-if="status.pendingJobs"> ・ 送信待ち {{ status.pendingJobs }}件</span></template>
            <template v-else>Googleでログインすると、専用リスト「タスマネ」と同期します。</template>
          </p>
        </div>
        <div class="flex gap-2">
          <UButton icon="i-lucide-rotate-cw" color="neutral" variant="ghost" aria-label="再読み込み" :loading="loading" @click="load" />
          <UButton icon="i-lucide-refresh-cw" :loading="syncing" :disabled="!status?.connected" @click="requestSync">今すぐ同期</UButton>
        </div>
      </div>
    </template>
    <div v-if="loading" class="space-y-3"><USkeleton v-for="index in 3" :key="index" class="h-14 w-full" /></div>
    <ErrorState v-else-if="loadError" :error="loadError" fallback="同期情報を取得できませんでした。" @retry="load" />
    <div v-else class="space-y-6">
      <!-- 専用リスト喪失（復旧フロー） -->
      <UAlert v-if="status?.taskListMissing" color="error" variant="subtle" icon="i-lucide-list-x" title="Google Tasksの専用リスト「タスマネ」が見つかりません" description="Google側で削除された可能性があります。作り直すと、すべてのタスクを新しいリストへ作成し直します（連携解除したタスクを除く）。">
        <template #actions>
          <UButton size="xs" color="error" icon="i-lucide-list-restart" :loading="recreating" @click="recreateList">専用リストを作り直す</UButton>
        </template>
      </UAlert>
      <!-- 最終失敗 -->
      <UAlert v-if="status?.failed.length" color="error" variant="subtle" icon="i-lucide-cloud-off" :title="`${status.failed.length}件のタスクをGoogleへ同期できませんでした`" description="「今すぐ同期」で再送します。繰り返し失敗する場合は内容を確認してください。">
        <template #description>
          <ul class="mt-1 space-y-0.5 text-xs">
            <li v-for="item in status.failed" :key="item.taskId"><NuxtLink :to="`/tasks/${item.taskId}`" class="underline">{{ item.title }}</NuxtLink>：{{ item.message }}</li>
          </ul>
        </template>
      </UAlert>

      <!-- 同期コンフリクト -->
      <section>
        <h3 class="mb-2 flex items-center gap-2 text-sm font-medium text-(--ui-text-highlighted)">同期コンフリクト <UBadge v-if="conflicts.length" :label="String(conflicts.length)" color="warning" variant="subtle" size="sm" /></h3>
        <ul v-if="conflicts.length" class="space-y-3">
          <li v-for="item in conflicts" :key="item.taskId" class="rounded-lg border border-(--ui-border) p-3">
            <div class="flex flex-wrap items-center gap-2">
              <UBadge :label="codeLabel[item.code] || item.code" color="warning" variant="subtle" size="sm" />
              <NuxtLink :to="`/tasks/${item.taskId}`" class="min-w-0 flex-1 truncate font-medium hover:underline">{{ item.taskTitle }}</NuxtLink>
            </div>
            <!-- 双方の値 -->
            <table v-if="item.code !== 'GOOGLE_DELETED'" class="mt-2 w-full text-xs">
              <thead class="text-(--ui-text-muted)"><tr><th class="w-16 py-1 text-left font-normal">項目</th><th class="py-1 text-left font-normal">タスマネ</th><th class="py-1 text-left font-normal">Google</th></tr></thead>
              <tbody>
                <tr v-for="key in changedFields(item)" :key="key" class="border-t border-(--ui-border)">
                  <td class="py-1 text-(--ui-text-muted)">{{ fieldLabel[key] }}</td>
                  <td class="py-1 pr-2">{{ fieldValue(item.local, key) }}</td>
                  <td class="py-1">{{ fieldValue(item.remote, key) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-2 text-xs text-(--ui-text-muted)">Google Tasks側でこのタスクが削除されました。タスマネでの扱いを選んでください。</p>
            <div class="mt-3 flex flex-wrap gap-2">
              <template v-if="item.code === 'GOOGLE_DELETED'">
                <UButton size="xs" color="error" variant="soft" icon="i-lucide-trash-2" :loading="busyKey === `${item.taskId}:DELETE`" @click="deleting = item; removeTask(item)">タスマネでも削除</UButton>
                <UButton size="xs" color="neutral" variant="soft" icon="i-lucide-unlink" :loading="busyKey === `${item.taskId}:UNLINK`" @click="resolve(item, 'UNLINK', 'タスマネに残し、Google連携を解除しました')">タスマネに残して連携解除</UButton>
                <UButton size="xs" icon="i-lucide-cloud-upload" :loading="busyKey === `${item.taskId}:RECREATE_GOOGLE`" @click="resolve(item, 'RECREATE_GOOGLE', 'Google Tasksに作り直します')">Googleに再作成</UButton>
              </template>
              <template v-else>
                <UButton size="xs" icon="i-lucide-upload" :loading="busyKey === `${item.taskId}:KEEP_TASMANE`" @click="resolve(item, 'KEEP_TASMANE', 'タスマネの値でGoogleを更新します')">タスマネの値を残す</UButton>
                <UButton size="xs" color="neutral" variant="soft" icon="i-lucide-download" @click="adoptGoogle(item)">Googleの値で編集</UButton>
              </template>
            </div>
          </li>
        </ul>
        <p v-else class="text-sm text-(--ui-text-muted)">未解決のコンフリクトはありません。</p>
      </section>

      <!-- 取込候補 -->
      <section>
        <h3 class="mb-1 text-sm font-medium text-(--ui-text-highlighted)">Googleにだけあるタスク</h3>
        <p class="mb-2 text-xs text-(--ui-text-muted)">ジャンルを選ぶと、タスマネのタスクとして取り込みます。</p>
        <ul v-if="imports.length" class="divide-y divide-(--ui-border) rounded-lg border border-(--ui-border)">
          <li v-for="item in imports" :key="candidateKey(item)" class="flex flex-col gap-2 px-3 py-2.5 sm:flex-row sm:items-center">
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ snapshotOf(item).title || '(無題)' }}</p>
              <p class="text-xs text-(--ui-text-muted)"><span v-if="snapshotOf(item).due">期限 {{ snapshotOf(item).due }} ・ </span>{{ statusLabel(snapshotOf(item).status) }}</p>
            </div>
            <USelect v-model="importGenre[candidateKey(item)]" :items="genreItems" placeholder="ジャンルを選択" class="w-full sm:w-44" :aria-label="`${snapshotOf(item).title} の取り込み先ジャンル`" />
            <UButton size="sm" icon="i-lucide-import" :disabled="!importGenre[candidateKey(item)]" :loading="busyKey === candidateKey(item)" @click="importCandidate(item)">取り込む</UButton>
          </li>
        </ul>
        <p v-else class="text-sm text-(--ui-text-muted)">取り込み待ちのタスクはありません。</p>
      </section>
    </div>
    <!-- タスマネ削除の影響確認（2段階目） -->
    <ImpactModal
      :open="Boolean(deleteImpact)"
      :title="deleteView?.together.length ? 'このタスクと一緒に、次のタスクも削除されます' : 'このタスクを削除しますか？'"
      description="削除後30日以内ならゴミ箱から復元できます。キャンセルした場合は何も変更されません。"
      :target="deleting?.taskTitle"
      :list-label="deleteView?.together.length ? '一緒に削除されるタスク' : undefined"
      :plan="shownDeleteImpact"
      :loading="Boolean(busyKey)"
      danger
      :confirm-label="deleteView?.together.length ? 'すべて削除する' : '削除する'"
      @cancel="deleteImpact = undefined; deleting = undefined"
      @confirm="deleting && removeTask(deleting, deleteImpact?.impactToken)"
    />
  </UCard>
</template>

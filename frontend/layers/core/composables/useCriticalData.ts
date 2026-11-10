import { shouldRedirectToLogin } from './loginRedirect'
import { onMounted } from 'vue'
import { useAsyncData, useRequestFetch, useRuntimeConfig } from '#app'
import { schedulePrefetch } from './criticalOrder'

// クリティカルデータの取得と後続プリフェッチ
export function useCriticalData<T>(key: string, critical: () => Promise<T>, prefetch?: () => Promise<unknown>) {
  const state = useAsyncData(key, critical)
  if (prefetch) onMounted(() => { void schedulePrefetch(prefetch) })
  return state
}

export interface ApiTask { id: string; genreId: string; taskTypeId?: string | null; parentTaskId?: string | null; title: string; body: string | null; status: string; progress: number | null; startAt?: string | null; endAt?: string | null; isAllDay: boolean; manualPriority?: string | null; effectivePriority: string; isOverdue: boolean; showFire: boolean; version: number; taskScore?: number; recommendationReasons?: string[]; recommended?: boolean; canComplete?: boolean; isLeaf?: boolean; warnings?: string[] }
export interface ApiGenre { id: string; name: string }
export interface ApiDependency { taskId: string; dependsOnTaskId: string }
export interface ApiAdvice { content: string; generatedAt: string }
export interface ApiDashboard { tasks: ApiTask[]; confirmations: ApiConfirmation[]; advice: ApiAdvice | null }
export interface ApiConfirmation { id: string; taskId: string; taskTitle: string; content: string; targetName: string; status: string; dueAt: string | null }
export interface ImpactPlan { code: 'IMPACT_CONFIRMATION_REQUIRED'; impactToken: string; affectedEntities: { id: string; type: string; title?: string | null; reason?: string | null }[]; warnings: string[] }
export interface TaskInput { title: string; body?: string | null; genreId: string; taskTypeId?: string | null; parentTaskId?: string | null; startAt?: string | null; endAt?: string | null; isAllDay: boolean; manualPriority?: string | null; impactToken?: string }
export interface UserSettings { timezone: string; googleSyncIntervalMinutes: number; memoIntegrationEnabled: boolean }
export interface ApiMemoLink { id: string; taskId: string; url: string; title: string; memoDate: string | null; createdAt: string }
export interface GoogleSnapshot { title?: string; notes?: string; due?: string; status?: string }
export interface GoogleConflict { taskId: string; code: string; message: string; taskTitle: string; local: GoogleSnapshot; remote: GoogleSnapshot }
export type GoogleResolution = 'KEEP_TASMANE' | 'RECREATE_GOOGLE' | 'UNLINK'
export interface GoogleStatus { connected: boolean; taskListMissing?: boolean; lastSyncFinishedAt: string | null; conflictCount: number; pendingJobs: number; failed: { taskId: string; title: string; message: string }[] }
export interface DeletedTask { id: string; genreId: string; parentTaskId: string | null; title: string; status: string; endAt: string | null; deletedAt: string; purgeAt: string }
export interface ConfirmationTarget { id: string; name: string }
export interface GoogleImportCandidate { googleTaskListId: string; googleTaskId: string; snapshot: unknown }

type RequestOptions = { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; headers?: Record<string, string>; body?: Record<string, unknown>; query?: Record<string, unknown> }
const enc = encodeURIComponent
// 変更系APIの成功時に window へ送るイベント名
export const DATA_CHANGED_EVENT = 'tasmane:data-changed'

// タスマネAPIクライアント（SSR時はCookieを転送し、401はログインへ誘導）
export function useTasmaneApi() {
  const config = useRuntimeConfig()
  const baseURL = String(config.public.apiBase || '/api/v1')
  const requestFetch = useRequestFetch()
  const request = <T>(path: string, options: RequestOptions = {}) => requestFetch<T>(path, {
    ...options,
    baseURL,
    onResponse({ response }) {
      // 変更系の成功を同じタブ内へ通知（フッターの進捗概要などの再取得用）
      if (import.meta.client && response.ok && options.method && options.method !== 'GET') globalThis.dispatchEvent(new globalThis.Event(DATA_CHANGED_EVENT))
    },
    onResponseError({ response }) {
      if (import.meta.client && response.status === 401 && shouldRedirectToLogin({ prerendering: Boolean((globalThis.document as { prerendering?: boolean } | undefined)?.prerendering), pathname: globalThis.location.pathname }))
        globalThis.location.assign(`/auth/login?returnTo=${enc(globalThis.location.pathname + globalThis.location.search)}`)
    },
  }) as Promise<T>
  return {
    task: (id: string) => request<ApiTask>(`/tasks/${enc(id)}`),
    tree: (id: string) => request<ApiTask[]>(`/tasks/${enc(id)}/tree`),
    dependencies: (id: string) => request<ApiDependency[]>(`/tasks/${enc(id)}/dependencies`),
    addDependency: (id: string, dependsOnTaskId: string, expectedVersion: number) =>
      request<void>(`/tasks/${enc(id)}/dependencies`, { method: 'POST', body: { dependsOnTaskId, expectedVersion } }),
    removeDependency: (id: string, dependsOnTaskId: string, expectedVersion: number) =>
      request<void>(`/tasks/${enc(id)}/dependencies/${enc(dependsOnTaskId)}`, { method: 'DELETE', query: { expectedVersion } }),
    createTask: (input: TaskInput) => request<ApiTask>('/tasks', { method: 'POST', body: { ...input } }),
    updateTask: (id: string, input: Partial<TaskInput> & { expectedVersion: number }) => request<ApiTask>(`/tasks/${enc(id)}`, { method: 'PATCH', body: input }),
    transitionTask: (id: string, toStatus: string, expectedVersion: number, impactToken?: string) => request<ApiTask>(`/tasks/${enc(id)}/transition`, { method: 'POST', body: { toStatus, expectedVersion, ...(impactToken ? { impactToken } : {}) } }),
    deleteTask: (id: string, expectedVersion: number, impactToken?: string) => request<void>(`/tasks/${enc(id)}`, { method: 'DELETE', query: { expectedVersion, ...(impactToken ? { impactToken } : {}) } }),
    memoLinks: (id: string) => request<ApiMemoLink[]>(`/tasks/${enc(id)}/memo-links`),
    addMemoLink: (id: string, input: { url: string; title?: string }) => request<ApiMemoLink>(`/tasks/${enc(id)}/memo-links`, { method: 'POST', body: input }),
    deleteMemoLink: (id: string, linkId: string) => request<void>(`/tasks/${enc(id)}/memo-links/${enc(linkId)}`, { method: 'DELETE' }),
    confirmations: (id: string) => request<ApiConfirmation[]>(`/tasks/${enc(id)}/confirmations`),
    createConfirmation: (id: string, input: { content: string; targetName: string; dueAt?: string | null }) => request<ApiConfirmation>(`/tasks/${enc(id)}/confirmations`, { method: 'POST', body: input }),
    updateConfirmation: (id: string, status: string) => request<ApiConfirmation>(`/confirmations/${enc(id)}`, { method: 'PATCH', body: { status } }),
    dashboard: (genreId?: string) => request<ApiDashboard>(genreId ? `/dashboard/genres/${enc(genreId)}` : '/dashboard'),
    genres: () => request<ApiGenre[]>('/genres'),
    genre: (id: string) => request<ApiGenre>(`/genres/${enc(id)}`),
    createGenre: (name: string) => request<ApiGenre>('/genres', { method: 'POST', body: { name } }),
    renameGenre: (id: string, name: string) => request<ApiGenre>(`/genres/${enc(id)}`, { method: 'PATCH', body: { name } }),
    deleteGenre: (id: string) => request<void>(`/genres/${enc(id)}`, { method: 'DELETE' }),
    taskTypes: (genreId: string) => request<ApiGenre[]>(`/genres/${enc(genreId)}/task-types`),
    allTaskTypes: () => request<ApiGenre[]>('/task-types'),
    createTaskType: (name: string) => request<ApiGenre>('/task-types', { method: 'POST', body: { name } }),
    attachTaskType: (genreId: string, taskTypeId: string) => request<void>(`/genres/${enc(genreId)}/task-types`, { method: 'POST', body: { taskTypeId } }),
    search: (query: Record<string, string>) => request<ApiTask[]>('/tasks', { query }),
    settings: () => request<UserSettings>('/settings'),
    updateSettings: (input: Partial<UserSettings>) => request<UserSettings>('/settings', { method: 'PATCH', body: input }),
    googleSync: () => request<void>('/google/sync', { method: 'POST' }),
    googleConflicts: () => request<GoogleConflict[]>('/google/conflicts'),
    googleImportCandidates: () => request<GoogleImportCandidate[]>('/google/import-candidates'),
    googleStatus: () => request<GoogleStatus>('/google/status'),
    recreateGoogleTaskList: () => request<void>('/google/task-list/recreate', { method: 'POST' }),
    resolveGoogleConflict: (taskId: string, resolution: GoogleResolution) => request<void>(`/google/conflicts/${enc(taskId)}/resolve`, { method: 'POST', body: { resolution } }),
    importGoogleCandidate: (candidate: GoogleImportCandidate, genreId: string) => request<ApiTask>('/google/import-candidates/import', { method: 'POST', body: { googleTaskListId: candidate.googleTaskListId, googleTaskId: candidate.googleTaskId, genreId } }),
    setProgress: (id: string, progress: number, expectedVersion: number) => request<ApiTask>(`/tasks/${enc(id)}/progress`, { method: 'PUT', body: { progress, expectedVersion } }),
    deletedTasks: () => request<DeletedTask[]>('/deleted-tasks'),
    restoreTask: (id: string) => request<{ restoredTaskIds: string[] }>(`/deleted-tasks/${enc(id)}/restore`, { method: 'POST' }),
    confirmationTargets: () => request<ConfirmationTarget[]>('/confirmation-targets'),
    createConfirmationTarget: (name: string) => request<ConfirmationTarget>('/confirmation-targets', { method: 'POST', body: { name } }),
    deleteConfirmationTarget: (id: string) => request<void>(`/confirmation-targets/${enc(id)}`, { method: 'DELETE' }),
    renameTaskType: (id: string, name: string) => request<ApiGenre>(`/task-types/${enc(id)}`, { method: 'PATCH', body: { name } }),
    deleteTaskType: (id: string) => request<void>(`/task-types/${enc(id)}`, { method: 'DELETE' }),
    detachTaskType: (genreId: string, taskTypeId: string) => request<void>(`/genres/${enc(genreId)}/task-types/${enc(taskTypeId)}`, { method: 'DELETE' }),
  }
}

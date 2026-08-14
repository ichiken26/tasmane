import { onMounted } from 'vue'
import { useAsyncData, useRuntimeConfig } from '#app'
import { schedulePrefetch } from './criticalOrder'

export function useCriticalData<T>(key: string, critical: () => Promise<T>, prefetch?: () => Promise<unknown>) {
  const state = useAsyncData(key, critical)
  if (prefetch) onMounted(() => { void schedulePrefetch(prefetch) })
  return state
}

export interface ApiTask { id: string; genreId: string; taskTypeId?: string | null; parentTaskId?: string | null; title: string; body: string | null; status: string; progress: number | null; startAt?: string | null; endAt?: string | null; isAllDay: boolean; manualPriority?: string | null; effectivePriority: string; isOverdue: boolean; showFire: boolean; version: number; taskScore?: number; recommendationReasons?: string[]; recommended?: boolean }
export interface ApiGenre { id: string; name: string }
export interface ApiDependency { taskId: string; dependsOnTaskId: string }
export interface ApiAdvice { content: string; generatedAt: string }
export interface ApiConfirmation { id: string; taskId: string; taskTitle: string; content: string; targetName: string; status: string; dueAt: string | null }
export interface ImpactPlan { code: 'IMPACT_CONFIRMATION_REQUIRED'; impactToken: string; affectedEntities: { id: string; type: string; title?: string | null; reason?: string | null }[]; warnings: string[] }
export interface TaskInput { title: string; body?: string | null; genreId: string; taskTypeId?: string | null; parentTaskId?: string | null; startAt?: string | null; endAt?: string | null; isAllDay: boolean; manualPriority?: string | null; impactToken?: string }
export interface UserSettings { timezone: string; googleSyncIntervalMinutes: number }
export interface GoogleConflict { taskId: string; code: string; message: string }
export interface GoogleImportCandidate { googleTaskListId: string; googleTaskId: string; snapshot: unknown }

export function useTasmaneApi() {
  const config = useRuntimeConfig()
  const baseURL = String(config.public.apiBase || '/v1')
  return {
    task: (id: string) => $fetch<ApiTask>(`${baseURL}/tasks/${encodeURIComponent(id)}`),
    tree: (id: string) => $fetch<ApiTask[]>(`${baseURL}/tasks/${encodeURIComponent(id)}/tree`),
    dependencies: (id: string) => $fetch<ApiDependency[]>(`${baseURL}/tasks/${encodeURIComponent(id)}/dependencies`),
    addDependency: (id: string, dependsOnTaskId: string, expectedVersion: number) =>
      $fetch<void>(`${baseURL}/tasks/${encodeURIComponent(id)}/dependencies`, { method: 'POST', body: { dependsOnTaskId, expectedVersion } }),
    removeDependency: (id: string, dependsOnTaskId: string, expectedVersion: number) =>
      $fetch<void>(`${baseURL}/tasks/${encodeURIComponent(id)}/dependencies/${encodeURIComponent(dependsOnTaskId)}`, { method: 'DELETE', query: { expectedVersion } }),
    createTask: (input: TaskInput) => $fetch<ApiTask>(`${baseURL}/tasks`, { method: 'POST', body: input }),
    updateTask: (id: string, input: Partial<TaskInput> & { expectedVersion: number }) => $fetch<ApiTask>(`${baseURL}/tasks/${encodeURIComponent(id)}`, { method: 'PATCH', body: input }),
    transitionTask: (id: string, toStatus: string, expectedVersion: number, impactToken?: string) => $fetch<ApiTask>(`${baseURL}/tasks/${encodeURIComponent(id)}/transition`, { method: 'POST', body: { toStatus, expectedVersion, ...(impactToken ? { impactToken } : {}) } }),
    deleteTask: (id: string, expectedVersion: number, impactToken?: string) => $fetch<void>(`${baseURL}/tasks/${encodeURIComponent(id)}`, { method: 'DELETE', query: { expectedVersion, ...(impactToken ? { impactToken } : {}) } }),
    confirmations: (id: string) => $fetch<ApiConfirmation[]>(`${baseURL}/tasks/${encodeURIComponent(id)}/confirmations`),
    createConfirmation: (id: string, input: { content: string; targetName: string; dueAt?: string | null }) => $fetch<ApiConfirmation>(`${baseURL}/tasks/${encodeURIComponent(id)}/confirmations`, { method: 'POST', body: input }),
    updateConfirmation: (id: string, status: string) => $fetch<ApiConfirmation>(`${baseURL}/confirmations/${encodeURIComponent(id)}`, { method: 'PATCH', body: { status } }),
    dashboard: (genreId?: string) => $fetch<{ tasks: ApiTask[]; confirmations: ApiConfirmation[]; advice: ApiAdvice | null }>(genreId ? `${baseURL}/dashboard/genres/${encodeURIComponent(genreId)}` : `${baseURL}/dashboard`),
    genres: () => $fetch<ApiGenre[]>(`${baseURL}/genres`),
    taskTypes: (genreId: string) => $fetch<ApiGenre[]>(`${baseURL}/genres/${encodeURIComponent(genreId)}/task-types`),
    search: (query: Record<string, string>) => $fetch<ApiTask[]>(`${baseURL}/tasks`, { query }),
    settings: () => $fetch<UserSettings>(`${baseURL}/settings`),
    updateSettings: (input: Partial<UserSettings>) => $fetch<UserSettings>(`${baseURL}/settings`, { method: 'PATCH', body: input }),
    googleConflicts: () => $fetch<GoogleConflict[]>(`${baseURL}/google/conflicts`),
    googleImportCandidates: () => $fetch<GoogleImportCandidate[]>(`${baseURL}/google/import-candidates`),
  }
}

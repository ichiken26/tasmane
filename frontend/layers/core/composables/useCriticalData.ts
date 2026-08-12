import { onMounted } from 'vue'
import { useAsyncData, useRuntimeConfig } from '#app'
import { schedulePrefetch } from './criticalOrder'

export function useCriticalData<T>(key: string, critical: () => Promise<T>, prefetch?: () => Promise<unknown>) {
  const state = useAsyncData(key, critical)
  if (prefetch) onMounted(() => { void schedulePrefetch(prefetch) })
  return state
}

export interface ApiTask { id: string; genreId: string; title: string; body: string | null; status: string; progress: number | null; startAt?: string | null; endAt?: string | null; effectivePriority: string; isOverdue: boolean; showFire: boolean; version: number }
export interface ApiGenre { id: string; name: string }
export interface ApiDependency { taskId: string; dependsOnTaskId: string }

export function useTasmaneApi() {
  const config = useRuntimeConfig()
  const baseURL = String(config.public.apiBase || '/v1')
  return {
    task: (id: string) => $fetch<ApiTask>(`${baseURL}/tasks/${encodeURIComponent(id)}`),
    tree: (id: string) => $fetch<ApiTask[]>(`${baseURL}/tasks/${encodeURIComponent(id)}/tree`),
    dependencies: (id: string) => $fetch<ApiDependency[]>(`${baseURL}/tasks/${encodeURIComponent(id)}/dependencies`),
    dashboard: (genreId?: string) => $fetch<{ tasks: ApiTask[] }>(genreId ? `${baseURL}/dashboard/genres/${encodeURIComponent(genreId)}` : `${baseURL}/dashboard`),
    genres: () => $fetch<ApiGenre[]>(`${baseURL}/genres`),
    search: (query: Record<string, string>) => $fetch<ApiTask[]>(`${baseURL}/tasks`, { query }),
  }
}

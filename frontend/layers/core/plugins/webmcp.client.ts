import { defineNuxtPlugin, useRuntimeConfig } from '#app'
import { createWebMcpTools, type WebMcpTool } from '../composables/webmcpTools'
import { useWebMcpPermission } from '../composables/useWebMcpPermission'

// WebMCP 対応ブラウザでのツール登録（navigator.modelContext。未対応なら何もしない）
interface ModelContext {
  registerTool?: (tool: WebMcpTool) => unknown
  provideContext?: (context: { tools: WebMcpTool[] }) => unknown
}

export default defineNuxtPlugin(() => {
  const modelContext = (globalThis.navigator as Navigator & { modelContext?: ModelContext } | undefined)?.modelContext
  if (!modelContext) return
  const baseURL = String(useRuntimeConfig().public.apiBase || '/api/v1')
  const { ask } = useWebMcpPermission()
  // 同一オリジンのセッションCookieで REST API を呼ぶ（Worker が WebMCP 経由として履歴へ記録）
  const request = (path: string, options: { method?: 'GET' | 'POST' | 'PUT' | 'PATCH'; body?: Record<string, unknown>; query?: Record<string, string> } = {}) =>
    $fetch(path, { ...options, baseURL, credentials: 'same-origin', headers: { 'x-tasmane-client': 'webmcp' } })
  const tools = createWebMcpTools(request, ask)
  if (typeof modelContext.registerTool === 'function') {
    for (const tool of tools) modelContext.registerTool(tool)
  } else if (typeof modelContext.provideContext === 'function') {
    modelContext.provideContext({ tools })
  }
})

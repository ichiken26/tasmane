import { useToast } from '#imports'
import { apiErrorMessage } from '../../core/composables/apiError'

// 操作結果のトースト通知
export function useNotify() {
  const toast = useToast()
  return {
    success: (title: string, description?: string) => toast.add({ title, description, color: 'success', icon: 'i-lucide-circle-check' }),
    error: (error: unknown, fallback: string) => toast.add({ title: apiErrorMessage(error, fallback), color: 'error', icon: 'i-lucide-circle-alert' }),
    info: (title: string, description?: string) => toast.add({ title, description, color: 'info', icon: 'i-lucide-info' }),
    warning: (title: string, description?: string) => toast.add({ title, description, color: 'warning', icon: 'i-lucide-triangle-alert', duration: 8000 }),
  }
}

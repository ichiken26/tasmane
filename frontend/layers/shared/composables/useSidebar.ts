import { useCookie } from '#imports'

// デスクトップのサイドバー開閉（Cookieに保存し、SSRでも同じ状態で描画）
export function useSidebar() {
  const collapsed = useCookie<boolean>('tasmane-sidebar-collapsed', { default: () => false, maxAge: 60 * 60 * 24 * 365, sameSite: 'lax' })
  return { collapsed, toggle: () => { collapsed.value = !collapsed.value } }
}

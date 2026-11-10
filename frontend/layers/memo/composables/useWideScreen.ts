import { onBeforeUnmount, onMounted, ref } from 'vue'

// lg 以上（左右分割表示できる幅）か
export function useWideScreen(query = '(min-width: 1024px)') {
  const wide = ref(false)
  let media: MediaQueryList | undefined
  const update = () => { wide.value = Boolean(media?.matches) }
  onMounted(() => {
    media = globalThis.matchMedia?.(query)
    update()
    media?.addEventListener('change', update)
  })
  onBeforeUnmount(() => media?.removeEventListener('change', update))
  return wide
}

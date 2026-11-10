import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useState } from '#imports'
import { clampSize, parseStoredSize, resizedSize } from './resize'

interface ResizableOptions {
  // 保存キー（ブラウザごとに記憶）
  storageKey: string
  initial: number
  min: number
  max: () => number
  axis: 'x' | 'y'
  // 1: 右/下へドラッグで拡大、-1: 左/上へドラッグで拡大
  direction: 1 | -1
}

// ドラッグ（とキーボード）でサイズを変えられる枠
export function useResizable(options: ResizableOptions) {
  const size = useState<number>(`resizable-${options.storageKey}`, () => options.initial)
  const dragging = ref(false)
  let start = 0
  let origin = 0

  const save = () => {
    try { globalThis.localStorage?.setItem(options.storageKey, String(size.value)) } catch { /* 保存不可でも動作は継続 */ }
  }
  const fit = () => { size.value = clampSize(size.value, options.min, options.max()) }

  // ドラッグ開始（iframe にポインタを奪われないよう、ドラッグ中は呼び出し側で覆いを出す）
  function onPointerDown(event: PointerEvent) {
    if (event.button !== 0) return
    event.preventDefault()
    dragging.value = true
    // ドラッグ中は iframe がポインタを奪わないようにする
    globalThis.document?.documentElement.classList.add(options.axis === 'x' ? 'memo-resizing-x' : 'memo-resizing-y')
    start = size.value
    origin = options.axis === 'x' ? event.clientX : event.clientY
    const target = event.currentTarget as HTMLElement | null
    target?.setPointerCapture?.(event.pointerId)
    const move = (moveEvent: PointerEvent) => {
      const delta = (options.axis === 'x' ? moveEvent.clientX : moveEvent.clientY) - origin
      size.value = resizedSize(start, delta, options.direction, options.min, options.max())
    }
    const end = () => {
      dragging.value = false
      globalThis.document?.documentElement.classList.remove('memo-resizing-x', 'memo-resizing-y')
      save()
      globalThis.removeEventListener('pointermove', move)
      globalThis.removeEventListener('pointerup', end)
      globalThis.removeEventListener('pointercancel', end)
    }
    globalThis.addEventListener('pointermove', move)
    globalThis.addEventListener('pointerup', end)
    globalThis.addEventListener('pointercancel', end)
  }

  // 矢印キーでの調整（Shiftで大きく）
  function onKeydown(event: KeyboardEvent) {
    const grow = options.axis === 'x' ? (options.direction === 1 ? 'ArrowRight' : 'ArrowLeft') : (options.direction === 1 ? 'ArrowDown' : 'ArrowUp')
    const shrink = options.axis === 'x' ? (options.direction === 1 ? 'ArrowLeft' : 'ArrowRight') : (options.direction === 1 ? 'ArrowUp' : 'ArrowDown')
    const step = event.shiftKey ? 80 : 20
    if (event.key !== grow && event.key !== shrink) return
    event.preventDefault()
    size.value = clampSize(size.value + (event.key === grow ? step : -step), options.min, options.max())
    save()
  }

  onMounted(() => {
    try { size.value = parseStoredSize(globalThis.localStorage?.getItem(options.storageKey) ?? null, size.value) } catch { /* 読めなければ既定値 */ }
    fit()
    globalThis.addEventListener('resize', fit)
  })
  onBeforeUnmount(() => globalThis.removeEventListener('resize', fit))

  return { size, dragging, onPointerDown, onKeydown, min: options.min, max: options.max }
}

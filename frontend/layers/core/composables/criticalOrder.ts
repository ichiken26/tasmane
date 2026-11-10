import { nextTick } from 'vue'
// 描画完了後のプリフェッチ予約
export function schedulePrefetch(afterRender: () => Promise<unknown>) { return nextTick().then(afterRender) }

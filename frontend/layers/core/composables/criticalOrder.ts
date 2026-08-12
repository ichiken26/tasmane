import { nextTick } from 'vue'
export function schedulePrefetch(afterRender: () => Promise<unknown>) { return nextTick().then(afterRender) }

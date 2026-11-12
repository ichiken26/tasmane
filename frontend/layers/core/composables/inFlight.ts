// 同じキーの実行中リクエストを共有（同時に呼ばれた同じGETは通信1回。完了後は忘れる）
export function shareInFlight<T>(pending: Map<string, Promise<unknown>>, key: string, load: () => Promise<T>): Promise<T> {
  const current = pending.get(key) as Promise<T> | undefined
  if (current) return current
  const next = load().finally(() => pending.delete(key))
  pending.set(key, next)
  return next
}

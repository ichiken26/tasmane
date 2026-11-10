interface ErrorLike { statusCode?: number; status?: number; response?: { status?: number }; data?: { code?: string } }

// エラーからHTTPステータスを抽出
export function apiErrorStatus(error: unknown): number | undefined {
  const value = error as ErrorLike | undefined
  return value?.statusCode ?? value?.status ?? value?.response?.status
}

// エラーコードの抽出
export const apiErrorCode = (error: unknown) => (error as ErrorLike | undefined)?.data?.code

// 利用者が次に取るべき行動が分かるメッセージ
export function apiErrorMessage(error: unknown, fallback: string): string {
  const status = apiErrorStatus(error)
  const code = apiErrorCode(error)
  if (status === 401) return 'ログインの有効期限が切れました。再度ログインしてください。'
  if (status === 403) return 'この操作を行う権限がありません。'
  if (status === 404) return '対象が見つかりません。削除された可能性があります。'
  if (code === 'IMPACT_TOKEN_INVALID') return '確認後に状態が変わりました。もう一度操作してください。'
  if (status === 409 && code === 'CONFLICT') return '他の操作で更新されています。最新の内容を読み込んでから再度お試しください。'
  if (status === 422) return `${fallback} 入力内容がルールに合っていません。`
  return fallback
}

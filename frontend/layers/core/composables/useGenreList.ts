import { useState } from '#app'
import type { ApiGenre } from './useCriticalData'

// アプリ全体で共有するジャンル一覧（Sidebar・Editor・一覧画面で同期）
export function useGenreList() {
  const genres = useState<ApiGenre[]>('genres', () => [])
  const loaded = useState<boolean>('genres-loaded', () => false)
  // 作成・更新結果の反映
  const upsert = (genre: ApiGenre) => {
    const index = genres.value.findIndex(item => item.id === genre.id)
    if (index >= 0) genres.value.splice(index, 1, genre)
    else genres.value.push(genre)
    genres.value.sort((a, b) => a.name.localeCompare(b.name, 'ja'))
  }
  // 削除結果の反映
  const remove = (id: string) => { genres.value = genres.value.filter(item => item.id !== id) }
  const nameOf = (id?: string | null) => genres.value.find(item => item.id === id)?.name
  return { genres, loaded, upsert, remove, nameOf }
}

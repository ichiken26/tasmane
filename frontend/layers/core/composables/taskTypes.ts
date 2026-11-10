// タスクタイプの作成・紐付け（タスク作成画面から利用）
// TaskType定義はユーザー共通・名前はcase-insensitive unique。既存の同名定義があれば再利用する。

export interface TaskTypeRecord { id: string; name: string }
export interface TaskTypeClient {
  allTaskTypes: () => Promise<TaskTypeRecord[]>
  createTaskType: (name: string) => Promise<TaskTypeRecord>
  attachTaskType: (genreId: string, taskTypeId: string) => Promise<void>
}

// 名前からタスクタイプを用意してジャンルへ紐付ける
export async function ensureGenreTaskType(client: TaskTypeClient, genreId: string, name: string): Promise<TaskTypeRecord> {
  const value = name.trim()
  if (!value) throw new Error('タスクタイプ名を入力してください')
  const key = value.toLocaleLowerCase()
  const existing = (await client.allTaskTypes()).find(type => type.name.toLocaleLowerCase() === key)
  const type = existing ?? await client.createTaskType(value)
  await client.attachTaskType(genreId, type.id)
  return type
}

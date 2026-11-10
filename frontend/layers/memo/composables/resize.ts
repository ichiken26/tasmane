// メモ表示枠のサイズ計算（ドラッグ・キーボード共通）

// 範囲内に丸める
export function clampSize(value: number, min: number, max: number): number {
  return Math.round(Math.min(Math.max(value, min), Math.max(min, max)))
}

// ドラッグ量からの新しいサイズ（direction: 下/右へ伸ばす=1、左/上へ伸ばす=-1）
export function resizedSize(start: number, delta: number, direction: 1 | -1, min: number, max: number): number {
  return clampSize(start + delta * direction, min, max)
}

// 保存済みサイズの読み出し（数値以外は既定値）
export function parseStoredSize(value: string | null, fallback: number): number {
  const parsed = Number(value)
  return value !== null && Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

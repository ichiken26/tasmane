import { useResizable } from './useResizable'

// 右側メモパネルの幅（左端ドラッグで変更。フォーム側に最低 420px を残す）
export function useMemoPanelWidth() {
  return useResizable({
    storageKey: 'tasmane-memo-panel-width',
    initial: 560,
    min: 320,
    max: () => Math.max(320, (globalThis.innerWidth || 1600) - 256 - 420 - 80),
    axis: 'x',
    direction: -1,
  })
}

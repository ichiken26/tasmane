import { describe, expect, it } from 'vitest'
import { clampSize, parseStoredSize, resizedSize } from '../layers/memo/composables/resize'

describe('memo frame resizing', () => {
  // 最小・最大の範囲に収める
  it('clamps sizes into the allowed range', () => {
    expect(clampSize(100, 240, 900)).toBe(240)
    expect(clampSize(1000, 240, 900)).toBe(900)
    expect(clampSize(500.6, 240, 900)).toBe(501)
  })
  // 下端ドラッグは下へ伸ばす、左端ドラッグは左へ動かすほど広げる
  it('applies the drag delta in the handle direction', () => {
    expect(resizedSize(440, 100, 1, 240, 900)).toBe(540)
    expect(resizedSize(480, -120, -1, 320, 1000)).toBe(600)
    expect(resizedSize(480, 900, -1, 320, 1000)).toBe(320)
  })
  // 保存値は数値のときだけ使う
  it('reads only numeric stored sizes', () => {
    expect(parseStoredSize('600', 440)).toBe(600)
    expect(parseStoredSize('abc', 440)).toBe(440)
    expect(parseStoredSize(null, 440)).toBe(440)
  })
})

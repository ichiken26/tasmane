import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../..')
const read = (path: string) => readFileSync(resolve(root, path))

describe('favicon', () => {
  // Nuxt 既定ではなくタスマネのロゴ（チェック付きカレンダー）を使う
  it('serves the Tasmane logo as the SVG favicon', () => {
    const svg = read('public/favicon.svg').toString('utf8')
    expect(svg).toContain('m13 25 5 5 9-10')
    expect(svg).toContain('M13 4v7M27 4v7')
  })

  // SVG 非対応環境と iOS のホーム画面向けの画像も用意する
  it('provides ico and apple-touch-icon fallbacks', () => {
    const ico = read('public/favicon.ico')
    expect([...ico.subarray(0, 4)]).toEqual([0, 0, 1, 0])
    const png = read('public/apple-touch-icon.png')
    expect(png.subarray(1, 4).toString('ascii')).toBe('PNG')
    expect(png.readUInt32BE(16)).toBe(180)
  })

  // head からアイコンを参照する
  it('links the icons from the app head', () => {
    const config = read('nuxt.config.ts').toString('utf8')
    expect(config).toContain("{ rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }")
    expect(config).toContain("{ rel: 'icon', href: '/favicon.ico', sizes: '32x32' }")
    expect(config).toContain("{ rel: 'apple-touch-icon', href: '/apple-touch-icon.png' }")
    expect(existsSync(resolve(root, 'public/favicon.svg'))).toBe(true)
  })
})

import { readFileSync } from 'node:fs'
import { compile } from '@tailwindcss/node'
import { beforeAll, describe, expect, it } from 'vitest'

// 全体の文字を各段階 +2px、余白（padding / margin / gap / space）を 0 以外は +2px 広げる
let css = ''
function rule(selector: string): string {
  const start = css.indexOf(`${selector} {`)
  expect(start, selector).toBeGreaterThanOrEqual(0)
  return css.slice(start, css.indexOf('}', start))
}
// ルール内で最後に宣言された値（後勝ち）
function last(selector: string, property: string): string {
  const values = [...rule(selector).matchAll(new RegExp(`(?:^|[;{\\s])${property}:\\s*([^;]+);`, 'g'))].map(match => match[1]!.trim())
  expect(values.length, `${selector} ${property}`).toBeGreaterThan(0)
  return values.at(-1)!
}

describe('UI scale', () => {
  beforeAll(async () => {
    const source = readFileSync('assets/css/main.css', 'utf8').replace(/@source[^;]+;/g, '')
    const compiler = await compile(source, { base: `${process.cwd()}/assets/css`, onDependency() {} })
    css = compiler.build(['text-xs', 'text-sm', 'text-base', 'text-lg', 'text-4xl', 'p-2', 'px-1.5', 'py-0.5', 'p-0', 'mt-4', 'mb-1', '-mr-1', 'gap-3', 'gap-x-2', 'space-y-2', 'mx-auto', 'h-16', 'size-5'])
  })

  it('raises every text size by 2px', () => {
    expect(css).toMatch(/--text-xs:\s*calc\(0\.75rem \+ 2px\)/)
    expect(css).toMatch(/--text-sm:\s*calc\(0\.875rem \+ 2px\)/)
    expect(css).toMatch(/--text-base:\s*calc\(1rem \+ 2px\)/)
    expect(css).toMatch(/--text-lg:\s*calc\(1\.125rem \+ 2px\)/)
    expect(css).toMatch(/--text-4xl:\s*calc\(2\.25rem \+ 2px\)/)
  })

  it('adds 2px to non-zero padding, margin and gaps', () => {
    expect(last('.p-2', 'padding')).toBe('calc(var(--spacing) * 2 + 2px)')
    expect(last('.px-1\\.5', 'padding-inline')).toBe('calc(var(--spacing) * 1.5 + 2px)')
    expect(last('.py-0\\.5', 'padding-block')).toBe('calc(var(--spacing) * 0.5 + 2px)')
    expect(last('.mt-4', 'margin-top')).toBe('calc(var(--spacing) * 4 + 2px)')
    expect(last('.mb-1', 'margin-bottom')).toBe('calc(var(--spacing) * 1 + 2px)')
    expect(last('.-mr-1', 'margin-right')).toBe('calc(calc(var(--spacing) * 1 + 2px) * -1)')
    expect(last('.gap-3', 'gap')).toBe('calc(var(--spacing) * 3 + 2px)')
    expect(last('.gap-x-2', 'column-gap')).toBe('calc(var(--spacing) * 2 + 2px)')
    expect(last(':where(.space-y-2 > :not(:last-child))', 'margin-block-end')).toContain('calc(var(--spacing) * 2 + 2px)')
  })

  it('keeps zero spacing, keywords and fixed sizes unchanged', () => {
    expect(last('.p-0', 'padding')).toBe('0px')
    expect(last('.mx-auto', 'margin-inline')).toBe('auto')
    // ヘッダー高さ（h-16）などはレイアウト計算に使うため据え置き
    expect(last('.h-16', 'height')).toBe('calc(var(--spacing) * 16)')
    expect(last('.size-5', 'width')).toBe('calc(var(--spacing) * 5)')
  })
})

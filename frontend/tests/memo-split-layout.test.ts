import { readFileSync } from 'node:fs'
import { parse } from '@vue/compiler-sfc'
import { describe, expect, it } from 'vitest'

type Node = { type: number; tag?: string; props?: { name: string }[]; children?: Node[] }

// スロットまでの経路（v-if/v-else の分岐にあれば、メモを開閉したときに中身が作り直される）
function slotPaths(node: Node, path: string[] = []): string[][] {
  if (node.type === 1 && node.tag === 'slot') return [path]
  const branch = node.type === 1 && node.props?.some(prop => prop.name === 'if' || prop.name === 'else' || prop.name === 'else-if')
  const next = node.type === 1 ? [...path, `${node.tag}${branch ? '(conditional)' : ''}`] : path
  return (node.children ?? []).flatMap(child => slotPaths(child, next))
}

describe('MemoSplitLayout', () => {
  // メモを右側に開閉しても、作成フォームの入力・未保存の紐付けが消えない
  it('keeps the page content mounted when the memo panel opens or closes', () => {
    const { descriptor } = parse(readFileSync('frontend/layers/memo/components/MemoSplitLayout.vue', 'utf8'))
    const paths = slotPaths(descriptor.template!.ast as unknown as Node)
    expect(paths).toHaveLength(1)
    expect(paths[0]!.some(step => step.endsWith('(conditional)'))).toBe(false)
  })
})

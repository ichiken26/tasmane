import { readFileSync, existsSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (path: string) => readFileSync(path, 'utf8')

describe('Nuxt application contract', () => {
  it('provides all required routes and layer boundaries', () => {
    for (const route of ['index', 'dashboard', 'tasks/new', 'tasks/[taskId]', 'genres', 'search'])
      expect(existsSync(`pages/${route}.vue`)).toBe(true)
    for (const layer of ['core', 'task', 'genre', 'dashboard', 'search', 'google', 'shared'])
      expect(existsSync(`frontend/layers/${layer}/nuxt.config.ts`)).toBe(true)
  })

  it('uses the warm design tokens, mobile breakpoint, focus and reduced-motion rules', () => {
    const css = read('assets/css/main.css')
    for (const token of ['#2b2622', '#383330', '#f7f5f0', '#3f3a36', '@media (max-width:767px)', ':focus-visible', 'prefers-reduced-motion'])
      expect(css).toContain(token)
  })

  it('keeps hierarchy and dependency semantics distinct and exposes warning dialogs', () => {
    expect(read('frontend/layers/task/components/HierarchyTree.vue')).toContain('Hierarchy Tree')
    expect(read('frontend/layers/task/components/DependencyGraph.vue')).toContain('Dependency Graph')
    expect(read('frontend/layers/task/components/DependencyGraph.vue')).toContain('multi-dependency')
    expect(read('frontend/layers/shared/components/ImpactModal.vue')).toContain('role="alertdialog"')
  })
})

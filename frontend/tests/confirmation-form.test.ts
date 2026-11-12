import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// 確認事項の入力: 「確認先」「期限（年月日）」→ その下に2〜3行の「確認する内容」
describe('confirmation form', () => {
  const source = readFileSync('frontend/layers/task/components/ConfirmationPanel.vue', 'utf8')
  const form = source.slice(source.indexOf('<form'), source.indexOf('</form>'))

  it('orders target, deadline (date only), then a multi-line content field', () => {
    const target = form.indexOf('label="確認先"')
    const due = form.indexOf('label="期限"')
    const content = form.indexOf('label="確認する内容"')
    expect(target).toBeGreaterThanOrEqual(0)
    expect(due).toBeGreaterThan(target)
    expect(content).toBeGreaterThan(due)
    expect(form).toMatch(/<UTextarea[^>]*v-model="form\.content"[^>]*:rows="3"/)
    expect(form).not.toMatch(/<UInput[^>]*v-model="form\.content"/)
    const dateField = form.match(/<DateField[^>]*\/>/)?.[0] ?? ''
    expect(dateField).toContain('v-model="form.dueAt"')
    expect(dateField).not.toMatch(/with-time/)
  })

  it('starts with empty inputs', () => {
    expect(source).toMatch(/reactive\(\{ content: '', targetName: '', dueAt: '' \}\)/)
  })
})

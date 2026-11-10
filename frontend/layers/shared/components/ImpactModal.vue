<script setup lang="ts">
import type { ModalProps } from '@nuxt/ui'
// DialogContentへrole=alertdialogを透過させる（型定義はprops外のattrsを持たないため変換）
const alertContent = { role: 'alertdialog' } as unknown as ModalProps['content']
import type { ImpactPlan } from '../../core/composables/useCriticalData'
import { impactReasonLabel } from '../../core/composables/impact'

// Domainが返したImpact Planを表示し、承認時のみ再送する（Frontendで影響範囲を推測しない）
const props = withDefaults(defineProps<{ open: boolean; title: string; impacts?: string[]; plan?: ImpactPlan; loading?: boolean; confirmLabel?: string; danger?: boolean }>(), { impacts: () => [], plan: undefined, confirmLabel: '承認して続行' })
const emit = defineEmits<{ confirm: []; cancel: [] }>()
const typeLabel = (type: string) => ({ task: 'タスク', dependency: '依存関係', genre: 'ジャンル', confirmation: '確認事項' } as Record<string, string>)[type.toLowerCase()] || type
// 閉じる操作はキャンセル扱い
function onOpenChange(value: boolean) { if (!value && !props.loading) emit('cancel') }
</script>

<template>
  <!-- 影響確認モーダル -->
  <UModal :open="open" :title="title" description="変更による影響範囲を確認してください。キャンセルした場合は何も変更されません。" :content="alertContent" :close="false" @update:open="onOpenChange">
    <template #body>
      <div class="space-y-4">
        <UAlert v-for="warning in plan?.warnings || []" :key="warning" color="warning" variant="subtle" icon="i-lucide-triangle-alert" :title="warning" />
        <ul v-if="plan?.affectedEntities.length" class="max-h-72 divide-y divide-(--ui-border) overflow-y-auto rounded-lg border border-(--ui-border)">
          <li v-for="entity in plan.affectedEntities" :key="`${entity.type}:${entity.id}`" class="flex items-start gap-3 px-3 py-2.5 text-sm">
            <UBadge :label="typeLabel(entity.type)" color="neutral" variant="subtle" size="sm" class="mt-0.5 shrink-0" />
            <div class="min-w-0">
              <p class="truncate font-medium text-(--ui-text-highlighted)">{{ entity.title || entity.id }}</p>
              <p v-if="entity.reason" class="text-(--ui-text-muted)">{{ impactReasonLabel(entity.reason) }}</p>
            </div>
          </li>
        </ul>
        <ul v-else-if="!plan && impacts.length" class="list-disc space-y-1 pl-5 text-sm">
          <li v-for="impact in impacts" :key="impact">{{ impact }}</li>
        </ul>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="outline" :disabled="loading" @click="emit('cancel')">キャンセル</UButton>
        <UButton :color="danger ? 'error' : 'primary'" :loading="loading" @click="emit('confirm')">{{ confirmLabel }}</UButton>
      </div>
    </template>
  </UModal>
</template>

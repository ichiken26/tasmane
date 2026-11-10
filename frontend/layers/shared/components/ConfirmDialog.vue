<script setup lang="ts">
import type { ModalProps } from '@nuxt/ui'
// DialogContentへrole=alertdialogを透過させる（型定義はprops外のattrsを持たないため変換）
const alertContent = { role: 'alertdialog' } as unknown as ModalProps['content']
// 破壊的操作の1段階目の確認ダイアログ
const props = withDefaults(defineProps<{ title: string; description?: string; confirmLabel?: string; loading?: boolean; danger?: boolean }>(), { description: undefined, confirmLabel: 'OK', danger: true })
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ confirm: [] }>()
function onOpenChange(value: boolean) { if (!props.loading) open.value = value }
</script>

<template>
  <UModal :open="open" :title="title" :description="description" :content="alertContent" @update:open="onOpenChange">
    <template v-if="$slots.default" #body><slot /></template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="outline" :disabled="loading" @click="open = false">キャンセル</UButton>
        <UButton :color="danger ? 'error' : 'primary'" :loading="loading" @click="emit('confirm')">{{ confirmLabel }}</UButton>
      </div>
    </template>
  </UModal>
</template>

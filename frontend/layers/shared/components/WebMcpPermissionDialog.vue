<script setup lang="ts">
import { computed } from 'vue'
import { useWebMcpPermission } from '../../core/composables/useWebMcpPermission'

// ブラウザ内 Agent（WebMCP）からの書き込み要求の許可ダイアログ
const { pending, answer } = useWebMcpPermission()
const open = computed({ get: () => Boolean(pending.value), set: (value: boolean) => { if (!value) answer(false) } })
</script>

<template>
  <UModal v-model:open="open" title="AIアシスタントからの操作要求" description="ブラウザ内のAIアシスタントがタスマネのデータを変更しようとしています。内容を確認してください。" :dismissible="false">
    <template #body>
      <p class="whitespace-pre-wrap rounded-lg bg-(--ui-bg-muted) px-3 py-2 text-sm text-(--ui-text-highlighted)">{{ pending?.summary }}</p>
      <p class="mt-3 text-xs text-(--ui-text-muted)">許可しない場合、操作は行われません。削除操作はAIアシスタントからは実行できません。</p>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="outline" @click="answer(false)">許可しない</UButton>
        <UButton icon="i-lucide-check" @click="answer(true)">許可する</UButton>
      </div>
    </template>
  </UModal>
</template>

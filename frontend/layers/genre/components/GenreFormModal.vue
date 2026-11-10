<script setup lang="ts">
import { ref, watch } from 'vue'
import type { ApiGenre } from '../../core/composables/useCriticalData'
import { useTasmaneApi } from '../../core/composables/useCriticalData'
import { useGenreList } from '../../core/composables/useGenreList'
import { apiErrorMessage, apiErrorStatus } from '../../core/composables/apiError'
import { useNotify } from '../../shared/composables/useNotify'

// ジャンルの作成・名前変更
const props = withDefaults(defineProps<{ genre?: ApiGenre; initialName?: string }>(), { genre: undefined, initialName: '' })
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [genre: ApiGenre] }>()
const api = useTasmaneApi()
const notify = useNotify()
const { upsert } = useGenreList()
const name = ref('')
const busy = ref(false)
const errorMessage = ref('')

watch(open, (value) => { if (value) { name.value = props.genre?.name ?? props.initialName; errorMessage.value = '' } })

// 保存
async function submit() {
  const value = name.value.trim()
  if (!value || busy.value) return
  busy.value = true
  errorMessage.value = ''
  try {
    const saved = props.genre ? await api.renameGenre(props.genre.id, value) : await api.createGenre(value)
    upsert(saved)
    notify.success(props.genre ? 'ジャンル名を変更しました' : `ジャンル「${saved.name}」を作成しました`)
    emit('saved', saved)
    open.value = false
  } catch (error) {
    errorMessage.value = apiErrorStatus(error) === 409 ? '同じ名前のジャンルが既にあります（大文字小文字は区別しません）。' : apiErrorMessage(error, '保存できませんでした。')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UModal v-model:open="open" :title="genre ? 'ジャンル名を変更' : 'ジャンルを追加'">
    <template #body>
      <form id="genre-form" class="space-y-4" @submit.prevent="submit">
        <UFormField label="ジャンル名" required :error="errorMessage || undefined">
          <UInput v-model="name" autofocus maxlength="120" placeholder="例: 開発、家事、学習" class="w-full" />
        </UFormField>
      </form>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="outline" @click="open = false">キャンセル</UButton>
        <UButton type="submit" form="genre-form" :loading="busy" :disabled="!name.trim()">{{ genre ? '変更を保存' : '作成' }}</UButton>
      </div>
    </template>
  </UModal>
</template>

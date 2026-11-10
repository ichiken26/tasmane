<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from '#imports'
import { apiErrorMessage, apiErrorStatus } from '../../core/composables/apiError'

// 取得失敗の表示（未ログインならログイン導線、それ以外は再試行）
const props = defineProps<{ error: unknown; fallback: string }>()
const emit = defineEmits<{ retry: [] }>()
const route = useRoute()
const unauthorized = computed(() => apiErrorStatus(props.error) === 401)
</script>

<template>
  <UAlert
    role="alert"
    :color="unauthorized ? 'info' : 'error'"
    variant="subtle"
    :icon="unauthorized ? 'i-lucide-log-in' : 'i-lucide-circle-alert'"
    :title="unauthorized ? 'ログインが必要です' : apiErrorMessage(error, fallback)"
    :actions="unauthorized
      ? [{ label: 'Googleでログイン', href: `/auth/login?returnTo=${encodeURIComponent(route.fullPath)}`, external: true, color: 'info' }]
      : [{ label: '再試行', icon: 'i-lucide-refresh-cw', color: 'error', variant: 'outline', onClick: () => emit('retry') }]"
  />
</template>

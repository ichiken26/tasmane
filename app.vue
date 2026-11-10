<script setup lang="ts">
import { ja } from '@nuxt/ui/locale'
</script>

<template>
  <UApp :locale="ja" :toaster="{ position: 'bottom-right' }" :tooltip="{ delayDuration: 300 }">
    <NuxtLoadingIndicator color="var(--ui-primary)" />
    <NuxtLayout><NuxtPage /></NuxtLayout>
    <ClientOnly><WebMcpPermissionDialog /></ClientOnly>
  </UApp>
</template>

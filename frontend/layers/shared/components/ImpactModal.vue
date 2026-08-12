<script setup lang="ts">
defineProps<{ open: boolean; title: string; impacts: string[] }>()
defineEmits<{ confirm: []; cancel: [] }>()
</script>
<template><div v-if="open" class="modal-backdrop"><section role="alertdialog" aria-modal="true" aria-labelledby="impact-title" class="modal"><h2 id="impact-title">{{ title }}</h2><p>変更による影響範囲を確認してください。</p><ul><li v-for="impact in impacts" :key="impact">{{ impact }}</li></ul><div class="actions"><button type="button" @click="$emit('cancel')">キャンセル</button><button class="primary" type="button" @click="$emit('confirm')">承認して続行</button></div></section></div></template>

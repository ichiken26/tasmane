<template>
  <!-- 期限切れ（🔥）表示: 自身または未完了子孫の期限切れ -->
  <UTooltip text="期限切れ（子孫を含む）">
    <span role="img" aria-label="期限切れ" class="inline-flex shrink-0 items-center text-(--ui-error)"><UIcon name="i-lucide-flame" class="size-4" /></span>
  </UTooltip>
</template>

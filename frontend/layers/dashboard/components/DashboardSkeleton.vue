<template>
  <!-- Dashboard読込中のプレースホルダ -->
  <div class="space-y-6" role="status" aria-label="Dashboardを読み込み中">
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4"><USkeleton v-for="index in 4" :key="index" class="h-28 rounded-xl" /></div>
    <USkeleton class="h-24 rounded-xl" />
    <div class="grid gap-6 xl:grid-cols-[1fr_380px]"><USkeleton class="h-96 rounded-xl" /><USkeleton class="h-96 rounded-xl" /></div>
  </div>
</template>

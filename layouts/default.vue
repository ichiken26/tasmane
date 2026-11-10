<template>
  <div class="min-h-dvh bg-(--ui-bg)">
    <a class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-1/2 focus:z-50 focus:-translate-x-1/2 focus:rounded-md focus:bg-(--ui-bg-inverted) focus:px-4 focus:py-2 focus:text-(--ui-text-inverted)" href="#main-content">本文へ移動</a>
    <AppHeader />
    <div class="mx-auto flex min-h-[calc(100dvh-4rem-2.25rem)] w-full max-w-[1600px]">
      <!-- デスクトップ用サイドバー -->
      <aside class="hidden w-64 shrink-0 border-r border-(--ui-border) lg:block">
        <div class="sticky top-16 max-h-[calc(100dvh-4rem-2.25rem)] overflow-y-auto"><AppSidebar /></div>
      </aside>
      <main id="main-content" tabindex="-1" class="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-8 focus:outline-none">
        <slot />
      </main>
    </div>
    <AppFooter />
  </div>
</template>

<script setup lang="ts">
const open = ref(false)
</script>
<template>
  <header class="header">
    <NuxtLink class="logo" to="/" aria-label="タスマネ トップ"><svg aria-hidden="true" viewBox="0 0 40 40"><path d="M7 12h26v20H7zM12 7v10m16-10v10M13 23h14" /></svg><span>タスマネ</span></NuxtLink>
    <a class="skip-link" href="#main-content">本文へ移動</a>
    <button class="menu-button" type="button" :aria-expanded="open" aria-controls="global-nav" @click="open = !open">メニュー</button>
    <nav id="global-nav" :class="{ open }" aria-label="メインナビゲーション">
      <NuxtLink to="/">Top</NuxtLink><NuxtLink to="/dashboard">Dashboard</NuxtLink><NuxtLink to="/tasks/new">新規タスク</NuxtLink><NuxtLink to="/genres">ジャンル一覧</NuxtLink><NuxtLink to="/search">検索</NuxtLink>
      <details><summary aria-label="Googleプロフィールメニュー">YK</summary><NuxtLink to="/settings">ユーザー設定</NuxtLink><button type="button">ログアウト</button></details>
    </nav>
  </header>
</template>

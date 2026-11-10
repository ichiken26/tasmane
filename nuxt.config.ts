export default defineNuxtConfig({
  compatibilityDate: '2025-05-15',
  devtools: { enabled: false },
  // ログイン必須の業務アプリのためSPAで描画する。
  // 期限の相対表示（今日/明日/超過）はユーザーのローカル時刻に依存し、UTCで動くWorkerでSSRすると表示がずれるため。
  ssr: false,
  // Layerの読込
  extends: [
    './frontend/layers/core', './frontend/layers/shared', './frontend/layers/task',
    './frontend/layers/genre', './frontend/layers/dashboard', './frontend/layers/search', './frontend/layers/google', './frontend/layers/memo',
  ],
  css: ['~/assets/css/main.css'],
  modules: ['@nuxt/ui', '@nuxtjs/google-fonts'],
  // Nuxt UI設定（フォントは@nuxtjs/google-fontsで読込）
  ui: { fonts: false, theme: { colors: ['primary', 'secondary', 'success', 'info', 'warning', 'error', 'neutral'] } },
  // カラーモード（OS設定に追従し、Headerから切替）
  colorMode: { preference: 'system', fallback: 'dark', storageKey: 'tasmane-color-mode' },
  // アイコンはクライアントへ同梱し外部取得を行わない（CSP connect-src 'self'）
  icon: {
    provider: 'server',
    fallbackToApi: false,
    clientBundle: {
      scan: true,
      // .ts（labels等）から動的に参照するアイコンとNuxt UI既定アイコンを明示的に同梱
      icons: ['lucide:arrow-left', 'lucide:arrow-left-to-line', 'lucide:arrow-right', 'lucide:arrow-up-down', 'lucide:arrow-up-right', 'lucide:badge-check', 'lucide:calendar', 'lucide:calendar-check', 'lucide:calendar-clock', 'lucide:calendar-range', 'lucide:chart-pie', 'lucide:check', 'lucide:chevron-down', 'lucide:chevron-left', 'lucide:chevron-right', 'lucide:chevron-up', 'lucide:chevrons-down', 'lucide:chevrons-down-up', 'lucide:chevrons-left', 'lucide:chevrons-right', 'lucide:chevrons-up', 'lucide:chevrons-up-down', 'lucide:circle', 'lucide:circle-alert', 'lucide:circle-check', 'lucide:circle-dot', 'lucide:circle-help', 'lucide:cloud-off', 'lucide:cloud-upload', 'lucide:coffee', 'lucide:corner-down-right', 'lucide:download', 'lucide:ellipsis', 'lucide:ellipsis-vertical', 'lucide:equal', 'lucide:external-link', 'lucide:flame', 'lucide:folder', 'lucide:folder-plus', 'lucide:folders', 'lucide:gantt-chart', 'lucide:git-branch', 'lucide:git-compare', 'lucide:git-merge', 'lucide:house', 'lucide:import', 'lucide:inbox', 'lucide:info', 'lucide:layout-dashboard', 'lucide:list-plus', 'lucide:list-restart', 'lucide:list-tree', 'lucide:list-x', 'lucide:loader-circle', 'lucide:log-in', 'lucide:log-out', 'lucide:menu', 'lucide:message-circle-more', 'lucide:message-circle-question', 'lucide:minus', 'lucide:monitor', 'lucide:moon', 'lucide:octagon-alert', 'lucide:party-popper', 'lucide:pencil', 'lucide:play', 'lucide:plus', 'lucide:refresh-cw', 'lucide:rotate-ccw', 'lucide:rotate-cw', 'lucide:search', 'lucide:search-x', 'lucide:settings', 'lucide:signal-high', 'lucide:sparkles', 'lucide:square-check-big', 'lucide:sticky-note', 'lucide:file-plus', 'lucide:link', 'lucide:sun', 'lucide:tag', 'lucide:tags', 'lucide:trash', 'lucide:trash-2', 'lucide:triangle-alert', 'lucide:undo-2', 'lucide:unlink', 'lucide:upload', 'lucide:user', 'lucide:workflow', 'lucide:x'],
    },
    serverBundle: { collections: ['lucide'] },
  },
  // Google Fonts設定
  googleFonts: { families: { Inter: [400, 500], 'DM Mono': [400], 'Instrument Serif': [400] }, display: 'swap' },
  typescript: { typeCheck: true, strict: true },
  // 実行時の機密と公開設定
  runtimeConfig: {
    googleClientId: process.env.NUXT_GOOGLE_CLIENT_ID,
    googleClientSecret: process.env.NUXT_GOOGLE_CLIENT_SECRET,
    googleRedirectUri: process.env.NUXT_GOOGLE_REDIRECT_URI,
    sessionSecret: process.env.NUXT_SESSION_SECRET,
    internalBindingSecret: process.env.NUXT_INTERNAL_BINDING_SECRET,
    apiInternalUrl: process.env.NUXT_API_INTERNAL_URL || 'http://127.0.0.1:8080',
    // ログイン許可メールアドレス（カンマ区切り。自宅サーバー運用では設定を推奨）
    allowedEmails: process.env.NUXT_ALLOWED_EMAILS || '',
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api/v1',
      // 連携するメモアプリ（埋め込み表示・ログイン引き継ぎ先）
      memoAppOrigin: process.env.NUXT_PUBLIC_MEMO_APP_ORIGIN || 'https://memo.kokage-studio.com',
    },
  },
  // Cloudflare向けNitro
  nitro: { preset: 'cloudflare_module' },
  // アプリのhead
  app: { head: { htmlAttrs: { lang: 'ja' }, meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } },
})

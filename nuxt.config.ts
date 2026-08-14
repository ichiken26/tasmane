export default defineNuxtConfig({
  compatibilityDate: '2025-05-15',
  devtools: { enabled: false },
  extends: [
    './frontend/layers/core', './frontend/layers/shared', './frontend/layers/task',
    './frontend/layers/genre', './frontend/layers/dashboard', './frontend/layers/search', './frontend/layers/google',
  ],
  css: ['~/assets/css/main.css'],
  modules: ['@nuxtjs/google-fonts'],
  googleFonts: { families: { Inter: [400, 500], 'DM Mono': [400], 'Instrument Serif': [400] }, display: 'swap' },
  typescript: { typeCheck: true, strict: true },
  runtimeConfig: {
    googleClientId: process.env.NUXT_GOOGLE_CLIENT_ID,
    googleClientSecret: process.env.NUXT_GOOGLE_CLIENT_SECRET,
    googleRedirectUri: process.env.NUXT_GOOGLE_REDIRECT_URI,
    sessionSecret: process.env.NUXT_SESSION_SECRET,
    internalBindingSecret: process.env.NUXT_INTERNAL_BINDING_SECRET,
    apiInternalUrl: process.env.NUXT_API_INTERNAL_URL || 'http://127.0.0.1:8080',
    public: { apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api/v1' },
  },
  nitro: { preset: 'cloudflare_module' },
  app: { head: { htmlAttrs: { lang: 'ja' }, meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } },
})

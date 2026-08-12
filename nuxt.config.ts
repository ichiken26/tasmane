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
  runtimeConfig: { public: { apiBase: process.env.NUXT_PUBLIC_API_BASE || '/v1' } },
  app: { head: { htmlAttrs: { lang: 'ja' }, meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } },
})

// Nuxt UIテーマ（暖色ニュートラル + ポップなオレンジ）
export default defineAppConfig({
  ui: {
    colors: { primary: 'orange', secondary: 'amber', success: 'emerald', info: 'sky', warning: 'amber', error: 'rose', neutral: 'stone' },
    button: { defaultVariants: { size: 'md' } },
    card: { slots: { root: 'rounded-xl' } },
  },
})

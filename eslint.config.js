import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'

export default [
  { ignores: ['.nuxt/**', '.output/**', 'node_modules/**'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: { parser: vueParser, parserOptions: { parser: tseslint.parser, extraFileExtensions: ['.vue'], sourceType: 'module' } },
  },
  {
    languageOptions: { globals: { ref: 'readonly', reactive: 'readonly', watch: 'readonly', useRoute: 'readonly', useRouter: 'readonly', useSeoMeta: 'readonly', onScopeDispose: 'readonly', defineNuxtConfig: 'readonly' } },
    rules: { 'vue/multi-word-component-names': 'off', 'vue/max-attributes-per-line': 'off', 'vue/singleline-html-element-content-newline': 'off', 'vue/html-self-closing': 'off', 'vue/html-closing-bracket-spacing': 'off' },
  },
]

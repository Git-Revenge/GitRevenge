import js from '@eslint/js'
import svelte from 'eslint-plugin-svelte'
import globals from 'globals'
import ts from 'typescript-eslint'
import svelteConfig from './svelte.config.js'

export default ts.config(
  { ignores: ['dist/', 'wailsjs/'] },
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs.recommended,
  {
    // __SVELTE_VERSION__ is set at build time: see vite.config.ts.
    languageOptions: { globals: { ...globals.browser, __SVELTE_VERSION__: 'readonly' } },
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts'],
    languageOptions: {
      parserOptions: {
        projectService: true,
        extraFileExtensions: ['.svelte'],
        parser: ts.parser,
        svelteConfig,
      },
    },
  },
  {
    // Nearly everything the app does is a call to the backend, which is a promise. One that's
    // dropped loses its failure, and one handed to something that expects a plain function
    // (an event handler, a callback) isn't waited for. These rules need types, so they run
    // where the project's tsconfig reaches. A promise left to itself on purpose says `void`.
    files: ['src/**/*.ts', 'src/**/*.svelte', 'e2e/**/*.ts'],
    languageOptions: { parserOptions: { projectService: true, extraFileExtensions: ['.svelte'] } },
    rules: {
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/no-misused-promises': 'error',
    },
  },
)

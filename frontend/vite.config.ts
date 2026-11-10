import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import {VERSION as svelteVersion} from 'svelte/compiler'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  define: {
    // The Svelte version this build was compiled with, for the About dialog's credits.
    __SVELTE_VERSION__: JSON.stringify(svelteVersion),
  },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          // The Svelte runtime gets a chunk of its own. Left alone, the bundler shares it out
          // between the main chunk and a side chunk differently from build to build, and the
          // main chunk's budget (scripts/bundle-budget.mjs) would measure that, not the app.
          groups: [{ name: 'svelte', test: /node_modules[/]svelte[/]/ }],
        },
      },
    },
  },
  server: {
    ws: {
      // The Wails desktop window lives on wails.localhost, which the dev
      // server's host guard rejects. WebView2 can't intercept WebSockets, so
      // name a host the guard allows instead of inheriting the page's.
      host: 'localhost',
    },
  },
})

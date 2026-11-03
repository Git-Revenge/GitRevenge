import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  server: {
    ws: {
      // The Wails desktop window lives on wails.localhost, which the dev
      // server's host guard rejects. WebView2 can't intercept WebSockets, so
      // name a host the guard allows instead of inheriting the page's.
      host: 'localhost',
    },
  },
})

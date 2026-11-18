/**
 * The Content-Security-Policy of the app's own page. vite.config.ts writes it into the page as
 * a <meta> tag, first thing in <head>.
 *
 * It's the second line of defence. Nothing from a repository is ever put into the page as
 * HTML or script (see SECURITY.md), and if a renderer or a library ever gets that wrong, this
 * is what keeps the mistake from running: a script in the page can call every Go method the
 * app binds.
 *
 * - Scripts come from the app's own files only: no inline script, no eval, no script from a
 *   data: or blob: address. 'wasm-unsafe-eval' lets a library compile WebAssembly it ships
 *   and allows nothing else.
 * - Nothing is fetched from the network: there is no http: or https: source anywhere.
 * - Pictures, sound and video come from the app's files, or from data: and blob: addresses
 *   made from repository content the Go side handed over.
 * - Styles may be inline, because Svelte, CodeMirror and Mermaid all set them.
 *
 * The app's own origin differs by platform: http://wails.localhost on Windows, wails://wails
 * on macOS and Linux. Both are named beside 'self', for a web view that doesn't count a
 * custom scheme as 'self'.
 */
const OWN = "'self' wails: http://wails.localhost"

export function contentPolicy(dev = false): string {
  // In development Vite's hot reload talks to its own server over a WebSocket, which under
  // `wails dev` is another port than the page's.
  const reload = dev ? ' ws://localhost:* ws://127.0.0.1:* http://localhost:* http://127.0.0.1:*' : ''
  return [
    "default-src 'none'",
    `script-src ${OWN} 'wasm-unsafe-eval'${dev ? ' http://localhost:* http://127.0.0.1:*' : ''}`,
    `style-src ${OWN} 'unsafe-inline'`,
    `img-src ${OWN} data: blob:`,
    `media-src ${OWN} data: blob:`,
    `font-src ${OWN} data: blob:`,
    `connect-src ${OWN}${reload}`,
    `worker-src ${OWN}`,
    "frame-src 'self'",
    "base-uri 'none'",
    "form-action 'none'",
  ].join('; ')
}

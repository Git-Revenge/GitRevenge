import { defineConfig } from '@playwright/test'

// Real components in a real browser against a stubbed Go backend (see e2e/harness.ts), so these
// run without wails dev. On Windows they use Edge, which every Windows machine has; anywhere
// else, and in CI, Playwright's own Chromium (`npx playwright install chromium`). Set
// PLAYWRIGHT_CHANNEL to use something else.
const channel = process.env.PLAYWRIGHT_CHANNEL ?? (process.env.CI || process.platform !== 'win32' ? undefined : 'msedge')

export default defineConfig({
  testDir: 'e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:5183',
    viewport: { width: 1280, height: 800 },
    trace: 'retain-on-failure',
  },
  // Linux runs the app in WebKit, so it's tested too. Playwright's WebKit isn't WebKitGTK, so this
  // checks compatibility, not rendering or speed.
  projects: [
    // Canvases are drawn in software, as headless Chromium on Linux draws them in CI. Edge on
    // Windows draws them on the GPU, which rounds anti-aliased edges by a level or two
    // differently from one run to the next, so tests that compare a graph's pixels failed there
    // for no reason in the code.
    { name: 'chromium', use: { browserName: 'chromium', channel, launchOptions: { args: ['--disable-accelerated-2d-canvas'] } } },
    { name: 'webkit', use: { browserName: 'webkit' } },
  ],
  webServer: {
    command: 'npx vite --port 5183 --strictPort',
    url: 'http://localhost:5183/e2e/harness.html',
    reuseExistingServer: !process.env.CI,
  },
})

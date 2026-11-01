import { defineConfig } from '@playwright/test'

// Real components in a real browser against a stubbed Go backend (see e2e/harness.ts), so these
// run without wails dev. Locally they use Edge, which every Windows machine has; CI installs
// Chromium. Set PLAYWRIGHT_CHANNEL to use something else.
const channel = process.env.PLAYWRIGHT_CHANNEL ?? (process.env.CI ? undefined : 'msedge')

export default defineConfig({
  testDir: 'e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:5183',
    channel,
    viewport: { width: 1280, height: 800 },
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npx vite --port 5183 --strictPort',
    url: 'http://localhost:5183/e2e/harness.html',
    reuseExistingServer: !process.env.CI,
  },
})

import { expect, test } from '@playwright/test'
import { calls, open } from './support'

// Insights in a browser (#886). Its store is tested without one, where Svelte's $state holds
// plain values. In a browser it holds proxies, and a check that compared a scope with the one
// it had just stored never passed: Insights opened and computed nothing.

const win = {
  target: 'origin/main',
  targetSha: '74cec43b701e879058ce439c48bc400b600b0d2f',
  asOf: 1793026800,
  since: 1791849600,
  until: 1793026800,
  range: '',
  kinds: {},
  botAuthors: [],
  references: [],
}
const scenario = {
  responses: {
    Groups: [{ name: 'Harbour team', paths: ['/r', '/s'] }],
    InsightsShipped: { window: win, items: [] },
    InsightsBranches: { window: win, branches: [] },
    InsightsFetchedAt: 0,
    InsightsThresholds: { stale: 7, longLived: 14, behind: 50, size: 500 },
    InsightsTags: [],
    InsightsReleaseBranches: [],
  },
}
const computedFor = async (page: import('@playwright/test').Page) => [...new Set((await calls(page, 'InsightsShipped')).map((c) => c[0] as string))].sort()

test.describe('Insights, opened', () => {
  test('computes the repository it opens on, and shows the results', async ({ page }) => {
    await open(page, 'insights', scenario)
    const main = page.getByRole('region', { name: 'Insights' })
    await expect(main.getByRole('heading', { level: 1 })).toHaveText('tidewater')
    await expect.poll(() => computedFor(page)).toEqual(['/r'])
    // Results are on the page: the tiles, and what's only offered once there's something to copy.
    await expect(main.getByText('items on main')).toBeVisible()
    await expect(main.getByRole('button', { name: 'Copy summary' })).toBeVisible()
    await expect(main.getByText('Computing…')).toHaveCount(0)
  })

  test('computes a group chosen as its scope, every repository of it', async ({ page }) => {
    await open(page, 'insights', scenario)
    await page.locator('.rail select').selectOption('group:Harbour team')
    const main = page.getByRole('region', { name: 'Insights' })
    await expect(main.getByRole('heading', { level: 1 })).toHaveText(/Harbour team/)
    await expect.poll(() => computedFor(page)).toEqual(['/r', '/s'])
    await expect(main.getByRole('button', { name: 'Copy summary' })).toBeVisible()
  })

  test('computes the repository again when the scope goes back to it', async ({ page }) => {
    await open(page, 'insights', scenario)
    await expect.poll(() => computedFor(page)).toEqual(['/r'])
    await page.locator('.rail select').selectOption('group:Harbour team')
    await expect.poll(() => computedFor(page)).toEqual(['/r', '/s'])
    const before = (await calls(page, 'InsightsShipped')).length
    await page.locator('.rail select').selectOption('repo:/r')
    await expect.poll(async () => (await calls(page, 'InsightsShipped')).length).toBeGreaterThan(before)
    await expect(page.getByRole('region', { name: 'Insights' }).getByRole('heading', { level: 1 })).toHaveText('tidewater')
  })
})

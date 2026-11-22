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

test('a branch with every flag keeps room for its name', async ({ page }) => {
  const name = 'origin/fix/fix-alerts-with-no-station-name-at-craig'
  const branch = {
    name,
    sha: 'a'.repeat(40),
    local: false,
    author: 'David Ramirez',
    lastCommit: 1791000000,
    firstCommit: 1789000000,
    behind: 80,
    added: 900,
    removed: 20,
    paths: [],
    staleDays: 20,
    ageDays: 30,
    flags: ['stale', 'longLived', 'drifting', 'large'],
  }
  await open(page, 'insights', { responses: { ...scenario.responses, InsightsBranches: { window: win, branches: [branch] } } })
  const shown = page.getByRole('region', { name: 'Insights' }).locator('code.branch', { hasText: name })
  await expect(shown).toBeVisible()
  const row = shown.locator('xpath=ancestor::div[contains(@class,"row")][1]')
  const [nameBox, rowBox] = [await shown.boundingBox(), await row.boundingBox()]
  // At least half the row, and no more than three lines of it.
  expect(nameBox!.width).toBeGreaterThanOrEqual(rowBox!.width * 0.45)
  const lineHeight = await shown.evaluate((el) => parseFloat(getComputedStyle(el).lineHeight) || 20)
  expect(nameBox!.height).toBeLessThanOrEqual(lineHeight * 3 + 1)
})


import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// The status bar's line about reading trailers (#826): every commit's trailers are read
// once, in the background. The line says so while that's taking a moment, for the
// repository on screen, and nothing otherwise.

const status = (over: object = {}) => ({ path: '/r', reading: false, read: 0, total: 0, done: false, commits: 0, ...over })
const emit = (page: Page, data: object) => page.evaluate((d) => (window as unknown as { __emit: (n: string, d: object) => void }).__emit('trailers:progress', d), data)
const line = (page: Page) => page.getByRole('group', { name: 'Reading trailers' })

test.describe('reading trailers, in the status bar', () => {
  test('says how far a long read has got, then says nothing', async ({ page }) => {
    await open(page, 'trailerProgress', { responses: { TrailerStatus: status({ done: true }) } })
    await expect(line(page)).toHaveCount(0)
    // It asked once, in case a read began before it was listening.
    expect(await calls(page, 'TrailerStatus')).toEqual([['/r']])

    await emit(page, status({ reading: true, read: 20000 }))
    await expect(line(page)).toBeVisible()
    await expect(line(page)).toContainText('Reading trailers')
    // The total is counted beside the read, and may not be in yet.
    await expect(line(page).locator('.how')).toHaveText(`${(20000).toLocaleString()} commits`)

    await emit(page, status({ reading: true, read: 412000, total: 1503134 }))
    await expect(line(page).locator('.how')).toHaveText(`${(412000).toLocaleString()} of ${(1503134).toLocaleString()} commits`)
    const bar = line(page).getByRole('progressbar')
    expect(Number(await bar.getAttribute('value'))).toBeCloseTo(412000 / 1503134, 3)

    await emit(page, status({ reading: false, read: 1503134, total: 1503134, done: true, commits: 1387733 }))
    await expect(line(page)).toHaveCount(0)
  })

  test("doesn't flash for a read that's over at once", async ({ page }) => {
    await open(page, 'trailerProgress', { responses: { TrailerStatus: status({ done: true }) } })
    await emit(page, status({ reading: true, read: 0, total: 2 }))
    await emit(page, status({ reading: false, read: 2, total: 2, done: true }))
    await page.waitForTimeout(900)
    await expect(line(page)).toHaveCount(0)
  })

  test('picks up a read that began before it was listening', async ({ page }) => {
    await open(page, 'trailerProgress', { responses: { TrailerStatus: status({ reading: true, read: 300000, total: 1503134 }) } })
    await expect(line(page).locator('.how')).toHaveText(`${(300000).toLocaleString()} of ${(1503134).toLocaleString()} commits`)
  })

  test('says nothing about a repository that is not the one on screen', async ({ page }) => {
    await open(page, 'trailerProgress', { responses: { TrailerStatus: status({ done: true }) } })
    await emit(page, status({ path: '/other', reading: true, read: 5000, total: 90000 }))
    await page.waitForTimeout(900)
    await expect(line(page)).toHaveCount(0)
  })
})

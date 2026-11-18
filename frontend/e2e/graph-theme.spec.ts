import { expect, test, type Page } from '@playwright/test'
import { open } from './support'

// The canvas can't read CSS variables, so the graph reads the theme's colours into it. It used
// to read them every frame. Now it keeps them until the theme changes, however that happens.

/** How light the ink on the canvas is: the mean of its solid pixels' colour, 0 to 255. The
 * page's ground isn't on the canvas, so this is the text and lines alone. */
const ink = (page: Page) =>
  page
    .locator('canvas')
    .first()
    .evaluate((c) => {
      const el = c as HTMLCanvasElement
      const px = el.getContext('2d')!.getImageData(0, 0, el.width, el.height).data
      let sum = 0
      let n = 0
      for (let i = 0; i < px.length; i += 4) {
        if (px[i + 3] < 200) continue
        sum += (px[i] + px[i + 1] + px[i + 2]) / 3
        n++
      }
      return n ? sum / n : -1
    })

/** The ink once it has stopped changing. */
async function settled(page: Page) {
  await page.waitForTimeout(300)
  let before = -2
  await expect
    .poll(async () => {
      const now = await ink(page)
      const same = Math.abs(now - before) < 0.5
      before = now
      return same
    })
    .toBe(true)
  return before
}

test('the graph is painted again in the new colours when the theme changes', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await open(page, 'graph', { graphRows: 40, theme: 'light', responses: { Refs: [], CommitSignatures: [] } })
  await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '40')
  const light = await settled(page)
  expect(light).toBeGreaterThan(0)

  // Set on the page itself, as the app does, with nothing telling the graph. Dark ink on a
  // light ground becomes light ink on a dark one.
  await page.evaluate(() => (document.documentElement.dataset.theme = 'dark'))
  const dark = await settled(page)
  expect(dark).toBeGreaterThan(light + 40)

  // And back: in the colours it started with, not ones kept from the dark theme.
  await page.evaluate(() => (document.documentElement.dataset.theme = 'light'))
  expect(Math.abs((await settled(page)) - light)).toBeLessThan(3)
})

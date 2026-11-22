import { expect, test, type Page } from '@playwright/test'
import { events, open, pickView } from './support'

// A pinned key's column in the Compact density (#868): the key is said once, in the
// heading, each commit's value sits under it, and the column goes when the window is narrow.

const id = (n: number) => String(n).padStart(40, '0')
const chip = (keyLabel: string, label: string, kind = 'label') => ({ key: keyLabel.toLowerCase(), keyLabel, value: `${kind}:${label}`, label, kind })
const fixed = chip('Changelog', 'fixed')
const sam = chip('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person')
const pins = [
  { key: 'changelog', label: 'Changelog' },
  { key: 'reviewed-by', label: 'Reviewed-by' },
]
const graph = (over: object = {}) => ({
  graphRows: 30,
  pins,
  graphChips: { [id(1)]: [fixed, sam], [id(2)]: [chip('Changelog', 'added')], [id(3)]: [sam] },
  responses: { Refs: [], CommitSignatures: [] },
  ...over,
})
const headings = (page: Page) => page.locator('.columns .pin-name')
const grid = (page: Page) => page.getByRole('grid', { name: 'Commit history' })
const findChip = (page: Page, saying: string) =>
  page.locator('.scroller').evaluate((el: HTMLElement, want: string) => {
    const box = el.getBoundingClientRect()
    for (let y = box.top + 8; y < box.bottom; y += 4) {
      for (let x = box.left + 120; x < box.right - 60; x += 6) {
        el.dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: y, bubbles: true }))
        if (el.title === want) return { x: x + 3, y }
      }
    }
    return null
  }, saying)

test.describe('a pinned key’s column in Compact', () => {
  test('names each key once, in the heading, left of the author', async ({ page }) => {
    await page.setViewportSize({ width: 1500, height: 700 })
    await open(page, 'graph', graph())
    // The timeline has no columns to name: there a pinned key's values follow the subject.
    await expect(headings(page)).toHaveCount(0)
    await pickView(page, 'Compact')
    await expect(headings(page)).toHaveText(['Changelog', 'Reviewed-by'])
    const boxes = await page.locator('.columns > span').evaluateAll((els) => els.map((e) => ({ text: (e.textContent ?? '').trim().split(/\s+/)[0], left: e.getBoundingClientRect().left })))
    const at = (name: string) => boxes.find((b) => b.text === name)!.left
    expect(at('Message')).toBeLessThan(at('Changelog'))
    expect(at('Changelog')).toBeLessThan(at('Reviewed-by'))
    expect(at('Reviewed-by')).toBeLessThan(at('Author'))
  })

  test('a value in the column filters by it, as a chip does', async ({ page }) => {
    await page.setViewportSize({ width: 1500, height: 700 })
    await open(page, 'graph', graph())
    await pickView(page, 'Compact')
    await expect(headings(page)).toHaveCount(2)
    await page.waitForTimeout(400)
    const at = await findChip(page, 'Filter the graph to Changelog: fixed')
    expect(at, 'the value "fixed" is drawn under Changelog').not.toBeNull()
    // It's under its heading.
    const head = (await headings(page).nth(0).boundingBox())!
    expect(Math.abs(at!.x - head.x)).toBeLessThan(20)
    await page.mouse.click(at!.x, at!.y)
    expect(await events(page, 'filter-pick')).toEqual([['changelog', 'label:fixed']])
    // A person is there by name, under the other heading.
    const who = await findChip(page, 'Filter the graph to Reviewed-by: Sam Ortiz')
    expect(who).not.toBeNull()
    expect(who!.x).toBeGreaterThan(at!.x + 100)
  })

  test('the heading’s button filters to the selected commit’s value', async ({ page }) => {
    await page.setViewportSize({ width: 1500, height: 700 })
    await open(page, 'graph', graph())
    await pickView(page, 'Compact')
    const button = page.getByRole('button', { name: "Filter to the selected commit's Changelog" })
    // The first commit has no Changelog: nothing to filter to, and the button says so.
    await expect(button).toBeDisabled()
    await expect(button).toHaveAttribute('title', 'The selected commit has no Changelog')
    await grid(page).focus()
    await page.keyboard.press('ArrowDown')
    await expect(button).toBeEnabled()
    await expect(button).toHaveAttribute('title', 'Filter to Changelog: fixed')
    await button.click()
    expect(await events(page, 'filter-pick')).toEqual([['changelog', 'label:fixed']])
    // Commit 3 was reviewed and has no Changelog.
    await grid(page).focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await expect(button).toBeDisabled()
    await expect(page.getByRole('button', { name: "Filter to the selected commit's Reviewed-by" })).toHaveAttribute('title', 'Filter to Reviewed-by: Sam Ortiz')
  })

  test('gives columns up as the window narrows, and moves nothing else', async ({ page }) => {
    await page.setViewportSize({ width: 1500, height: 700 })
    await open(page, 'graph', graph())
    await pickView(page, 'Compact')
    await expect(headings(page)).toHaveCount(2)
    const author = () => page.locator('.columns > span', { hasText: 'Author' }).evaluate((e) => window.innerWidth - e.getBoundingClientRect().left)
    const wide = await author()
    await page.setViewportSize({ width: 1200, height: 700 })
    await expect(headings(page)).toHaveText(['Changelog'])
    // The author's column is where it was, measured from the right edge.
    expect(await author()).toBe(wide)
    await page.setViewportSize({ width: 1000, height: 700 })
    await expect(headings(page)).toHaveCount(0)
    expect(await author()).toBe(wide)
  })

  test('has no column with nothing pinned', async ({ page }) => {
    await page.setViewportSize({ width: 1500, height: 700 })
    await open(page, 'graph', graph({ pins: [] }))
    await pickView(page, 'Compact')
    await expect(page.locator('.columns > span', { hasText: 'Author' })).toBeVisible()
    await expect(headings(page)).toHaveCount(0)
  })
})

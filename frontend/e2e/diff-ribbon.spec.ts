// #944: diffs drawn from the Go diff — changed words, moves, stepping through changes, and the
// split view's ribbon.
import { expect, test, type Page } from '@playwright/test'
import { open } from './support'

// harbour() moves below forecast(), forecast() renames `at` to `when` and gains a line, and
// 200 unchanged lines follow, so the sides have somewhere to scroll.
const tail = Array.from({ length: 200 }, (_, i) => `// tide table ${i}`)
const oldLines = ['package tides', '', 'func harbour() string {', '\treturn "Tidewater harbour master"', '}', '', 'func forecast(at time.Time) Tide {', '\treadings := fetch(at)', '\treturn predict(readings)', '}', '', ...tail]
const newLines = ['package tides', '', 'func forecast(when time.Time) Tide {', '\treadings := fetch(when)', '\tlog.Printf("forecast for %v", when)', '\treturn predict(readings)', '}', '', 'func harbour() string {', '\treturn "Tidewater harbour master"', '}', '', ...tail]
const text = (lines: string[]) => lines.map((l) => l + '\n').join('')

/** A span over the first `word` in a line. */
const at = (lines: string[], line: number, word: string) => {
  const from = lines[line].indexOf(word)
  return { line, from, to: from + word.length }
}

const diff = {
  blocks: [
    {
      oldStart: 2,
      oldEnd: 8,
      newStart: 2,
      newEnd: 5,
      old: [at(oldLines, 6, 'at'), at(oldLines, 7, 'at')],
      new: [at(newLines, 2, 'when'), at(newLines, 3, 'when'), { line: 4, from: 1, to: newLines[4].length }],
    },
    { oldStart: 11, oldEnd: 11, newStart: 8, newEnd: 12 },
  ],
  moves: [{ oldStart: 2, oldEnd: 5, newStart: 8, newEnd: 11, reindented: false }],
  algorithm: 'histogram',
  added: 7,
  removed: 6,
}

const side = (lines: string[]) => ({ exists: true, text: text(lines), size: text(lines).length })
const fileDiff = { contents: { path: 'internal/tides/forecast.go', kind: 'text', old: side(oldLines), new: side(newLines) }, diff }
const prefs = (ribbon: boolean) => ({ theme: '', graphDensity: '', experimentalViews: [], updateChecks: true, gitPath: '', gitInUse: 'git', cloneDir: '', zoom: 100, forgeColors: false, diffAlgorithm: '', diffRibbon: ribbon, diffMoves: true })

async function openDiff(page: Page, layout: 'unified' | 'split', ribbon = true) {
  await page.addInitScript((l) => localStorage.setItem('gitrevenge.diffLayout', l), layout)
  await open(page, 'diff', { commandKeys: true, responses: { FileDiff: fileDiff, Preferences: prefs(ribbon) } })
}

test.describe('the unified view', () => {
  test('says what changed, and marks the words', async ({ page }) => {
    await openDiff(page, 'unified')
    const summary = page.getByRole('group', { name: 'What changed' })
    await expect(summary).toContainText('7 lines added')
    await expect(summary).toContainText('6 lines removed')
    await expect(summary).toContainText('1 move')
    await expect(summary).toContainText('3 changes')
    await expect(page.locator('tr.del .chg').first()).toHaveText('at')
    await expect(page.locator('tr.add .chg').first()).toHaveText('when')
  })

  test('shows a moved block once, as a move', async ({ page }) => {
    await openDiff(page, 'unified')
    await expect(page.locator('tr.moved')).toHaveCount(6)
    await expect(page.locator('tr.del.moved .move-note')).toHaveText('moved to line 9')
    await expect(page.locator('tr.add.moved .move-note')).toHaveText('moved from line 3, unchanged')
  })

  test('F7 and Shift+F7 step through the changes', async ({ page }) => {
    await openDiff(page, 'unified')
    const where = page.getByRole('group', { name: 'What changed' }).locator('.where')
    await page.keyboard.press('F7')
    await expect(where).toHaveText('1 of 3')
    await expect(page.locator('tr.current', { hasText: 'func forecast(at' })).toHaveCount(1)
    await page.keyboard.press('F7')
    await expect(where).toHaveText('2 of 3')
    await expect(page.locator('tr.current .move-note').first()).toBeVisible()
    await page.keyboard.press('Shift+F7')
    await page.keyboard.press('Shift+F7')
    await expect(where).toHaveText('3 of 3')
    await page.getByRole('button', { name: 'Previous change' }).click()
    await expect(where).toHaveText('2 of 3')
  })
})

test.describe('the split view', () => {
  test('joins old lines to new ones with the ribbon', async ({ page }) => {
    await openDiff(page, 'split')
    await expect(page.getByText('Before · c0ffee0^')).toBeVisible()
    await expect(page.getByText('After · c0ffee0')).toBeVisible()
    const ribbon = page.locator('svg.gap')
    await expect(ribbon.locator('g.band.change')).toHaveCount(1)
    await expect(ribbon.locator('g.band.move')).toHaveCount(1)
    await expect(ribbon.locator('g.band.add')).toHaveCount(1)
    // The sides aren't padded: the old side is a plain file, one line per line.
    await expect(page.locator('[data-side="old"] .cm-line').first()).toHaveText('package tides')
    await expect(page.locator('.cm-mergeSpacer')).toHaveCount(0)
    // Moves cross the others, so they're drawn last.
    await expect(ribbon.locator('g.band').last()).toHaveClass(/move/)
  })

  test('marks the changed words and moved lines on each side', async ({ page }) => {
    await openDiff(page, 'split')
    await expect(page.locator('[data-side="old"] .cm-word-del').first()).toHaveText('at')
    await expect(page.locator('[data-side="new"] .cm-word-add').first()).toHaveText('when')
    await expect(page.locator('[data-side="old"] .cm-move-note')).toHaveText('moved to line 9')
    await expect(page.locator('[data-side="new"] .cm-moved')).toHaveCount(3)
  })

  test('outlines the change being read', async ({ page }) => {
    await openDiff(page, 'split')
    await page.getByRole('button', { name: 'Next change' }).click()
    await expect(page.locator('svg.gap g.band.current')).toHaveCount(1)
    await expect(page.locator('svg.gap g.band.current')).toHaveClass(/change/)
    await expect(page.locator('[data-side="new"] .cm-current').first()).toContainText('func forecast(when')
  })

  test('scrolls the two sides together', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 500 })
    await openDiff(page, 'split')
    await expect(page.locator('svg.gap g.band').first()).toBeAttached()
    const scrollers = page.locator('.pane .cm-scroller')
    // Wheel over the new side, as a reader would.
    await page.locator('[data-side="new"]').hover()
    await page.mouse.wheel(0, 1500)
    await expect.poll(() => scrollers.nth(0).evaluate((el) => el.scrollTop)).toBeGreaterThan(500)
    const [l, r] = await Promise.all([scrollers.nth(0).evaluate((el) => el.scrollTop), scrollers.nth(1).evaluate((el) => el.scrollTop)])
    // Past the changes the sides run one line apart: the new side has one line more.
    expect(Math.abs(r - l)).toBeLessThan(40)
  })

  test('pads the sides to line up when the ribbon is off', async ({ page }) => {
    await openDiff(page, 'split', false)
    await expect(page.locator('.cm-mergeView')).toBeVisible()
    await expect(page.locator('svg.gap')).toHaveCount(0)
    await expect(page.locator('.cm-word-add').first()).toHaveText('when')
  })
})

import { expect, test, type Locator, type Page } from '@playwright/test'
import { calls, open } from './support'

// A change or a commit of tens of thousands of files drew a row for every one of them. Now only
// the rows in view are in the DOM, and the keyboard still reaches the rest.

const COUNT = 20_000
/** A hundred files to a folder, so the flat list has a heading every hundred rows. */
const name = (i: number) => `dir-${String(Math.floor(i / 100)).padStart(3, '0')}/file-${String(i).padStart(5, '0')}.txt`
const short = (i: number) => name(i).slice(name(i).indexOf('/') + 1)
const FOLDERS = COUNT / 100
/** More than fills the view with some to spare, and nowhere near all of them. */
const DRAWN = 100

/** A timeline row's height: in the repository view the first commit starts this far down. */
const ROW = 52

const identity = { name: 'A', email: 'a@example.com' }
const workingTree = (files: unknown[]) => ({ status: { files }, identity, signing: { enabled: false } })

/** Scrolls a list's scroller to the end, as dragging its scrollbar would. */
const scrollToEnd = (scroller: Locator) => scroller.evaluate((el) => el.scrollTo(0, el.scrollHeight))

/** Whether a row is drawn in the middle of the scroller, not the blank left by rows scrolled away. */
const rowInTheMiddle = (scroller: Locator) =>
  scroller.evaluate((el) => {
    const box = el.getBoundingClientRect()
    return !!document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2)?.closest('li[data-row]')
  })

test.describe('a working tree with 20,000 changed files', () => {
  const files = Array.from({ length: COUNT }, (_, i) => ({ path: name(i), index: '.', workTree: 'M', untracked: false, conflicted: false }))
  const scenario = { graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: workingTree(files) } }
  const rows = (page: Page) => page.locator('.lists button.name')
  const row = (page: Page, i: number) => page.locator('.lists button.name', { hasText: short(i) })
  const ready = async (page: Page) => {
    await open(page, 'repoView', scenario)
    await expect(row(page, 0)).toBeVisible()
  }

  test('only draws the rows in view, and scrolling shows later ones', async ({ page }) => {
    await ready(page)
    await expect(page.getByRole('heading', { name: '20000 files changed' })).toBeVisible()
    await expect(page.getByText('Tick all 20000')).toBeVisible()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
    await expect(row(page, COUNT - 1)).toHaveCount(0)

    await scrollToEnd(page.locator('.lists'))
    await expect(row(page, COUNT - 1)).toBeInViewport()
    await expect(row(page, 0)).toHaveCount(0)
    expect(await rows(page).count()).toBeLessThan(DRAWN)

    // Part of the way, a wheel at a time.
    await page.locator('.lists').hover()
    await page.mouse.wheel(0, -90_000)
    await expect(row(page, COUNT - 1)).toHaveCount(0)
    await expect.poll(() => rowInTheMiddle(page.locator('.lists'))).toBe(true)
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('says where each row is among them all', async ({ page }) => {
    await ready(page)
    const item = (i: number) => page.locator('.lists li', { has: page.locator('button.name', { hasText: short(i) }) })
    // The first file comes after its folder's heading.
    await expect(item(0)).toHaveAttribute('aria-posinset', '2')
    await expect(item(0)).toHaveAttribute('aria-setsize', String(COUNT + FOLDERS))
    await scrollToEnd(page.locator('.lists'))
    await expect(item(COUNT - 1)).toHaveAttribute('aria-posinset', String(COUNT + FOLDERS))
  })

  test('the arrow keys, Home and End reach rows that were not drawn', async ({ page }) => {
    await ready(page)
    await row(page, 0).focus()
    await page.keyboard.press('End')
    await expect(row(page, COUNT - 1)).toBeFocused()
    await expect(row(page, COUNT - 1)).toBeInViewport()
    await page.keyboard.press('ArrowUp')
    await expect(row(page, COUNT - 2)).toBeFocused()
    expect(await rows(page).count()).toBeLessThan(DRAWN)

    await page.keyboard.press('Home')
    await expect(row(page, 0)).toBeFocused()
    await expect(row(page, 0)).toBeInViewport()
    // Down past the first folder's files, over the next heading, and out of the rows first drawn.
    for (let i = 0; i < 120; i++) await page.keyboard.press('ArrowDown')
    await expect(row(page, 120)).toBeFocused()
    await expect(row(page, 120)).toBeInViewport()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('a row keeps the focus when it scrolls out of view', async ({ page }) => {
    await ready(page)
    await row(page, 3).focus()
    await scrollToEnd(page.locator('.lists'))
    await expect(row(page, COUNT - 1)).toBeInViewport()
    await expect(row(page, 3)).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(row(page, 4)).toBeFocused()
    await expect(row(page, 4)).toBeInViewport()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('Space ticks a row the keyboard went to', async ({ page }) => {
    await ready(page)
    await row(page, 0).focus()
    await page.keyboard.press('End')
    await expect(row(page, COUNT - 1)).toBeFocused()
    await page.keyboard.press('Space')
    await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', [name(COUNT - 1)]]])
  })

  test('filters, and folds the tree, among them all', async ({ page }) => {
    await ready(page)
    await page.getByRole('searchbox', { name: 'Filter files' }).fill(short(COUNT - 1))
    await expect(rows(page)).toHaveCount(1)
    await expect(row(page, COUNT - 1)).toBeVisible()
    await page.getByRole('searchbox', { name: 'Filter files' }).fill('')
    await expect(row(page, 0)).toBeVisible()

    await page.getByRole('button', { name: 'Tree', exact: true }).click()
    await page.getByRole('button', { name: 'dir-000/', exact: true }).click()
    await expect(row(page, 0)).toHaveCount(0)
    await expect(row(page, 100)).toBeVisible()
    await row(page, 100).focus()
    await page.keyboard.press('End')
    await expect(row(page, COUNT - 1)).toBeFocused()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('the ignored files under them are windowed too', async ({ page }) => {
    const rule = { pattern: '*.log', source: '.gitignore', line: 1 }
    const ignored = Array.from({ length: COUNT }, (_, i) => ({ path: `logs/run-${String(i).padStart(5, '0')}.log`, rule, canExcept: true }))
    await open(page, 'repoView', { graphRows: 3, responses: { ...scenario.responses, IgnoredFiles: { files: ignored, truncated: false } } })
    await page.getByRole('button', { name: 'Ignored' }).click()
    await scrollToEnd(page.locator('.lists'))
    const last = page.locator('.ignored button.name', { hasText: 'run-19999.log' })
    await expect(last).toBeInViewport()
    expect(await page.locator('.lists li').count()).toBeLessThan(DRAWN)
    await last.focus()
    await page.keyboard.press('Home')
    await expect(page.locator('.ignored button.name', { hasText: 'run-00000.log' })).toBeFocused()
    await expect(page.locator('.ignored button.name', { hasText: 'run-00000.log' })).toBeInViewport()
  })
})

test.describe('a commit with 20,000 files', () => {
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const commit = { id: '0'.repeat(40), subject: 'Commit 0', body: 'The long story.', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
  const files = Array.from({ length: COUNT }, (_, i) => ({ status: 'M', path: name(i), added: 1, deleted: 0 }))
  const scenario = { graphRows: 3, responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: workingTree([]), CommitDetail: { commit, files }, CommitRow: 1 } }
  const rows = (page: Page) => page.locator('.files .file-row')
  const row = (page: Page, i: number) => page.locator('.files .file-row', { hasText: short(i) })
  const openCommit = async (page: Page) => {
    await open(page, 'repoView', scenario)
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + ROW + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
  }
  const openFiles = async (page: Page) => {
    await openCommit(page)
    await page.getByRole('tab', { name: 'Files', exact: true }).click()
    await expect(row(page, 0)).toBeVisible()
  }

  test('only draws the rows in view, and scrolling shows later ones', async ({ page }) => {
    await openFiles(page)
    expect(await rows(page).count()).toBeLessThan(DRAWN)
    await expect(row(page, COUNT - 1)).toHaveCount(0)
    await expect(page.locator('.files li').first()).toHaveAttribute('aria-setsize', String(COUNT + FOLDERS))

    await scrollToEnd(page.getByRole('tabpanel'))
    await expect(row(page, COUNT - 1)).toBeInViewport()
    await expect(row(page, 0)).toHaveCount(0)
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('the arrow keys, Home and End reach rows that were not drawn', async ({ page }) => {
    await openFiles(page)
    await row(page, 0).focus()
    await page.keyboard.press('End')
    await expect(row(page, COUNT - 1)).toBeFocused()
    await expect(row(page, COUNT - 1)).toBeInViewport()
    await page.keyboard.press('ArrowUp')
    await expect(row(page, COUNT - 2)).toBeFocused()
    await page.keyboard.press('Home')
    await expect(row(page, 0)).toBeFocused()
    for (let i = 0; i < 120; i++) await page.keyboard.press('ArrowDown')
    await expect(row(page, 120)).toBeFocused()
    await expect(row(page, 120)).toBeInViewport()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('filters, and folds the tree, among them all', async ({ page }) => {
    await openFiles(page)
    const panel = page.getByRole('tabpanel')
    await panel.getByRole('searchbox', { name: 'Filter files' }).fill(short(COUNT - 1))
    await expect(rows(page)).toHaveCount(1)
    await panel.getByRole('searchbox', { name: 'Filter files' }).fill('')
    await panel.getByRole('button', { name: 'Tree', exact: true }).click()
    await panel.getByRole('button', { name: 'dir-000/', exact: true }).click()
    await expect(row(page, 0)).toHaveCount(0)
    await expect(row(page, 100)).toBeVisible()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })

  test('beside the tabs, when wide, the list scrolls on its own', async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 800 })
    await openCommit(page)
    await page.getByRole('separator', { name: 'Resize the right panel' }).focus()
    for (let i = 0; i < 16; i++) await page.keyboard.press('ArrowLeft')
    await expect(page.locator('.side-files')).toBeVisible()
    await expect(row(page, 0)).toBeVisible()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
    await scrollToEnd(page.locator('.side-files'))
    await expect(row(page, COUNT - 1)).toBeInViewport()
    expect(await rows(page).count()).toBeLessThan(DRAWN)
  })
})

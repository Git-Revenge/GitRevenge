import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open, remote, tag } from './support'

test.describe('commit menu', () => {
  // #323: repeated labels threw each_key_duplicate, and no menu appeared.
  test('opens on a commit with two or more branches', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: [branch('main', { head: true }), remote('origin/main')] } })
    await expect(page.getByRole('menu')).toBeVisible()
    await expect(page.getByRole('menuitem').first()).toBeVisible()
    const labels = await page.getByRole('menuitem').allTextContents()
    expect(new Set(labels).size).toBe(labels.length)
  })

  test('opens on a commit with a branch and a tag', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: [branch('release/v1'), tag('v1.0')] } })
    await expect(page.getByRole('menu')).toBeVisible()
  })

  test('stays inside a short window', async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 400 })
    await open(page, 'commitMenu', { responses: { Refs: [branch('one'), branch('two'), remote('origin/one'), tag('v1')] } })
    const box = (await page.getByRole('menu').boundingBox())!
    expect(box.y).toBeGreaterThanOrEqual(0)
    expect(box.y + box.height).toBeLessThanOrEqual(400)
  })
})

test.describe('commit graph', () => {
  /** The row just inside the top edge, read by right-clicking it. */
  async function topRow(page: Page) {
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 400, box.y + 2, { button: 'right' })
    return (await events(page, 'context-row')).at(-1)?.[0]
  }

  // #340: reaching the top was ignored, leaving part of the first row cut off.
  test('scrolling back up reaches the first row', async ({ page }) => {
    await open(page, 'graph', { graphRows: 500, responses: { Refs: [], CommitSignatures: [] } })
    const scroller = page.locator('.scroller')
    await expect(scroller).toHaveAttribute('aria-rowcount', '500')
    const box = (await scroller.boundingBox())!
    await page.mouse.move(box.x + 400, box.y + 200)
    for (const delta of [120, 70, -60, -500]) {
      await page.mouse.wheel(0, delta)
      await page.waitForTimeout(120)
    }
    await expect.poll(() => scroller.evaluate((e) => e.scrollTop)).toBe(0)
    expect(await topRow(page)).toBe(0)
  })

  // #340: a selection before the first measure scrolled the graph one row down.
  test('opens at the top', async ({ page }) => {
    await open(page, 'graph', { graphRows: 500, responses: { Refs: [], CommitSignatures: [] } })
    const scroller = page.locator('.scroller')
    await expect(scroller).toHaveAttribute('aria-rowcount', '500')
    await page.waitForTimeout(200)
    expect(await scroller.evaluate((e) => e.scrollTop)).toBe(0)
  })
})

test.describe('start page', () => {
  const GROUPS = [
    { name: 'Work', paths: ['/r/api', '/r/web', '/r/docs', '/r/ops'] },
    { name: 'Side', paths: ['/r/game', '/r/notes'] },
    { name: 'Tools', paths: ['/r/cli'] },
  ]
  const scenario = () => ({ responses: { Groups: structuredClone(GROUPS) } })
  const card = (page: Page, name: string) => page.locator('[data-card]', { has: page.getByRole('heading', { name, exact: true }) })
  const names = (page: Page) => page.locator('[data-card] h3').allTextContents()
  const repos = (page: Page, name: string) => card(page, name).locator('.repo').allTextContents()

  async function drag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }) {
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    await page.mouse.move(to.x, to.y)
    await page.mouse.up()
  }
  const middle = (b: { x: number; y: number; width: number; height: number }) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 })

  test('dragging a card moves the group', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').hover()
    const from = middle((await card(page, 'Work').locator('.card-grip').boundingBox())!)
    const last = (await card(page, 'Tools').boundingBox())!
    await drag(page, from, { x: last.x + last.width - 4, y: last.y + 10 })
    await expect.poll(() => names(page)).toEqual(['Side', 'Tools', 'Work'])
    expect(await calls(page, 'GroupAction')).toContainEqual(['move', ['Work', '2']])
  })

  // #349: unpositioned rows stacked every grip and marker in the middle of the card.
  test('each repository has its grip beside its own row', async ({ page }) => {
    await open(page, 'start', scenario())
    const rows = card(page, 'Work').locator('[data-repo]')
    await expect(rows).toHaveCount(4)
    const gaps = await rows.evaluateAll((items) =>
      items.map((li) => {
        const grip = li.querySelector('.repo-grip')!.getBoundingClientRect()
        const row = li.getBoundingClientRect()
        return Math.abs(grip.top + grip.height / 2 - (row.top + row.height / 2))
      }),
    )
    for (const gap of gaps) expect(gap).toBeLessThanOrEqual(1)
  })

  test('dragging a repository moves it, and does not open it', async ({ page }) => {
    await open(page, 'start', scenario())
    const first = card(page, 'Work').locator('[data-repo]').first()
    await first.hover()
    const from = middle((await first.locator('.repo-grip').boundingBox())!)
    const target = (await card(page, 'Work').locator('[data-repo]').nth(2).boundingBox())!
    await drag(page, from, { x: from.x, y: target.y + target.height - 2 })
    await expect.poll(() => repos(page, 'Work')).toEqual(['web', 'docs', 'api', 'ops'])
    expect(await calls(page, 'OpenRepo')).toEqual([])
  })

  test('dropping a repository on another group changes nothing', async ({ page }) => {
    await open(page, 'start', scenario())
    const first = card(page, 'Work').locator('[data-repo]').first()
    await first.hover()
    const from = middle((await first.locator('.repo-grip').boundingBox())!)
    await drag(page, from, middle((await card(page, 'Side').boundingBox())!))
    await page.waitForTimeout(150)
    expect(await repos(page, 'Work')).toEqual(['api', 'web', 'docs', 'ops'])
    expect((await calls(page, 'GroupAction')).length).toBe(0)
  })

  test('Alt+arrows move a repository and keep focus', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').getByRole('button', { name: 'docs', exact: true }).focus()
    await page.keyboard.press('Alt+ArrowUp')
    await expect.poll(() => repos(page, 'Work')).toEqual(['api', 'docs', 'web', 'ops'])
    await expect(card(page, 'Work').getByRole('button', { name: 'docs', exact: true })).toBeFocused()
    await expect(page.locator('[aria-live]')).toContainText('Moved docs to position 2 of 4')
  })

  test('the menu moves a group, and is disabled at the ends', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').getByRole('button', { name: 'More actions for Work' }).click()
    await expect(page.getByRole('menuitem', { name: 'Move earlier' })).toHaveAttribute('aria-disabled', 'true')
    await page.getByRole('menuitem', { name: 'Move later' }).click()
    await expect.poll(() => names(page)).toEqual(['Side', 'Work', 'Tools'])
  })
})

test.describe('upstream dialog', () => {
  const refs = [branch('main', { upstream: 'origin/main' }), remote('origin/main'), remote('origin/x'), tag('v1')]
  const field = (page: Page) => page.getByLabel('Track')
  const save = (page: Page) => page.getByRole('button', { name: 'Save' })

  test('opens on the current upstream, suggesting remote branches', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await expect(field(page)).toHaveValue('origin/main')
    const options = await page.locator('datalist option').evaluateAll((o) => o.map((x) => (x as HTMLOptionElement).value))
    expect(options).toEqual(['origin/main', 'origin/x'])
  })

  test('sends a typed name, trimmed', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await field(page).fill('  origin/typed  ')
    await save(page).click()
    await expect.poll(() => calls(page, 'BranchAction')).toEqual([['/r', 'upstream', ['main', 'origin/typed']]])
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('emptying it stops tracking', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await field(page).fill('')
    await save(page).click()
    await expect.poll(() => calls(page, 'BranchAction')).toEqual([['/r', 'upstream', ['main', '']]])
  })

  test('keeps the dialog open with git’s error', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs }, failures: { BranchAction: 'the requested upstream branch does not exist' } })
    await save(page).click()
    await expect(page.getByRole('alert')).toContainText('does not exist')
    expect(await events(page, 'close')).toHaveLength(0)
  })
})

test('loading text waits, so a fast load does not flash', async ({ page }) => {
  await open(page, 'loading')
  await expect(page.getByRole('status')).toHaveCount(0)
  await expect(page.getByRole('status')).toContainText('Loading…')
})

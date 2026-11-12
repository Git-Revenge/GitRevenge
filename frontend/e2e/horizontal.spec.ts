import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open, pickView, tag } from './support'

// #651: an experimental view of the graph with time running across.
test.describe('the Horizontal view', () => {
  const id = (i: number) => String(i).padStart(40, '0')
  const DAY = 24 * 60 * 60
  // Noon three days running, six commits a day, so the days don't depend on the time zone.
  const noon = Math.floor(new Date(2026, 10, 11, 12).getTime() / 1000)
  const rows = Array.from({ length: 120 }, (_, i) => ({
    index: i,
    col: 0,
    color: 0,
    up: i ? [{ from: 0, to: 0, color: 0 }] : [],
    down: [{ from: 0, to: 0, color: 0 }],
    width: 1,
    id: id(i),
    parents: 1,
    author: 'Ris Adams',
    authorTime: noon - Math.floor(i / 6) * DAY - (i % 6) * 600,
    subject: `Commit number ${i}`,
  }))
  const graphRefs = { [id(0)]: [branch('main', { head: true, branchKind: 'trunk' })], [id(4)]: [tag('v1.0')] }
  const scenario = (views: object = { view: '', experimental: ['horizontal'] }) => ({
    graphData: rows,
    graphRefs,
    responses: { CommitSignatures: [], Refs: [branch('main', { head: true, branchKind: 'trunk' })], RepoPreferences: { hideKinds: [] }, GraphViews: views, HeadCommit: { branch: 'main', commit: id(0) } },
  })
  const view = (page: Page) => page.getByRole('listbox', { name: 'Commit history, newest first' })
  const commit = (page: Page, i: number) => page.getByRole('option', { name: new RegExp(`^Commit number ${i},`) })
  const lastSelected = async (page: Page) => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index

  test('is not in the menu until it is switched on', async ({ page }) => {
    await open(page, 'graph', scenario({ view: '', experimental: [] }))
    await page.getByRole('button', { name: /^Graph view/ }).click()
    await expect(page.getByRole('menuitemradio')).toHaveText(['Timeline', 'Compact'])
  })

  test('sits under an experimental heading in the menu, and choosing it shows and saves it', async ({ page }) => {
    await open(page, 'graph', scenario())
    await expect(view(page)).toHaveCount(0)
    await page.getByRole('button', { name: /^Graph view/ }).click()
    await expect(page.getByRole('menu')).toContainText('experimental')
    await page.getByRole('menuitemradio', { name: 'Horizontal, experimental' }).click()
    await expect(view(page)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Graph view: Horizontal' })).toBeVisible()
    expect(await calls(page, 'SetGraphView')).toEqual([['horizontal']])
    // The rows underneath take no input while they're covered.
    await expect(page.locator('.viewport')).toHaveAttribute('inert', '')
  })

  test('opens in it when it was the saved view', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await expect(view(page)).toBeVisible()
    await expect(commit(page, 0)).toBeVisible()
  })

  test('names the day across the top, with how many commits it has', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    const day = page.locator('.day').first()
    await expect(day).toContainText('Nov 11')
    await expect(day).toContainText('6 commits')
  })

  test('shows a branch and a tag over the commits they point at', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await expect(commit(page, 0).locator('.labels')).toContainText('main')
    await expect(page.getByRole('option', { name: /^Commit number 4,.*v1\.0/ }).locator('.labels')).toContainText('v1.0')
  })

  test('a click selects a commit and shows its subject in full below', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await commit(page, 3).click({ position: { x: 20, y: 300 } })
    await expect(commit(page, 3)).toHaveAttribute('aria-selected', 'true')
    await expect(page.locator('.detail')).toContainText('Commit number 3')
    await expect(page.locator('.detail')).toContainText('Ris Adams')
    expect(await lastSelected(page)).toBe(3)
  })

  test('Right goes to the older commit, Left to the newer, and Home and End to the ends', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await commit(page, 3).click({ position: { x: 20, y: 300 } })
    await page.keyboard.press('ArrowRight')
    await expect(commit(page, 4)).toHaveAttribute('aria-selected', 'true')
    await page.keyboard.press('ArrowLeft')
    await page.keyboard.press('ArrowLeft')
    await expect(commit(page, 2)).toHaveAttribute('aria-selected', 'true')
    // The last commit is far off the right edge: the view goes to it.
    await page.keyboard.press('End')
    await expect(commit(page, 119)).toHaveAttribute('aria-selected', 'true')
    await expect(commit(page, 119)).toBeInViewport()
    await page.keyboard.press('Home')
    await expect(commit(page, 0)).toHaveAttribute('aria-selected', 'true')
    await expect(commit(page, 0)).toBeInViewport()
  })

  test('the wheel scrolls sideways', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await expect(commit(page, 0)).toBeInViewport()
    await view(page).hover({ position: { x: 300, y: 300 } })
    await page.mouse.wheel(0, 1320)
    await expect(commit(page, 32)).toBeInViewport()
    await expect(commit(page, 0)).not.toBeInViewport()
  })

  test('a right-click selects the commit and asks for its menu', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await commit(page, 5).click({ button: 'right', position: { x: 20, y: 300 } })
    await expect(commit(page, 5)).toHaveAttribute('aria-selected', 'true')
    expect(await events(page, 'context-row')).toEqual([[5]])
  })

  test('going back to the timeline keeps the selected commit', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await commit(page, 7).click({ position: { x: 20, y: 300 } })
    await pickView(page, 'Timeline')
    await expect(view(page)).toHaveCount(0)
    await expect(page.locator('.viewport')).not.toHaveAttribute('inert')
    expect(await calls(page, 'SetGraphView')).toEqual([['']])
    // Down from where the selection was left.
    await page.locator('.scroller').focus()
    await page.keyboard.press('ArrowDown')
    await expect.poll(() => lastSelected(page)).toBe(8)
  })
})

test.describe('experimental views in settings', () => {
  const prefs = { theme: '', graphDensity: '', experimentalViews: [], updateChecks: true, gitPath: '', gitInUse: '/usr/bin/git', cloneDir: '', zoom: 100 }

  test('are off until asked for, and a tick is saved', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: prefs } })
    const box = page.getByRole('checkbox', { name: 'Horizontal' })
    await expect(box).not.toBeChecked()
    await box.check()
    await page.getByRole('button', { name: 'Save' }).click()
    await expect.poll(async () => ((await calls(page, 'SetPreferences'))[0]?.[0] as { experimentalViews: string[] } | undefined)?.experimentalViews).toEqual(['horizontal'])
  })

  test('shows one that is already on, and unticking it saves that', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: { ...prefs, experimentalViews: ['horizontal'] } } })
    const box = page.getByRole('checkbox', { name: 'Horizontal' })
    await expect(box).toBeChecked()
    await box.uncheck()
    await page.getByRole('button', { name: 'Save' }).click()
    await expect.poll(async () => ((await calls(page, 'SetPreferences'))[0]?.[0] as { experimentalViews: string[] } | undefined)?.experimentalViews).toEqual([])
  })
})

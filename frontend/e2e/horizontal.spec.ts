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

  // #657: a one-commit day is one column wide, and cut its name part way.
  test('a day too narrow for its name says just the day, whole', async ({ page }) => {
    const daily = rows.map((r, i) => ({ ...r, authorTime: noon - i * DAY }))
    await open(page, 'graph', { ...scenario({ view: 'horizontal', experimental: ['horizontal'] }), graphData: daily })
    await expect(commit(page, 0)).toBeVisible()
    const names = await page.locator('.day .meta').allInnerTexts()
    expect(names.length).toBeGreaterThan(5)
    for (const name of names) expect(name).toMatch(/^\d{1,2}$/)
    // The whole name is still there to point at.
    await expect(page.locator('.day').nth(1)).toHaveAttribute('title', /Nov 10/)
    // And a line marks where each day ends.
    expect(await page.locator('.daycut').count()).toBeGreaterThan(5)
  })

  // #657: in a narrow panel the strip dropped the subject and kept who and when.
  test('a narrow panel keeps the selected commit’s subject under the lanes', async ({ page }) => {
    await page.setViewportSize({ width: 300, height: 600 })
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    await commit(page, 1).click({ position: { x: 20, y: 300 } })
    const subject = (await page.locator('.detail .what').boundingBox())!
    expect(subject.width).toBeGreaterThan(120)
  })

  // #659: the pinned column names each lane; tags have a row of their own.
  test('names the lane in the pinned column, and puts a tag in the row over its commit', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    const names = page.getByRole('list', { name: 'Branches, by lane' })
    await expect(names.getByRole('listitem')).toHaveCount(1)
    await expect(names.getByRole('listitem').first()).toContainText('main')
    await expect(names).toContainText('you’re here')
    // main is said once, by the column: not again over its commit.
    await expect(page.locator('.mark')).toHaveCount(1)
    await expect(page.locator('.mark')).toContainText('v1.0')
    const mark = (await page.locator('.mark').boundingBox())!
    const column = (await commit(page, 4).boundingBox())!
    expect(mark.x).toBeGreaterThanOrEqual(column.x)
    expect(mark.x).toBeLessThan(column.x + column.width)
  })

  test('keeps the lane’s name pinned when its tip has scrolled away', async ({ page }) => {
    await open(page, 'graph', scenario({ view: 'horizontal', experimental: ['horizontal'] }))
    const names = page.getByRole('list', { name: 'Branches, by lane' })
    await expect(names).toContainText('main')
    await view(page).hover({ position: { x: 300, y: 300 } })
    await page.mouse.wheel(0, 1760)
    await expect(commit(page, 0)).not.toBeInViewport()
    await expect(names.getByRole('listitem').first()).toContainText('main')
    const box = (await names.boundingBox())!
    expect(box.x).toBeLessThan(5)
  })

  test('names a second lane, and scrolls the lanes when there are more than fit', async ({ page }) => {
    await page.setViewportSize({ width: 1000, height: 420 })
    // Twelve lanes side by side for the first rows: a tip on each, all joining main at row 12.
    const wide = rows.map((r, i) => {
      if (i >= 12) return { ...r, width: 1 }
      const pass = Array.from({ length: i }, (_, k) => ({ from: k + 1, to: k + 1, color: k + 1 }))
      const keep = [{ from: 0, to: 0, color: 0 }, ...pass]
      const own = i === 0 ? [] : [{ from: i, to: i, color: i }]
      return i === 0 ? { ...r, width: 12 } : { ...r, col: i, color: i, width: 12, up: keep, down: [...keep, ...own] }
    })
    const refs = { ...graphRefs, [id(3)]: [branch('topic-three')], [id(11)]: [branch('topic-eleven')] }
    await open(page, 'graph', { ...scenario({ view: 'horizontal', experimental: ['horizontal'] }), graphData: wide, graphRefs: refs })
    const names = page.getByRole('list', { name: 'Branches, by lane' })
    await expect(names).toContainText('topic-three')
    // The lanes run past the bottom: the column says so, and the wheel over it brings them up.
    await expect(names).toContainText('more below')
    await expect(names.getByText('topic-eleven')).not.toBeInViewport()
    await names.hover()
    await page.mouse.wheel(0, 400)
    await expect(names.getByText('topic-eleven')).toBeInViewport()
    await expect(names).toContainText('more above')
  })

  // #663: the scrollbar is marked as the timeline's ruler is.
  test('marks the scrollbar where the checked-out commit, a tag and a note are', async ({ page }) => {
    const marks = { loaded: 120, months: [{ row: 0, time: noon }, { row: 60, time: noon - 10 * DAY }], tips: [0], tags: [4, 90], notes: [30] }
    const s = scenario({ view: 'horizontal', experimental: ['horizontal'] })
    await open(page, 'graph', { ...s, responses: { ...s.responses, RulerMarks: marks } })
    const bar = page.locator('.bar')
    await expect(bar.locator('.tick .head')).toHaveCount(1)
    await expect(bar.locator('.tick .tags')).toHaveCount(2)
    await expect(bar.locator('.tick .notes')).toHaveCount(1)
    await expect(bar.locator('.month')).toHaveCount(1)
    // A tick three quarters of the way through the history sits three quarters along the bar.
    const box = (await bar.boundingBox())!
    const far = (await bar.locator('.tick').last().boundingBox())!
    expect((far.x - box.x) / box.width).toBeGreaterThan(0.7)
    expect((far.x - box.x) / box.width).toBeLessThan(0.8)
    // Pointing at one says what's there.
    await page.mouse.move(far.x, box.y + box.height / 2)
    await expect(bar).toHaveAttribute('title', '1 tag')
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

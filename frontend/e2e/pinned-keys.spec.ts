import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// A trailer key pinned to the graph's rows (#831): its values are chips after the subject,
// a click on one filters the graph, and the pin is in the Smart filters panel and beside
// each key in a commit's Trailers tab.

const id = (n: number) => String(n).padStart(40, '0')
const chip = (keyLabel: string, label: string, kind = 'label') => ({ key: keyLabel.toLowerCase(), keyLabel, value: `${kind}:${label}`, label, kind })

test.describe('chips on the graph’s rows', () => {
  const fixed = chip('Changelog', 'fixed')
  const sam = chip('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person')
  const graph = { graphRows: 30, graphChips: { [id(1)]: [fixed, sam], [id(2)]: [chip('Changelog', 'added')] }, responses: { Refs: [], CommitSignatures: [] } }
  const grid = (page: Page) => page.getByRole('grid', { name: 'Commit history' })
  // The first place along commit n's row where the pointer is on a chip, found by what the
  // graph says it would do there.
  // The pointer is moved by the page itself, a step at a time: thousands of real moves take
  // a browser too long.
  const findChip = (page: Page, n: number, saying: string) =>
    page.locator('.scroller').evaluate((el: HTMLElement, want: string) => {
      const box = el.getBoundingClientRect()
      for (let y = box.top + 20; y < box.bottom; y += 5) {
        for (let x = box.left + 120; x < box.right - 60; x += 6) {
          el.dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: y, bubbles: true }))
          if (el.title === want) return { x: x + 3, y }
        }
      }
      return null
    }, saying)

  test('a click on a chip puts its value in the filter, and selects nothing', async ({ page }) => {
    await open(page, 'graph', graph)
    await expect(grid(page)).toBeVisible()
    await page.waitForTimeout(500)
    const at = await findChip(page, 1, 'Filter the graph to Changelog: fixed')
    expect(at, 'a chip that says Changelog: fixed is drawn on a row').not.toBeNull()
    const before = (await events(page, 'select')).length
    await page.mouse.click(at!.x, at!.y)
    expect(await events(page, 'filter-pick')).toEqual([['changelog', 'label:fixed']])
    expect((await events(page, 'select')).length).toBe(before)
    // Off the chip the row is a row again.
    await page.mouse.move(at!.x - 110, at!.y)
    await expect(grid(page)).toHaveAttribute('title', '')
  })

  test('a person’s chip says them by name', async ({ page }) => {
    await open(page, 'graph', graph)
    await page.waitForTimeout(500)
    expect(await findChip(page, 1, 'Filter the graph to Reviewed-by: Sam Ortiz')).not.toBeNull()
  })

  test('the selected row’s chips are buttons for the keyboard, seen once focused', async ({ page }) => {
    await open(page, 'graph', graph)
    await grid(page).focus()
    await page.keyboard.press('ArrowDown')
    await expect.poll(async () => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index).toBe(1)
    const button = page.getByRole('button', { name: 'Changelog: fixed, filter' })
    await expect(button).toHaveCount(1)
    await expect(button).toHaveAttribute('aria-pressed', 'false')
    // Out of sight until it has the focus.
    expect((await button.boundingBox())!.width).toBeLessThan(3)
    await button.focus()
    await expect(button).toBeFocused()
    await page.keyboard.press('Tab')
    await page.keyboard.press('Shift+Tab')
    expect((await button.boundingBox())!.width).toBeGreaterThan(40)
    await page.keyboard.press('Enter')
    expect(await events(page, 'filter-pick')).toEqual([['changelog', 'label:fixed']])
    await expect(page.getByRole('button', { name: 'Reviewed-by: Sam Ortiz, filter' })).toHaveCount(1)
  })

  test('a chip in the filter says it is pressed', async ({ page }) => {
    await open(page, 'graph', { ...graph, filterPicks: [{ key: 'changelog', value: 'label:fixed' }] })
    await grid(page).focus()
    await page.keyboard.press('ArrowDown')
    await expect(page.getByRole('button', { name: 'Changelog: fixed, filter' })).toHaveAttribute('aria-pressed', 'true')
    await expect(page.getByRole('button', { name: 'Reviewed-by: Sam Ortiz, filter' })).toHaveAttribute('aria-pressed', 'false')
  })

  test('rows with no chips have no chip buttons, and nothing to pick', async ({ page }) => {
    await open(page, 'graph', graph)
    await grid(page).focus()
    for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowDown')
    await expect.poll(async () => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index).toBe(3)
    await expect(page.getByRole('button', { name: /, filter$/ })).toHaveCount(0)
  })
})

test.describe('the pin in the Smart filters panel', () => {
  const value = (label: string, kind: string, commits: number) => ({ id: `${kind}:${label}`, label, kind, commits })
  const key = (label: string, kind: string, commits: number) => ({ id: label.toLowerCase(), label, spellings: [{ text: label, commits }], kind, commits, lines: commits, values: 2, held: '', known: true })
  const panel = {
    keys: [
      { key: key('Reviewed-by', 'person', 40), top: [value('Sam Ortiz <sam@tidewater.example>', 'person', 30)], more: 0 },
      { key: key('Changelog', 'label', 21), top: [value('fixed', 'label', 14), value('added', 'label', 7)], more: 0 },
      { key: key('Closes', 'ticket', 9), top: [value('#12', 'ticket', 2)], more: 0 },
    ],
    moreKeys: 0,
    held: [],
    matches: 0,
    status: { reading: false, read: 212, total: 212, done: true, commits: 140 },
  }
  const scenario = (over: object = {}) => ({ responses: { Refs: [], ShowingHidden: false, Worktrees: [], TrailerPanel: panel }, ...over })
  const show = async (page: Page, s: object = scenario()) => {
    await open(page, 'sidebar', s)
    // The sidebar remembers its panel: the second time it's already showing.
    const tree = page.getByRole('tree', { name: 'Smart filters' })
    if (!(await tree.isVisible().catch(() => false))) {
      await page.getByRole('navigation', { name: 'Sidebar panels' }).getByRole('button', { name: /^Smart filters/ }).click()
    }
    await expect(tree).toBeVisible()
  }
  const offer = (page: Page) => page.getByRole('group', { name: 'Show a trailer on the graph' })

  test('offers the repository’s label once, and pins it when asked', async ({ page }) => {
    await show(page)
    await expect(offer(page)).toContainText('Show Changelog on the graph’s rows?')
    await offer(page).getByRole('button', { name: 'Pin it' }).click()
    expect(await events(page, 'pin')).toEqual([['changelog', true]])
    await expect(offer(page)).toHaveCount(0)
    // It isn't offered again for this repository.
    await show(page)
    await expect(offer(page)).toHaveCount(0)
  })

  test('"Not now" is an answer too', async ({ page }) => {
    await show(page)
    await offer(page).getByRole('button', { name: 'Not now' }).click()
    await expect(offer(page)).toHaveCount(0)
    expect(await events(page, 'pin')).toEqual([])
    await show(page)
    await expect(offer(page)).toHaveCount(0)
  })

  test('offers nothing once a key is pinned', async ({ page }) => {
    await show(page, scenario({ pinnedKeys: ['reviewed-by'] }))
    await expect(offer(page)).toHaveCount(0)
  })

  test('each key has a pin, which says what it would do', async ({ page }) => {
    await show(page, scenario({ pinnedKeys: ['changelog'] }))
    const tree = page.getByRole('tree', { name: 'Smart filters' })
    // A pinned key's pin is always there to see.
    const pinned = tree.getByRole('button', { name: 'Unpin Changelog from the graph’s rows' })
    await expect(pinned).toBeVisible()
    await expect(pinned).toHaveAttribute('aria-pressed', 'true')
    await pinned.click()
    expect(await events(page, 'pin')).toEqual([['changelog', false]])
    // Another key's pin shows when its row is pointed at, and takes no room until then.
    const other = tree.getByRole('button', { name: 'Pin Closes to the graph’s rows', includeHidden: true })
    await tree.getByRole('treeitem').filter({ hasText: 'Changelog' }).first().hover()
    await expect(other).toBeHidden()
    await tree.getByRole('treeitem').filter({ hasText: 'Closes' }).first().hover()
    await expect(other).toBeVisible()
    await expect(other).toHaveAttribute('aria-pressed', 'false')
    await other.click()
    expect((await events(page, 'pin')).at(-1)).toEqual(['closes', true])
    // The pin doesn't fold the key or pick anything.
    expect(await events(page, 'filter')).toEqual([])
    // A screen reader hears that the key is pinned.
    await expect(tree.getByRole('treeitem').filter({ hasText: 'Changelog' }).first()).toContainText('pinned to the graph’s rows')
  })

  test('the menu key opens a key’s menu, where the keyboard pins', async ({ page }) => {
    await show(page)
    const tree = page.getByRole('tree', { name: 'Smart filters' })
    await tree.focus()
    await page.keyboard.press('ContextMenu')
    const menus = await events(page, 'menu')
    expect(menus).toHaveLength(1)
    expect((menus[0][0] as { label: string }[]).map((m) => m.label)).toEqual(['Pin Reviewed-by to the graph’s rows'])
    // And a right click on a key does the same.
    await tree.getByRole('treeitem').filter({ hasText: 'Changelog' }).first().click({ button: 'right' })
    expect(((await events(page, 'menu')).at(-1)?.[0] as { label: string }[])[0].label).toBe('Pin Changelog to the graph’s rows')
  })
})

test.describe('the pin in a commit’s Trailers tab', () => {
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const trailers = [
    { key: 'Closed', value: '#12' },
    { key: 'Reviewed-by', value: 'Sam Ortiz <sam@tidewater.example>' },
  ]
  const commit = { id: id(0), subject: 'Commit 0', body: 'The long story.', trailers, parents: ['a'.repeat(40)], author: person, committer: person }
  const pin = (keyName: string) => ({ key: keyName, label: keyName, kind: 'ticket', scope: 'local' })
  const scenario = (pins: object[]) => ({
    graphRows: 3,
    responses: {
      Refs: [],
      CommitSignatures: [],
      CommitNotes: [],
      WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
      CommitDetail: { commit, files: [] },
      CommitRow: 1,
      TrailerFilter: { picks: [], only: true, kept: 0, total: 3 },
      TrailerPins: pins,
      SetTrailerPin: [pin('closes')],
    },
  })
  const ROW = 78

  test('pins a key by the name the commit wrote it under, and shows what is pinned', async ({ page }) => {
    await open(page, 'repoView', scenario([]))
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + ROW + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    await page.getByRole('tab', { name: 'Trailers', exact: true }).click()
    const closes = page.getByRole('button', { name: 'Pin Closed to the graph’s rows' })
    await expect(closes).toHaveAttribute('aria-pressed', 'false')
    await closes.click()
    await expect.poll(() => calls(page, 'SetTrailerPin')).toHaveLength(1)
    const asked = (await calls(page, 'SetTrailerPin'))[0]
    expect(asked[0]).toBe('/r')
    // By the key as the commit wrote it: the backend knows "Closed" is Closes.
    expect(asked[1]).toBe('Closed')
    expect(asked[2]).toBe(true)
    // The backend says Closes is pinned: "Closed" is the same key.
    await expect(page.getByRole('button', { name: 'Unpin Closed from the graph’s rows' })).toHaveAttribute('aria-pressed', 'true')
    await expect(page.getByRole('button', { name: 'Pin Reviewed-by to the graph’s rows' })).toHaveAttribute('aria-pressed', 'false')
  })
})

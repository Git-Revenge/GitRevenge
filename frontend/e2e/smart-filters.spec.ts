import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open } from './support'

// The sidebar's Smart filters panel (#828): what a repository's trailers offer to filter
// by. The backend is stubbed, so these check what the panel shows, what it asks for, and
// what a pick does: a filter is a search of the graph, and nothing is ever written.

const value = (label: string, kind: string, commits: number, over: object = {}) => ({ id: `${kind}:${label}`, label, kind, commits, ...over })
const key = (label: string, kind: string, commits: number, over: object = {}) => ({ id: label.toLowerCase(), label, spellings: [{ text: label, commits }], kind, commits, lines: commits, values: 1, held: '', known: true, ...over })
const done = { reading: false, read: 212, total: 212, done: true, commits: 140 }

const reviewed = {
  key: key('Reviewed-by', 'person', 31, { spellings: [{ text: 'Reviewed-by', commits: 29 }, { text: 'Reviewed-By', commits: 2 }] }),
  top: [value('Sam Ortiz <sam@tidewater.example>', 'person', 22), value('Sarah Lind <sarah@tidewater.example>', 'person', 9), value('dependabot[bot] <bot@users.noreply.example>', 'person', 40, { bot: true })],
  more: 24,
}
const closes = { key: key('Closes', 'ticket', 18), top: [value('#14', 'ticket', 3), value('#12', 'ticket', 1)], more: 0 }
const changelog = { key: key('Changelog', 'label', 11, { known: false }), top: [value('fixed', 'label', 7), value('added', 'label', 4)], more: 0 }
const refs = { key: key('Refs', 'ticket', 6), top: [value('TIDE-88', 'ticket', 4)], more: 0 }
const note = { key: key('Note', 'text', 5, { known: false }), top: [value('revert if the feed changes', 'text', 1)], more: 0 }
const held = [key('Signed-off-by', 'person', 200, { held: 'everywhere' }), key('Change-Id', 'identifier', 90, { held: 'identifier', known: false }), key('docs', 'text', 1, { held: 'rare', known: false })]
const panel = { keys: [reviewed, closes, changelog, refs, note], moreKeys: 0, held, matches: 0, status: done }

const scenario = (over: object = {}, more: object = {}) => ({ responses: { Refs: [branch('main', { head: true })], ShowingHidden: false, Worktrees: [], TrailerPanel: panel, ...over }, ...more })
const rail = (page: Page) => page.getByRole('navigation', { name: 'Sidebar panels' })
const tree = (page: Page) => page.getByRole('tree', { name: 'Smart filters' })
const item = (page: Page, name: string | RegExp) => tree(page).getByRole('treeitem').filter({ hasText: name })
/** A key's folder, by its exact name: "Note" is also in a value's text. */
const folder = (page: Page, name: string) => tree(page).locator('.row.key').filter({ has: page.locator('.name', { hasText: new RegExp(`^${name}$`) }) })
async function show(page: Page, s: object = scenario()) {
  await open(page, 'sidebar', s)
  await rail(page).getByRole('button', { name: /^Smart filters/ }).click()
  await expect(tree(page)).toBeVisible()
}

test.describe('the Smart filters panel', () => {
  test('lists keys as folders, with what they hold and their values', async ({ page }) => {
    await show(page)
    await expect(page.getByRole('heading', { name: 'Smart filters' })).toBeVisible()
    await expect(page.locator('.heading .from')).toHaveText('from trailers')
    expect(await calls(page, 'TrailerPanel')).toEqual([['/repo', '', 0]])

    // A key: its name, the kind of thing its values are, and how many commits have it.
    const reviewedBy = folder(page, 'Reviewed-by')
    await expect(reviewedBy).toHaveAttribute('aria-expanded', 'true')
    await expect(reviewedBy.locator('.kind')).toHaveText('people')
    await expect(reviewedBy.locator('.count')).toHaveText('31')
    await expect(reviewedBy).toHaveAttribute('title', 'Written as Reviewed-by, Reviewed-By')
    await expect(item(page, 'Closes').locator('.kind')).toHaveText('tickets')
    await expect(item(page, 'Changelog').locator('.kind')).toHaveText('labels')

    // Values: a person by name, a bot marked as one, and the rest behind "more".
    const sam = item(page, 'Sam Ortiz')
    await expect(sam).toHaveText(/^Sam Ortiz\s*22$/)
    await expect(item(page, 'dependabot[bot]').locator('.bot')).toHaveText('bot')
    await expect(item(page, '24 more…')).toBeVisible()

    // The four keys used most start open; the rest wait to be asked.
    await expect(item(page, 'Refs')).toHaveAttribute('aria-expanded', 'true')
    await expect(folder(page, 'Note')).toHaveAttribute('aria-expanded', 'false')
    await expect(item(page, 'revert if the feed changes')).toHaveCount(0)
  })

  test('picking a value searches the graph for it, and picking it again clears the search', async ({ page }) => {
    await show(page)
    await item(page, 'Sam Ortiz').click()
    expect(await events(page, 'search')).toEqual([['Reviewed-by: Sam Ortiz']])
    await item(page, 'fixed').click()
    expect((await events(page, 'search')).at(-1)).toEqual(['Changelog: fixed'])
  })

  test('the value the graph is searching for is the picked one', async ({ page }) => {
    await show(page, scenario({}, { search: 'changelog: fixed' }))
    const fixed = item(page, 'fixed')
    await expect(fixed).toHaveAttribute('aria-selected', 'true')
    await expect(fixed).toHaveClass(/picked/)
    await expect(item(page, 'added')).toHaveAttribute('aria-selected', 'false')
    await fixed.click()
    expect(await events(page, 'search')).toEqual([['']])
  })

  test('"more…" brings the rest of a key\'s values', async ({ page }) => {
    const longer = { ...reviewed, top: [...reviewed.top, value('Maya Okafor <maya@tidewater.example>', 'person', 5)], more: 23 }
    await show(page, scenario({ TrailerMore: longer }))
    await item(page, '24 more…').click()
    await expect(item(page, 'Maya Okafor')).toBeVisible()
    await expect(item(page, '23 more…')).toBeVisible()
    expect(await calls(page, 'TrailerMore')).toEqual([['/repo', 'reviewed-by', '', 53]])
  })

  test('says how many keys are held back, why, and still lets them be searched for', async ({ page }) => {
    await show(page)
    await expect(page.locator('.held-line')).toContainText('3 keys held back')
    await expect(page.getByRole('list', { name: 'Keys held back' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Show them' }).click()
    const rows = page.getByRole('list', { name: 'Keys held back' }).getByRole('listitem')
    await expect(rows).toHaveCount(3)
    await expect(rows.nth(0)).toContainText('Signed-off-by')
    await expect(rows.nth(0)).toContainText('on nearly every commit')
    await expect(rows.nth(1)).toContainText('a different value on nearly every commit')
    await expect(rows.nth(2)).toContainText('on only a commit or two')
    await rows.nth(2).getByRole('button').click()
    expect(await events(page, 'search')).toEqual([['docs:']])
  })

  test('folders open and close, and remember it', async ({ page }) => {
    await show(page)
    await folder(page, 'Note').click()
    await expect(item(page, 'revert if the feed changes')).toBeVisible()
    await item(page, 'Closes').click()
    await expect(item(page, '#14')).toHaveCount(0)
    const saved = await page.evaluate(() => JSON.parse(localStorage.getItem('gitrevenge.sidebar:/repo') ?? '{}'))
    expect(saved.panel).toBe('filters')
    expect(saved.open).toMatchObject({ 'filter:note': true, 'filter:closes': false })
  })

  test('the sidebar\'s filter narrows it, and the rail counts its matches', async ({ page }) => {
    const narrowed = { ...panel, keys: [{ ...reviewed, top: [reviewed.top[1]], more: 0 }], held: [], matches: 1 }
    await open(page, 'sidebar', scenario())
    // Filtering from another panel asks this one too, for the rail's count.
    await page.evaluate((p) => (window as unknown as { __respond: (r: object) => void }).__respond({ TrailerPanel: p }), narrowed)
    await page.getByRole('searchbox', { name: 'Filter everything in the sidebar' }).fill('sarah')
    await expect.poll(async () => (await calls(page, 'TrailerPanel')).at(-1)).toEqual(['/repo', 'sarah', 0])
    const tab = rail(page).getByRole('button', { name: /^Smart filters/ })
    await expect(tab).toHaveAttribute('aria-label', 'Smart filters, 1 match')
    await tab.click()
    await expect(item(page, 'Sarah Lind').locator('mark')).toHaveText('Sarah')
    // Held keys aren't part of what the filter narrows.
    await expect(page.locator('.held-line')).toHaveCount(0)
  })

  test('says the counts are so far while the history is still being read', async ({ page }) => {
    await show(page, scenario({ TrailerPanel: { ...panel, status: { reading: true, read: 412000, total: 1503134, done: false, commits: 380000 } } }))
    await expect(page.getByRole('status')).toHaveText(`Counts so far: ${(412000).toLocaleString()} of ${(1503134).toLocaleString()} commits read.`)
  })

  test('a history with no trailers says so, and names the way to add one', async ({ page }) => {
    await open(page, 'sidebar', scenario({ TrailerPanel: { keys: [], moreKeys: 0, held: [], matches: 0, status: done } }))
    await rail(page).getByRole('button', { name: /^Smart filters/ }).click()
    await expect(page.locator('.scroll:not([hidden])')).toContainText('No trailers in this history yet.')
    await expect(page.locator('.scroll:not([hidden])')).toContainText('Trailer, in the commit form, adds one.')
  })

  test('works from the keyboard: arrows move and open, Enter picks, typing jumps', async ({ page }) => {
    await show(page)
    await tree(page).focus()
    const active = () => tree(page).locator('.row.active')
    await expect(active()).toContainText('Reviewed-by')
    await page.keyboard.press('ArrowDown')
    await expect(active()).toContainText('Sam Ortiz')
    await page.keyboard.press('Enter')
    expect(await events(page, 'search')).toEqual([['Reviewed-by: Sam Ortiz']])
    // Left goes to the key, and again closes it.
    await page.keyboard.press('ArrowLeft')
    await expect(active()).toContainText('Reviewed-by')
    await page.keyboard.press('ArrowLeft')
    await expect(item(page, 'Sam Ortiz')).toHaveCount(0)
    await page.keyboard.press('ArrowRight')
    await expect(item(page, 'Sam Ortiz')).toBeVisible()
    // Typing jumps to a name.
    await page.keyboard.type('chang')
    await expect(active()).toContainText('Changelog')
    await page.keyboard.press('End')
    await expect(active()).toContainText('Note')
    await page.keyboard.press('Home')
    await expect(active()).toContainText('Reviewed-by')
  })
})

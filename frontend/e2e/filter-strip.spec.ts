import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open } from './support'

// The graph's smart filter (#829): the strip above the graph, and what picking a value in
// the sidebar does to the graph. Search moves between matches; a filter changes what the
// graph shows.

const pick = (key: string, keyLabel: string, value: string, valueLabel: string, kind: string) => ({ key, value, keyLabel, valueLabel, kind })
const fixed = pick('changelog', 'Changelog', 'label:fixed', 'fixed', 'label')
const sam = pick('reviewed-by', 'Reviewed-by', 'person:Sam Ortiz <sam@tidewater.example>', 'Sam Ortiz <sam@tidewater.example>', 'person')
const strip = (page: Page) => page.getByRole('group', { name: 'Smart filter' })
const modes = (page: Page) => page.getByRole('group', { name: 'What a filter does to other commits' })

test.describe('the filter strip', () => {
  test('says how many commits are kept, and shows a chip for each value', async ({ page }) => {
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed, sam], only: true, kept: 11, total: 212 } } })
    await expect(strip(page).getByRole('status')).toHaveText('11 of 212 commits')
    const chips = strip(page).locator('.chip')
    await expect(chips).toHaveCount(2)
    await expect(chips.nth(0)).toHaveText(/Changelog\s*fixed/)
    // A person is on the chip by name.
    await expect(chips.nth(1).locator('.value')).toHaveText('Sam Ortiz')

    await strip(page).getByRole('button', { name: 'Remove the Changelog: fixed filter' }).click()
    expect((await events(page, 'remove'))[0][0]).toMatchObject({ key: 'changelog', value: 'label:fixed' })
    await strip(page).getByRole('button', { name: 'Filter', exact: true }).click()
    expect(await events(page, 'add')).toHaveLength(1)
    await strip(page).getByRole('button', { name: 'Clear' }).click()
    expect(await events(page, 'clear')).toHaveLength(1)
  })

  test('switches between Only these and Highlight', async ({ page }) => {
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed], only: true, kept: 11, total: 212 }, only: true } })
    await expect(modes(page).getByRole('button', { name: 'Only these' })).toHaveAttribute('aria-pressed', 'true')
    await expect(modes(page).getByRole('button', { name: 'Highlight' })).toHaveAttribute('aria-pressed', 'false')
    await modes(page).getByRole('button', { name: 'Highlight' }).click()
    expect(await events(page, 'mode')).toEqual([[false]])
  })

  // Enter on the graph stays the author's card, so stepping between highlighted commits has
  // its own buttons (#866). Under Only these every row is a match: the arrow keys step.
  test('under Highlight, steps between the commits the filter keeps', async ({ page }) => {
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed], only: false, kept: 11, total: 212 }, only: false } })
    await strip(page).getByRole('button', { name: 'Next match' }).click()
    await strip(page).getByRole('button', { name: 'Previous match' }).click()
    expect(await events(page, 'step')).toEqual([[1], [-1]])
  })

  test('has no step buttons under Only these, nor when the filter keeps nothing', async ({ page }) => {
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed], only: true, kept: 11, total: 212 }, only: true } })
    await expect(strip(page).getByRole('button', { name: 'Next match' })).toHaveCount(0)
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed], only: false, kept: 0, total: 212 }, only: false } })
    await expect(strip(page).getByRole('button', { name: 'Next match' })).toBeDisabled()
  })

  test('offers only Highlight where taking commits out would break the lanes', async ({ page }) => {
    await open(page, 'filterStrip', { filterStrip: { filter: { picks: [fixed], only: false, kept: 11, total: 0 }, only: false, onlyOffered: false } })
    const onlyThese = modes(page).getByRole('button', { name: 'Only these' })
    await expect(onlyThese).toBeDisabled()
    await expect(onlyThese).toHaveAttribute('title', /Horizontal view/)
    await expect(modes(page).getByRole('button', { name: 'Highlight' })).toHaveAttribute('aria-pressed', 'true')
    // Until the history has been counted, it says how many and no more.
    await expect(strip(page).getByRole('status')).toHaveText('11 commits')
  })
})

test.describe('filtering the graph from the sidebar', () => {
  const value = (label: string, kind: string, commits: number) => ({ id: `${kind}:${label}`, label, kind, commits })
  const key = (label: string, kind: string, commits: number) => ({ id: label.toLowerCase(), label, spellings: [{ text: label, commits }], kind, commits, lines: commits, values: 1, held: '', known: true })
  const panel = {
    keys: [{ key: key('Changelog', 'label', 11), top: [value('fixed', 'label', 7), value('added', 'label', 4)], more: 0 }],
    moreKeys: 0,
    held: [],
    matches: 0,
    status: { reading: false, read: 212, total: 212, done: true, commits: 140 },
  }
  const none = { picks: [], only: true, kept: 0, total: 212 }
  const on = { picks: [fixed], only: true, kept: 7, total: 212 }
  const responses = {
    Refs: [branch('main', { head: true })],
    ShowingHidden: false,
    Worktrees: [],
    CommitSignatures: [],
    CommitNotes: [],
    WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
    TrailerPanel: panel,
    TrailerFilter: none,
    SetTrailerFilter: on,
    TrailerFilterRows: { rows: [1, 2], total: 2 },
  }
  const respond = (page: Page, r: object) => page.evaluate((x) => (window as unknown as { __respond: (r: object) => void }).__respond(x), r)

  test('a pick puts the strip above the graph, and Clear takes the filter off', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses })
    await expect(strip(page)).toHaveCount(0)
    await page.getByRole('navigation', { name: 'Sidebar panels' }).getByRole('button', { name: /^Smart filters/ }).click()
    await page.getByRole('tree', { name: 'Smart filters' }).getByRole('treeitem').filter({ hasText: 'fixed' }).click()

    // Only these is the mode until Highlight is chosen.
    await expect.poll(() => calls(page, 'SetTrailerFilter')).toEqual([['/r', [{ key: 'changelog', value: 'label:fixed' }], true]])
    await expect(strip(page).getByRole('status')).toHaveText('7 of 212 commits')
    await expect(strip(page).locator('.chip')).toHaveText(/Changelog\s*fixed/)
    await expect(page.getByRole('tree', { name: 'Smart filters' }).getByRole('treeitem').filter({ hasText: 'fixed' })).toHaveAttribute('aria-selected', 'true')
    // Under Only these nothing is highlighted: the graph holds only the matches.
    expect(await calls(page, 'TrailerFilterRows')).toEqual([])

    // Highlight asks again, with the mode, and for the rows that keep their ink.
    await respond(page, { SetTrailerFilter: { ...on, only: false } })
    await modes(page).getByRole('button', { name: 'Highlight' }).click()
    await expect.poll(async () => (await calls(page, 'SetTrailerFilter')).at(-1)).toEqual(['/r', [{ key: 'changelog', value: 'label:fixed' }], false])
    await expect.poll(() => calls(page, 'TrailerFilterRows')).toEqual([['/r']])
    await expect(modes(page).getByRole('button', { name: 'Highlight' })).toHaveAttribute('aria-pressed', 'true')
    // The mode is remembered for the repository.
    expect(await page.evaluate(() => localStorage.getItem('gitrevenge.filterMode:/r'))).toBe('highlight')

    await respond(page, { SetTrailerFilter: none })
    await strip(page).getByRole('button', { name: 'Clear' }).click()
    await expect.poll(async () => (await calls(page, 'SetTrailerFilter')).at(-1)).toEqual(['/r', [], false])
    await expect(strip(page)).toHaveCount(0)
  })

  test('the step buttons go to the row the backend gives, from the selected one', async ({ page }) => {
    const highlighting = { picks: [fixed], only: false, kept: 2, total: 212 }
    await page.addInitScript(() => localStorage.setItem('gitrevenge.filterMode:/r', 'highlight'))
    await open(page, 'repoView', { graphRows: 6, responses: { ...responses, TrailerFilter: highlighting, SetTrailerFilter: highlighting, TrailerFilterStep: 4 } })
    await expect(modes(page).getByRole('button', { name: 'Highlight' })).toHaveAttribute('aria-pressed', 'true')
    await strip(page).getByRole('button', { name: 'Next match' }).click()
    await expect.poll(async () => (await calls(page, 'TrailerFilterStep')).length).toBe(1)
    const [asked] = await calls(page, 'TrailerFilterStep')
    expect(asked[0]).toBe('/r')
    expect(asked[2]).toBe(1)
    // Row 4 is selected: the commit panel asks about its commit.
    const asked4 = (id: string) => async () => (await calls(page, 'CommitDetail')).some((c) => c[1] === id)
    await expect.poll(asked4('4'.padStart(40, '0'))).toBe(true)
    await respond(page, { TrailerFilterStep: 1 })
    await strip(page).getByRole('button', { name: 'Previous match' }).click()
    await expect.poll(async () => (await calls(page, 'TrailerFilterStep')).at(-1)).toEqual(['/r', 4, -1])
    await expect.poll(asked4('1'.padStart(40, '0'))).toBe(true)
  })

  test('a tab that had a filter shows it again', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses: { ...responses, TrailerFilter: on } })
    await expect(strip(page).locator('.chip')).toHaveText(/Changelog\s*fixed/)
    await expect(modes(page).getByRole('button', { name: 'Only these' })).toHaveAttribute('aria-pressed', 'true')
  })
})

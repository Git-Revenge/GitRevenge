import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open } from './support'

// The history search on the graph (#982): the commits that add or remove a text, as a chip on
// the smart filter's strip. The backend (#981) is stubbed: its state comes back from each call
// and from history:progress events, sent here by hand.

const opts = (o: object = {}) => ({ text: 'km/h', matchCase: false, regex: false, lines: false, paths: [] as string[], since: 0, ...o })
const searching = (o: object = {}) => ({ options: opts(), found: 0, searched: 0, total: 1_511, backTo: 0, running: false, stopped: false, ms: 0, estimateMs: 0, problem: '', ...o })
const filtered = (h: ReturnType<typeof searching>, o: object = {}) => ({ picks: [], only: true, kept: h.found, total: 1_511, history: h, ...o })
const id = (n: number) => String(n).padStart(40, '0')
// Nov 2, 2024: "late 2024", from any day this test is likely to run.
const LATE_2024 = Math.floor(new Date(2024, 10, 2).getTime() / 1000)

const strip = (page: Page) => page.getByRole('group', { name: 'Smart filter' })
const chip = (page: Page) => strip(page).getByRole('button', { name: 'Commits that add or remove km/h, options' })
const dialog = (page: Page) => page.getByRole('dialog', { name: 'Change the history search' })

test.describe('the history search’s chip', () => {
  const show = (page: Page, h: ReturnType<typeof searching>) => open(page, 'filterStrip', { filterStrip: { filter: filtered(h) } })

  test('says how the search is getting on, and stops it', async ({ page }) => {
    await show(page, searching({ running: true, found: 83, backTo: LATE_2024 }))
    await expect(chip(page)).toHaveText(/adds or removes\s*km\/h/)
    await expect(strip(page).getByRole('status')).toHaveText('83 of 1,511 commits')
    await expect(strip(page).getByText('Searching… 83 found, back to late 2024.')).toBeVisible()
    await strip(page).getByRole('button', { name: 'Stop' }).click()
    expect(await events(page, 'history-stop')).toHaveLength(1)
    await strip(page).getByRole('button', { name: 'Remove the search for commits that add or remove km/h' }).click()
    expect(await events(page, 'history-remove')).toHaveLength(1)
  })

  test('stopped, it carries on; done, it says what it searched; ended early, it says why', async ({ page }) => {
    await show(page, searching({ stopped: true, found: 83, backTo: LATE_2024 }))
    await expect(strip(page).getByText('Stopped at late 2024: 83 found. Older commits weren’t searched.')).toBeVisible()
    await expect(strip(page).getByRole('button', { name: 'Stop' })).toHaveCount(0)
    await strip(page).getByRole('button', { name: 'Carry on' }).click()
    expect(await events(page, 'history-carry-on')).toHaveLength(1)

    await show(page, searching({ found: 7, searched: 1_511, ms: 200 }))
    await expect(strip(page).getByText('Searched the whole history · 0.2 s')).toBeVisible()
    await expect(strip(page).getByRole('button', { name: 'Carry on' })).toHaveCount(0)

    await show(page, searching({ problem: 'Not a regular expression: its ( and ) don’t pair up.' }))
    await expect(strip(page).getByText('Not a regular expression: its ( and ) don’t pair up.')).toBeVisible()
  })

  test('offers to narrow a search of a big history with long to go', async ({ page }) => {
    await show(page, searching({ running: true, found: 83, total: 1_507_311, ms: 6_000, estimateMs: 20 * 60_000 }))
    const offer = strip(page).getByRole('note')
    await expect(offer).toContainText('All 1.5 million commits will take about 20 minutes. Narrow it?')
    await offer.getByRole('button', { name: 'The last year' }).click()
    const [asked] = (await events(page, 'history-search'))[0] as [ReturnType<typeof opts>]
    expect(asked).toMatchObject({ text: 'km/h', lines: false, paths: [] })
    // A year back from now, give or take the test's own minutes.
    expect(Math.abs(asked.since - (Date.now() / 1000 - 365 * 86_400))).toBeLessThan(600)

    // A folder is typed into the chip's options, which open on Only in.
    await offer.getByRole('button', { name: 'Only in a folder…' }).click()
    await expect(dialog(page).getByRole('textbox', { name: 'Only in' })).toBeFocused()
  })

  test('its options change how it matches, where and how far back', async ({ page }) => {
    await show(page, searching({ found: 7, searched: 1_511, ms: 200 }))
    await chip(page).click()
    await expect(dialog(page)).toBeVisible()
    await expect(dialog(page).getByRole('textbox', { name: 'Text' })).toHaveValue('km/h')
    // The way it searched has its count now it's done; the other hasn't been searched.
    await expect(dialog(page).getByRole('radio', { name: /appears or goes away/ })).toBeChecked()
    await expect(dialog(page).getByText('-S · 7 commits')).toBeVisible()
    await expect(dialog(page).getByText('-G', { exact: true })).toBeVisible()

    await dialog(page).getByRole('radio', { name: /a line with km\/h changes/ }).check()
    await dialog(page).getByRole('checkbox', { name: 'Match case' }).check()
    await dialog(page).getByRole('textbox', { name: 'Only in' }).fill('src/, docs/')
    await dialog(page).getByRole('combobox', { name: 'As far back as' }).selectOption('date')
    // Without a date there's nothing to search back to.
    await expect(dialog(page).getByRole('button', { name: 'Search again' })).toBeDisabled()
    await dialog(page).getByLabel('From').fill('2025-01-01')
    await dialog(page).getByRole('button', { name: 'Search again' }).click()
    await expect(dialog(page)).toHaveCount(0)
    const [asked] = (await events(page, 'history-search'))[0] as [ReturnType<typeof opts>]
    expect(asked).toEqual({ ...opts({ lines: true, matchCase: true, paths: ['src/', 'docs/'] }), since: Math.floor(new Date(2025, 0, 1).getTime() / 1000) })
    await expect(chip(page)).toBeFocused()

    // Escape closes it as it was; Remove takes the search out.
    await chip(page).click()
    await page.keyboard.press('Escape')
    await expect(dialog(page)).toHaveCount(0)
    await expect(chip(page)).toBeFocused()
    await chip(page).click()
    await dialog(page).getByRole('button', { name: 'Remove' }).click()
    expect(await events(page, 'history-remove')).toHaveLength(1)
  })

  test('sits beside a trailer’s chip', async ({ page }) => {
    const fixed = { key: 'changelog', value: 'label:fixed', keyLabel: 'Changelog', valueLabel: 'fixed', kind: 'label' }
    await open(page, 'filterStrip', { filterStrip: { filter: filtered(searching({ found: 7, searched: 1_511, ms: 200 }), { picks: [fixed], kept: 2 }) } })
    await expect(chip(page)).toBeVisible()
    await expect(strip(page).locator('.chip').filter({ hasText: 'fixed' })).toBeVisible()
    await expect(strip(page).getByRole('status')).toHaveText('2 of 1,511 commits')
  })
})

test.describe('the search box’s row', () => {
  const graph = { graphRows: 30, finds: true, histories: true, responses: { Refs: [], CommitSignatures: [], SearchGraph: { rows: [], total: 0 }, FindCount: { lines: 2, files: 2, superseded: false } } }
  const box = (page: Page) => page.getByRole('combobox', { name: 'Search commits' })
  const options = (page: Page) => page.getByRole('listbox', { name: 'Search suggestions' }).getByRole('option')

  test('comes after Find’s, and hands the text over', async ({ page }) => {
    await open(page, 'graph', graph)
    await box(page).fill('km/h')
    await expect(options(page)).toHaveCount(3)
    await expect(options(page).nth(2)).toHaveText(/Commits\s*that add or remove “km\/h”/)
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('Enter')
    await expect.poll(() => events(page, 'history-search')).toEqual([['km/h']])
    // The box empties: the text is on the chip now.
    await expect(box(page)).toHaveValue('')
  })
})

test.describe('the marks beside the commits it found', () => {
  test('are asked for the rows in view, a few at a time', async ({ page }) => {
    await open(page, 'graph', {
      graphRows: 4,
      sparse: true,
      graphHistory: { key: 'km/h', options: opts() },
      responses: { Refs: [], CommitSignatures: [], HistoryChanges: { [id(1)]: [{ file: 'public/map.js', added: 1, removed: 0 }] } },
    })
    // Each commit once, however many frames it takes to draw them all.
    const asked = async () => (await calls(page, 'HistoryChanges')).flatMap((c) => c[1] as string[])
    await expect.poll(async () => [...new Set(await asked())].sort()).toEqual([id(0), id(1), id(2), id(3)])
    expect((await asked()).length).toBe(4)
    expect((await calls(page, 'HistoryChanges'))[0][0]).toBe('/repo')
  })
})

test.describe('on the graph', () => {
  const none = { picks: [], only: true, kept: 0, total: 1_511 }
  const person = { name: 'Emily Carter', email: 'emily@tidewater.example', time: 1793500000 }
  const detail = { commit: { id: id(1), subject: 'Plot each station as a pin', body: '', trailers: [], parents: [id(2)], author: person, committer: person }, files: [{ status: 'M', path: 'public/map.js', added: 4, deleted: 1 }, { status: 'M', path: 'public/style.css', added: 2, deleted: 0 }] }
  const responses = {
    Refs: [branch('main', { head: true })],
    ShowingHidden: false,
    Worktrees: [],
    CommitSignatures: [],
    CommitNotes: [],
    WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
    TrailerFilter: none,
    SearchGraph: { rows: [], total: 0 },
    FindCount: { lines: 2, files: 2, superseded: false },
    SetHistorySearch: filtered(searching({ running: true })),
  }
  const respond = (page: Page, r: object) => page.evaluate((x) => (window as unknown as { __respond: (r: object) => void }).__respond(x), r)
  /** Types in the graph's search box, which the repository's bar folds into its button. */
  async function search(page: Page, text: string) {
    await page.getByRole('button', { name: 'Search commits' }).click()
    await page.getByRole('combobox', { name: 'Search commits' }).fill(text)
  }
  const emit = (page: Page, name: string, data: object) => page.evaluate(([n, d]) => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit(n as string, d), [name, data] as const)

  test('the search box starts one, and the strip follows it as it goes', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses })
    await expect(strip(page)).toHaveCount(0)
    await search(page, 'km/h')
    await page.getByRole('option', { name: /Commits\s*that add or remove “km\/h”/ }).click()
    await expect.poll(() => calls(page, 'SetHistorySearch')).toEqual([['/r', opts()]])
    await expect(chip(page)).toBeVisible()
    await expect(strip(page).getByText('Searching… 0 found.')).toBeVisible()

    // The backend says how it's getting on, at most once a second.
    await emit(page, 'history:progress', { path: '/r', filter: filtered(searching({ running: true, found: 2, searched: 2, backTo: LATE_2024 })) })
    await expect(strip(page).getByText('Searching… 2 found, back to late 2024.')).toBeVisible()
    await emit(page, 'history:progress', { path: '/r', filter: filtered(searching({ found: 2, searched: 3, ms: 240 })) })
    await expect(strip(page).getByText('Searched the whole history · 0.2 s')).toBeVisible()
    await expect(strip(page).getByRole('status')).toHaveText('2 of 1,511 commits')
    // Another repository's search is its own.
    await emit(page, 'history:progress', { path: '/other', filter: none })
    await expect(chip(page)).toBeVisible()
  })

  test('Stop, Carry on and Clear go to the backend', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses: { ...responses, TrailerFilter: filtered(searching({ running: true, found: 1 })), StopHistorySearch: filtered(searching({ stopped: true, found: 1, backTo: LATE_2024 })) } })
    await strip(page).getByRole('button', { name: 'Stop' }).click()
    await expect.poll(() => calls(page, 'StopHistorySearch')).toEqual([['/r']])
    await respond(page, { CarryOnHistorySearch: filtered(searching({ running: true, found: 1 })) })
    await strip(page).getByRole('button', { name: 'Carry on' }).click()
    await expect.poll(() => calls(page, 'CarryOnHistorySearch')).toEqual([['/r']])

    await respond(page, { SetHistorySearch: none })
    await strip(page).getByRole('button', { name: 'Clear' }).click()
    await expect.poll(async () => (await calls(page, 'SetHistorySearch')).at(-1)).toEqual(['/r', opts({ text: '' })])
    await expect(strip(page)).toHaveCount(0)
  })

  test('a search started under Highlight stays in Highlight', async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem('gitrevenge.filterMode:/r', 'highlight'))
    const highlighting = filtered(searching({ running: true }), { only: false })
    await open(page, 'repoView', { graphRows: 3, responses: { ...responses, SetTrailerFilter: highlighting, TrailerFilterRows: { rows: [], total: 0 } } })
    await search(page, 'km/h')
    await page.getByRole('option', { name: /Commits\s*that add or remove “km\/h”/ }).click()
    // The backend starts a search as Only these; the mode chosen here goes after it.
    await expect.poll(() => calls(page, 'SetTrailerFilter')).toEqual([['/r', [], false]])
    await expect(strip(page).getByRole('group', { name: 'What a filter does to other commits' }).getByRole('button', { name: 'Highlight' })).toHaveAttribute('aria-pressed', 'true')
  })

  test('a commit asked about before the search found it is asked about again once it has found more', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses: { ...responses, TrailerFilter: filtered(searching({ running: true })), HistoryChanges: {} } })
    // Nothing comes back for them yet: the search hasn't found them.
    const askedAbout = async (id: string) => (await calls(page, 'HistoryChanges')).filter((c) => (c[1] as string[]).includes(id)).length
    await expect.poll(() => askedAbout(id(2))).toBe(1)
    await emit(page, 'history:progress', { path: '/r', filter: filtered(searching({ running: true, found: 2, searched: 3 })) })
    await expect.poll(() => askedAbout(id(2))).toBe(2)
  })

  test('the commit panel leads with the files where the text changed', async ({ page }) => {
    await open(page, 'repoView', {
      graphRows: 3,
      responses: {
        ...responses,
        TrailerFilter: filtered(searching({ found: 2, searched: 3, ms: 200 })),
        CommitDetail: detail,
        // Whichever commit is selected first: they all brought it in.
        HistoryChanges: Object.fromEntries([0, 1, 2].map((n) => [id(n), [{ file: 'public/map.js', added: 1, removed: 0 }]])),
      },
    })
    const details = page.getByRole('complementary', { name: 'Commit details' })
    await details.getByRole('tab', { name: 'Files' }).click()
    const changed = details.getByRole('list', { name: 'Where km/h changed' })
    await expect(changed.getByRole('button')).toHaveCount(1)
    await expect(changed.getByRole('button')).toHaveText(/map\.js\s*\+1: brings it in/)
    await changed.getByRole('button').click()
    await expect.poll(async () => (await calls(page, 'FileDiff')).some((c) => c.includes('public/map.js'))).toBe(true)
  })
})

test.describe('from Find in files', () => {
  async function openFind(page: Page, found: { lines: number; files: number }) {
    await page.addInitScript(() => localStorage.clear())
    await open(page, 'sidebar', { responses: { Refs: [], FindInFiles: { commit: '', truncated: false, ms: 200, superseded: false, ...found } } })
    await page.getByRole('button', { name: 'Find in files', exact: true }).click()
  }

  test('nothing found asks when it was in the history', async ({ page }) => {
    await openFind(page, { lines: 0, files: 0 })
    await page.getByRole('textbox', { name: 'Text to find' }).fill('km/s')
    await page.getByRole('button', { name: 'Match case' }).click()
    await page.getByRole('textbox', { name: 'Only these files' }).fill('src/')
    await page.getByRole('button', { name: 'When was it in the history?' }).click()
    await expect.poll(() => events(page, 'history-search')).toEqual([[opts({ text: 'km/s', matchCase: true, paths: ['src/'] })]])
  })

  test('a file’s menu asks when it gained or lost the text', async ({ page }) => {
    await openFind(page, { lines: 1, files: 1 })
    await page.getByRole('textbox', { name: 'Text to find' }).fill('km/h')
    await expect.poll(async () => (await calls(page, 'FindInFiles')).length).toBe(1)
    const token = ((await calls(page, 'FindInFiles'))[0] as unknown[])[1]
    const found = { file: 'public/map.js', line: 6, text: 'pin.title = km/h', cutStart: false, cutEnd: false, matches: [{ from: 12, to: 16 }] }
    await page.evaluate((b) => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('find:lines', b), { path: '/repo', token, commit: '', lines: [found] })
    await page.getByRole('listbox', { name: 'Lines found' }).getByRole('option').first().click({ button: 'right' })
    await expect.poll(async () => ((await events(page, 'menu')).at(-1)?.[0] as { label: string }[] | undefined)?.map((i) => i.label)).toEqual(['History', 'Commits that add or remove “km/h” here', 'Blame', 'Copy the path'])
  })
})

import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Find in files (#980): the sidebar's panel, the file it opens at a line, and the search
// box's row. The backend is stubbed: lines arrive as find:lines events, sent here by hand
// with the token the panel started the search with.

type Line = { file: string; line: number; text: string; cutStart?: boolean; cutEnd?: boolean; matches: { from: number; to: number }[] }
const line = (file: string, n: number, text: string, word = 'km/h'): Line => {
  const at = text.indexOf(word)
  return { file, line: n, text, cutStart: false, cutEnd: false, matches: at < 0 ? [] : [{ from: at, to: at + word.length }] }
}
const MAP = line('public/map.js', 6, 'pin.title = `${s.name}: ${s.wind} km/h`')
const NOTES = line('notebooks/wind.ipynb', 10, 'The alert limit is $v > 60$ km/h.')
const done = (over: object = {}) => ({ commit: '', lines: 2, files: 2, truncated: false, ms: 200, superseded: false, ...over })

const panel = (page: Page) => page.getByRole('listbox', { name: 'Lines found' })
const rows = (page: Page) => panel(page).getByRole('option')
const findBox = (page: Page) => page.getByRole('textbox', { name: 'Text to find' })

/** The last call to method's argument at index, or undefined until there's a call. */
const lastArg = async (page: Page, method: string, index: number) => ((await calls(page, method)).at(-1) as unknown[] | undefined)?.[index]
/** The last event of that name's first value, or undefined until there's one. */
const lastEvent = async (page: Page, name: string) => ((await events(page, name)).at(-1) as unknown[] | undefined)?.[0]
const menuLabels = async (page: Page) => ((await lastEvent(page, 'menu')) as { label: string }[] | undefined)?.map((i) => i.label)

/** Sends lines as the backend does, for the search the panel last started. */
async function send(page: Page, lines: Line[], commit = '') {
  const token = await lastArg(page, 'FindInFiles', 1)
  await page.evaluate((b) => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('find:lines', b), { path: '/repo', token, commit, lines })
}

async function openFind(page: Page, scenario: { responses?: object } & Record<string, unknown> = {}) {
  // The sidebar remembers its panel: start from Branches each time, so the rail opens Find.
  await page.addInitScript(() => localStorage.clear())
  await open(page, 'sidebar', { delays: { FindInFiles: 600 }, ...scenario, responses: { Refs: [], FindInFiles: done(), ...(scenario.responses ?? {}) } })
  await page.getByRole('button', { name: 'Find in files', exact: true }).click()
}

test.describe('the Find panel', () => {
  test('searches as you type, and lists the lines by file with the matches marked', async ({ page }) => {
    await openFind(page)
    // The sidebar's own filter doesn't reach this panel: it has its own box.
    await expect(page.getByRole('searchbox', { name: 'Filter everything in the sidebar' })).toBeHidden()
    await findBox(page).fill('km/h')
    await expect.poll(async () => (await calls(page, 'FindInFiles')).length).toBe(1)
    const [path, , options] = (await calls(page, 'FindInFiles'))[0] as [string, string, Record<string, unknown>]
    expect(path).toBe('/repo')
    expect(options).toEqual({ text: 'km/h', matchCase: false, wholeWord: false, regex: false, untracked: false, rev: '', paths: [] })

    await send(page, [MAP, NOTES])
    await expect(rows(page)).toHaveCount(4)
    await expect(rows(page).nth(0)).toHaveAccessibleName('public/map.js, 1 line')
    await expect(rows(page).nth(0).locator('.folder')).toHaveText('public/')
    await expect(rows(page).nth(1).locator('.num')).toHaveText('6')
    await expect(rows(page).nth(1).locator('mark')).toHaveText('km/h')
    await expect(page.getByText('2 lines in 2 files · 0.2 s')).toBeVisible()

    await rows(page).nth(3).click()
    await expect.poll(() => events(page, 'open-file')).toEqual([[{ mode: 'found', file: 'notebooks/wind.ipynb', line: 10, find: options }]])
  })

  test('passes on its options: case, whole word, regex, where to look, and which files', async ({ page }) => {
    await openFind(page)
    await findBox(page).fill('wind_(kmh|knots)')
    await page.getByRole('button', { name: 'Match case' }).click()
    await page.getByRole('button', { name: 'Whole word' }).click()
    await page.getByRole('button', { name: 'Regular expression' }).click()
    await page.getByRole('checkbox', { name: 'untracked files too' }).check()
    await page.getByRole('textbox', { name: 'Only these files' }).fill('src/, !*.min.js')
    await expect.poll(() => lastArg(page, 'FindInFiles', 2)).toEqual({ text: 'wind_(kmh|knots)', matchCase: true, wholeWord: true, regex: true, untracked: true, rev: '', paths: ['src/', '!*.min.js'] })

    // A commit has no untracked files: the option goes with the working tree.
    await page.getByRole('combobox', { name: 'Where to look' }).selectOption({ label: 'A branch, tag or commit…' })
    await expect(page.getByRole('checkbox', { name: 'untracked files too' })).toHaveCount(0)
    await page.getByRole('textbox', { name: 'Branch, tag or commit to search' }).fill('old')
    await expect.poll(() => lastArg(page, 'FindInFiles', 2)).toMatchObject({ rev: 'old', untracked: false })
  })

  test("drops a stopped search's lines, and opens a commit's file at that commit", async ({ page }) => {
    await openFind(page, { responses: { FindInFiles: done({ commit: 'c'.repeat(40), lines: 1, files: 1 }) } })
    await page.getByRole('combobox', { name: 'Where to look' }).selectOption({ label: 'HEAD' })
    await findBox(page).fill('km/h')
    await expect.poll(async () => (await calls(page, 'FindInFiles')).length).toBe(1)
    // A batch from another search is no part of this one.
    await page.evaluate(() => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('find:lines', { path: '/repo', token: 'an-old-one', commit: '', lines: [{ file: 'stale.txt', line: 1, text: 'km/h', matches: [] }] }))
    await send(page, [MAP], 'c'.repeat(40))
    await expect(rows(page)).toHaveCount(2)
    await rows(page).nth(1).click()
    await expect.poll(() => lastEvent(page, 'open-file')).toMatchObject({ mode: 'found', file: 'public/map.js', line: 6, commit: 'c'.repeat(40) })
  })

  test('says when nothing was found, when it stopped at its limit, and what git could not read', async ({ page }) => {
    await openFind(page, { responses: { FindInFiles: done({ lines: 0, files: 0 }) } })
    await findBox(page).fill('km/s')
    await expect(page.getByText('“km/s” isn\'t in any file of the working tree.')).toBeVisible()

    await openFind(page, { responses: { FindInFiles: done({ lines: 10000, files: 569, truncated: true }) } })
    await findBox(page).fill('the')
    await expect(page.getByText(`The first ${(10000).toLocaleString()} lines. Narrow it with Files, Whole word or Match case.`)).toBeVisible()

    await openFind(page, { failures: { FindInFiles: "Not a regular expression: its ( and ) don't pair up. Turn off Regular expression to search for it as written." } })
    await page.getByRole('button', { name: 'Regular expression' }).click()
    await findBox(page).fill('kmalloc_array(')
    await expect(page.getByRole('alert')).toHaveText(/its \( and \) don't pair up/)
  })

  test('the keys move through the lines and open one, and a menu offers blame and history', async ({ page }) => {
    await openFind(page)
    await findBox(page).fill('km/h')
    await expect.poll(async () => (await calls(page, 'FindInFiles')).length).toBe(1)
    await send(page, [MAP, NOTES])
    await panel(page).focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await expect(rows(page).nth(3)).toHaveAttribute('aria-selected', 'true')
    await page.keyboard.press('Enter')
    await expect.poll(() => lastEvent(page, 'open-file')).toMatchObject({ file: 'notebooks/wind.ipynb', line: 10 })

    await rows(page).nth(0).click({ button: 'right' })
    // With a history search to start (#982): when this file gained or lost the text.
    await expect.poll(() => menuLabels(page)).toEqual(['History', 'Commits that add or remove “km/h” here', 'Blame', 'Copy the path'])
    await rows(page).nth(1).click({ button: 'right' })
    await expect.poll(() => menuLabels(page)).toEqual(['Blame at this line'])
  })

  test('Ctrl+Alt+F opens it with the text selected, and Alt+↓ goes to the next file found', async ({ page }) => {
    await open(page, 'sidebar', { commandKeys: true, delays: { FindInFiles: 600 }, diffShown: { mode: 'found', file: 'public/map.js', line: 6 }, responses: { Refs: [], FindInFiles: done() } })
    const filter = page.getByRole('searchbox', { name: 'Filter everything in the sidebar' })
    await filter.fill('km/h')
    await filter.selectText()
    await page.keyboard.press('Control+Alt+F')
    // The panel loads the first time it's shown, which takes a while beside a full run.
    await expect(findBox(page)).toHaveValue('km/h', { timeout: 15000 })
    await expect.poll(async () => (await calls(page, 'FindInFiles')).length).toBe(1)
    await send(page, [MAP, NOTES])
    await expect(rows(page)).toHaveCount(4)
    // From the results: in the box, Alt+↓ is left to the text.
    await panel(page).focus()
    await page.keyboard.press('Alt+ArrowDown')
    await expect.poll(() => lastEvent(page, 'open-file')).toMatchObject({ file: 'notebooks/wind.ipynb', line: 10 })
  })
})

const FEED = ['// Wind in km/h.', 'const a = 1', 'export const unit = "KM/H"', 'const b = 2', 'const c = 3', 'pin.title = `${s.wind} km/h`', 'const d = 4', 'const e = 5', 'speed(km/h)'].join('\n') + '\n'
const contents = (text: string, kind = 'text') => ({ path: 'src/feed.js', old: { exists: true, text: '', size: 0 }, new: { exists: true, text, size: text.length }, kind })
const target = { mode: 'found', file: 'src/feed.js', line: 6, find: { text: 'km/h', matchCase: false, wholeWord: false, regex: false, untracked: false, rev: '', paths: [] } }

test.describe('a file Find in files opened', () => {
  test('marks every match, starts at the line clicked, and F7 steps through them', async ({ page }) => {
    await open(page, 'foundFile', { commandKeys: true, foundTarget: target, responses: { FileContents: contents(FEED) } })
    await expect(page.locator('.cm-content .found')).toHaveCount(4)
    await expect(page.getByText('3 of 4')).toBeVisible()
    await expect(page.locator('.found.current')).toHaveText('km/h')
    expect(await calls(page, 'FileContents')).toEqual([['/repo', 'unstaged', '', 'src/feed.js', '', false]])
    await expect(page.getByText('working tree')).toBeVisible()

    await page.keyboard.press('F7')
    await expect(page.getByText('4 of 4')).toBeVisible()
    await page.keyboard.press('F7')
    await expect(page.getByText('1 of 4')).toBeVisible()
    await page.keyboard.press('Shift+F7')
    await expect(page.getByText('4 of 4')).toBeVisible()

    await page.getByRole('button', { name: 'Blame' }).click()
    await expect.poll(() => events(page, 'open')).toEqual([[{ mode: 'blame', file: 'src/feed.js', line: 9 }]])
  })

  test("reads a commit's version, and asks before loading a large one", async ({ page }) => {
    const sha = 'c'.repeat(40)
    await open(page, 'foundFile', { foundTarget: { ...target, commit: sha }, responses: { FileContents: contents('', 'large') } })
    await expect(page.getByText(`at ${sha.slice(0, 7)}`)).toBeVisible()
    await page.getByRole('button', { name: 'Load anyway' }).click()
    await expect.poll(() => calls(page, 'FileContents')).toEqual([
      ['/repo', 'commit', sha, 'src/feed.js', '', false],
      ['/repo', 'commit', sha, 'src/feed.js', '', true],
    ])
  })
})

test.describe("the search box's row", () => {
  const graph = { graphRows: 30, finds: true, responses: { Refs: [], CommitSignatures: [], SearchGraph: { rows: [], total: 0 }, FindCount: { lines: 2, files: 2, superseded: false } } }
  const box = (page: Page) => page.getByRole('combobox', { name: 'Search commits' })
  const options = (page: Page) => page.getByRole('listbox', { name: 'Search suggestions' }).getByRole('option')

  test('offers Find in files second, with its count, and Enter on it hands the text over', async ({ page }) => {
    await open(page, 'graph', graph)
    await box(page).fill('km/h')
    await expect(options(page)).toHaveCount(2)
    await expect(options(page).nth(1)).toHaveText(/Find\s*“km\/h” in the files\s*2 lines in 2 files/)
    expect(await lastArg(page, 'FindCount', 1)).toMatchObject({ text: 'km/h', rev: '' })
    // Enter still searches the commits until an arrow reaches the row.
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('Enter')
    await expect.poll(() => events(page, 'find-in-files')).toEqual([['km/h']])
  })
})

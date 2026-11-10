import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// #597: the conflict tool. The backend is stubbed; these check what the tool shows for a
// conflicted file and what it saves.

const lines = (...l: string[]) => l.join('\n') + '\n'
const ours = lines('const FEED = 1', '// Wind in knots.', 'export function read(id) {', '  return wind / 1.852', '}')
const theirs = lines('const FEED = 1', '// Wind in metres per second.', 'export function read(id) {', '  return wind / 3.6', '}')
const base = lines('const FEED = 1', 'export function read(id) {', '  return wind', '}')
const merged = lines('const FEED = 1', '<<<<<<< HEAD', '// Wind in knots.', '=======', '// Wind in metres per second.', '>>>>>>> feature/metric-units', 'export function read(id) {', '<<<<<<< HEAD', '  return wind / 1.852', '=======', '  return wind / 3.6', '>>>>>>> feature/metric-units', '}')
const diff3 = lines(
  'const FEED = 1',
  '<<<<<<< ours',
  '// Wind in knots.',
  '||||||| base',
  '=======',
  '// Wind in metres per second.',
  '>>>>>>> theirs',
  'export function read(id) {',
  '<<<<<<< ours',
  '  return wind / 1.852',
  '||||||| base',
  '  return wind',
  '=======',
  '  return wind / 3.6',
  '>>>>>>> theirs',
  '}',
)

const side = (text: string, exists = true) => ({ exists, text, size: text.length })
const file = (over: object = {}) => ({ path: 'src/feed.js', base: side(base), ours: side(ours), theirs: side(theirs), merged: side(merged), binary: false, diff3, ...over })
const merging = (conflicts: string[] = ['README.md', 'src/feed.js']) => ({ operation: 'merge', step: 0, total: 0, actions: ['continue', 'abort'], conflicts, ours: 'main', theirs: 'feature/metric-units', commit: 'c'.repeat(40), subject: 'Report wind in metres per second', origHead: 'd'.repeat(40), written: [] })
const scenario = (cf: object = file(), more: object = {}) => ({ responses: { ConflictFile: cf, RepoState: merging(), ConflictContext: { mergeTool: '', clean: ['src/units.js'] }, ConflictOrigin: { commit: '', author: '', subject: '' }, ...more } })

const result = (page: Page) => page.locator('.editor .cm-content')
const text = (page: Page) => page.evaluate(() => [...document.querySelectorAll('.editor .cm-line')].map((l) => l.textContent).join('\n'))
const place = (page: Page, n: number) => page.getByRole('group', { name: new RegExp(`^Place ${n} ·`) })
const mark = (page: Page) => page.getByRole('button', { name: 'Mark resolved' })
const saved = async (page: Page) => ((await calls(page, 'ResolveConflict'))[0] as [string, string, string])?.[2]

test('the result holds no conflict markers, and each place offers its own choices', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await expect(place(page, 1)).toBeVisible()
  await expect(place(page, 2)).toBeVisible()
  expect(await text(page)).toBe(['const FEED = 1', 'export function read(id) {', '}', ''].join('\n'))
  await expect(page.getByRole('status')).toHaveText('2 of 2 places left')
  // An undecided place shows what each side would put there.
  await expect(place(page, 1)).toContainText('// Wind in knots.')
  await expect(place(page, 1)).toContainText('// Wind in metres per second.')
  await expect(mark(page)).toHaveAttribute('aria-disabled', 'true')
  await expect(mark(page)).toHaveAttribute('title', '2 places are still undecided')
  await mark(page).click({ force: true })
  expect(await calls(page, 'ResolveConflict')).toEqual([])
})

test('taking a side fills the place, Start again empties it, and the result is saved clean', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await place(page, 1).getByRole('button', { name: 'Theirs' }).click()
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · took theirs')
  await expect(page.getByRole('status')).toHaveText('1 of 2 places left')
  expect(await text(page)).toContain('// Wind in metres per second.')

  await place(page, 1).getByRole('button', { name: 'Start again' }).click()
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · choose a side, or type here')
  expect(await text(page)).not.toContain('metres per second.\nexport')

  await place(page, 1).getByRole('button', { name: 'Both' }).click()
  await place(page, 2).getByRole('button', { name: 'Ours' }).click()
  await expect(page.getByRole('status')).toHaveText('All 2 places are decided')
  await expect(mark(page)).not.toHaveAttribute('aria-disabled', 'true')
  await mark(page).click()
  await expect.poll(() => saved(page)).toBe(lines('const FEED = 1', '// Wind in knots.', '// Wind in metres per second.', 'export function read(id) {', '  return wind / 1.852', '}'))
  // README.md is still conflicted, so the tool moves on to it.
  await expect.poll(() => events(page, 'open')).toEqual([['README.md']])
})

test('typing inside a place makes it edited, which counts as decided', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await place(page, 1).getByRole('button', { name: 'Ours' }).click()
  await place(page, 2).getByRole('button', { name: 'Theirs' }).click()
  await result(page).getByText('return wind / 3.6').click()
  await page.keyboard.press('End')
  await page.keyboard.type(' // by hand')
  await expect(place(page, 2)).toHaveAccessibleName("Place 2 · you're writing this one by hand")
  await expect(place(page, 2).getByRole('button', { name: 'Edited' })).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByRole('status')).toHaveText('All 2 places are decided')
  // Typing outside the places leaves them as they were.
  await result(page).getByText('const FEED = 1').click()
  await page.keyboard.press('End')
  await page.keyboard.type('00')
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · took ours')
  await mark(page).click()
  await expect.poll(() => saved(page)).toBe(lines('const FEED = 100', '// Wind in knots.', 'export function read(id) {', '  return wind / 3.6 // by hand', '}'))
})

test('typing at an undecided place writes it by hand', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await place(page, 1).click()
  await page.keyboard.type('// Wind in knots, to one place.')
  await page.keyboard.press('Enter')
  await expect(place(page, 1)).toHaveAccessibleName("Place 1 · you're writing this one by hand")
  await expect(page.getByRole('status')).toHaveText('1 of 2 places left')
})

test('the three views show the sides of the current place, with who last changed each', async ({ page }) => {
  await open(page, 'conflictView', {
    ...scenario(),
    responses: { ...scenario().responses, ConflictOrigin: { commit: 'a'.repeat(40), author: 'Maya Okafor', subject: 'Show wind in knots' } },
  })
  const panes = page.getByLabel('The versions of this place, read-only')
  // Ours and theirs to begin with.
  await expect(panes.locator('section')).toHaveCount(2)
  await expect(panes.locator('section.ours')).toContainText('main')
  await expect(panes.locator('section.theirs')).toContainText('feature/metric-units')
  await expect(panes.locator('section.ours .ln.own')).toHaveText(/Wind in knots/)
  await expect(panes.locator('section.ours')).toContainText('Maya Okafor · Show wind in knots')
  expect(await calls(page, 'ConflictOrigin')).toContainEqual(['/r', 'src/feed.js', 'ours', 2, 2])

  await page.getByRole('button', { name: 'Three-way' }).click()
  await expect(panes.locator('section')).toHaveCount(3)
  await expect(panes.locator('section.base')).toContainText('where both started')
  // Neither side's first place had an original line.
  await expect(panes.locator('section.base')).toContainText('nothing here on this side')

  await page.getByRole('button', { name: 'Result only' }).click()
  await expect(panes).toHaveCount(0)
})

test('F7 moves between places, and 1 and 2 take a side for the current one', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  const panes = page.getByLabel('The versions of this place, read-only')
  await expect(panes.locator('section.ours .ln.own')).toHaveText(/Wind in knots/)
  await page.getByRole('button', { name: /^Next/ }).click()
  await expect(panes.locator('section.ours .ln.own')).toHaveText(/1\.852/)
  await page.keyboard.press('Shift+F7')
  await expect(panes.locator('section.ours .ln.own')).toHaveText(/Wind in knots/)

  // Take 2 on the pane, then with the key from inside the editor.
  await panes.getByRole('button', { name: /^Take 2/ }).click()
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · took theirs')
  await page.keyboard.press('F7')
  await result(page).click()
  await page.keyboard.press('F7')
  await page.keyboard.press('Shift+F7')
  await page.keyboard.press('F7')
  await page.keyboard.press('Alt+1')
  await expect(place(page, 2)).toHaveAccessibleName('Place 2 · took ours')
})

test('the panel lists the files of the merge, whole-file choices and what merged cleanly', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  const panel = page.getByRole('complementary', { name: 'Files in this merge' })
  await expect(panel.getByRole('heading')).toHaveText('2 files to settle')
  await expect(panel.getByText('0 of 2 places')).toBeVisible()
  await expect(panel.getByText('Merged cleanly · 1')).toBeVisible()
  await expect(panel.getByText('src/units.js')).toBeVisible()
  await expect(panel.getByRole('button', { name: /^Open in/ })).toHaveCount(0)
  // Another conflicted file opens from the list.
  await panel.getByRole('button', { name: 'README.md' }).click()
  expect(await events(page, 'open')).toEqual([['README.md']])

  await panel.getByRole('button', { name: 'Take theirs' }).click()
  await expect.poll(() => calls(page, 'TakeConflictSide')).toEqual([['/r', 'src/feed.js', 'theirs']])
})

test('Start this file again puts every place back', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  const again = page.getByRole('button', { name: 'Start this file again' })
  await expect(again).toBeDisabled()
  await place(page, 1).getByRole('button', { name: 'Ours' }).click()
  await place(page, 2).getByRole('button', { name: 'Theirs' }).click()
  await again.click()
  await expect(page.getByRole('status')).toHaveText('2 of 2 places left')
  expect(await text(page)).toBe(['const FEED = 1', 'export function read(id) {', '}', ''].join('\n'))
})

test('offers your own merge tool when one is set, and leaves when it resolves the file', async ({ page }) => {
  await open(page, 'conflictView', scenario(file(), { ConflictContext: { mergeTool: 'kdiff3', clean: [] } }))
  const tool = page.getByRole('button', { name: 'Open in kdiff3' })
  await expect(tool).toBeVisible()
  await page.evaluate(() => (window as unknown as { __respond: (r: object) => void }).__respond({ RepoState: { operation: 'merge', conflicts: ['README.md'], actions: [], ours: 'main', theirs: 'x', commit: '', subject: '', origHead: '', written: [], step: 0, total: 0 } }))
  await tool.click()
  await expect.poll(() => calls(page, 'OpenMergeTool')).toEqual([['/r', 'src/feed.js']])
  await expect.poll(() => events(page, 'open')).toEqual([['README.md']])
})

test('a file one side deleted keeps the whole-file choices only', async ({ page }) => {
  await open(page, 'conflictView', scenario(file({ ours: side('', false), diff3: '', merged: side(theirs) })))
  await expect(page.getByText('Ours (main) deleted this file and theirs (feature/metric-units) changed it.')).toBeVisible()
  await expect(page.locator('.editor')).toHaveCount(0)
  await expect(mark(page)).toHaveCount(0)
  await page.getByRole('button', { name: 'Delete it, as ours did' }).click()
  await expect.poll(() => calls(page, 'TakeConflictSide')).toEqual([['/r', 'src/feed.js', 'ours']])
})

test('a binary file keeps the whole-file choices only', async ({ page }) => {
  await open(page, 'conflictView', scenario(file({ binary: true, diff3: '' })))
  await expect(page.getByText("This file is binary, so it can't be merged line by line.")).toBeVisible()
  await expect(page.getByRole('button', { name: 'Take ours' }).first()).toBeVisible()
})

test('without the original, three-way is off and the rest still works', async ({ page }) => {
  await open(page, 'conflictView', scenario(file({ diff3: '' })))
  await expect(page.getByRole('button', { name: 'Three-way' })).toBeDisabled()
  await place(page, 1).getByRole('button', { name: 'Ours' }).click()
  await expect(page.getByRole('status')).toHaveText('1 of 2 places left')
})

test('in a rebase, ours is what is being rebased onto', async ({ page }) => {
  const rebasing = { ...merging(['src/feed.js']), operation: 'rebase', ours: 'feature/metric-units', theirs: 'main', step: 2, total: 3 }
  await open(page, 'conflictView', scenario(file(), { RepoState: rebasing }))
  const panes = page.getByLabel('The versions of this place, read-only')
  await expect(panes.locator('section.ours .whose')).toHaveText('main')
  await expect(panes.locator('section.theirs .whose')).toHaveText('feature/metric-units')
  await expect(page.getByRole('complementary', { name: 'Files in this rebase' })).toBeVisible()
})

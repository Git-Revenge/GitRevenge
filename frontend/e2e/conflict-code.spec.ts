import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// #946: the conflict tool reads the code. The backend is stubbed: these check how the tool
// shows what reading each place as code found, and what it saves.

const lines = (...l: string[]) => l.join('\n') + '\n'
type Three = { ours: string; base: string; theirs: string }
const head: Three = { ours: 'func Forecast(ctx int, from int, n int) int {', base: 'func Forecast(from int, n int) int {', theirs: 'func Forecast(from int, n int, step int) int {' }
const clash: Three = { ours: 'const maxHours = 72', base: 'const maxHours = 48', theirs: 'const maxHours = 96' }
const notes: Three = { ours: 'const maxHours = 72 // two days', base: 'const maxHours = 48 // two days', theirs: 'const maxHours = 48 // four days' }

/** A Go file conflicted in two places, as git leaves it, with the stages and re-merge. */
function conflicted(second: Three) {
  const version = (pick: keyof Three) => lines('package tides', '', head[pick], '\treturn n', '}', '', second[pick])
  const place = (p: Three, base: boolean) => ['<<<<<<< ours', p.ours, ...(base ? ['||||||| base', p.base] : []), '=======', p.theirs, '>>>>>>> theirs']
  const merged = (base: boolean) => lines('package tides', '', ...place(head, base), '\treturn n', '}', '', ...place(second, base))
  const side = (text: string) => ({ exists: true, text, size: text.length })
  return { path: 'internal/tides/forecast.go', base: side(version('base')), ours: side(version('ours')), theirs: side(version('theirs')), merged: side(merged(false)), binary: false, diff3: merged(true) }
}

const combined = 'func Forecast(ctx int, from int, n int, step int) int {\n'
const suggestHead = {
  ok: true,
  text: combined,
  reason: 'ours added `ctx int,`; theirs added `, step int`',
  origins: [
    { from: 14, to: 22, side: 'ours' },
    { from: 38, to: 48, side: 'theirs' },
  ],
  ours: [{ from: 14, to: 22 }],
  theirs: [{ from: 29, to: 39 }],
}
const cantCombine = { ok: false, text: '', reason: 'both sides changed `48`', origins: [], ours: [{ from: 17, to: 19 }], theirs: [{ from: 17, to: 19 }] }
const suggestNotes = { ok: true, text: 'const maxHours = 72 // four days\n', reason: 'ours changed `48` to `72`; theirs changed `// two days` to `// four days`', origins: [], ours: [{ from: 17, to: 19 }], theirs: [{ from: 20, to: 32 }] }
const reading = (over: object = {}) => ({ language: 'Go', fallback: '', suggestions: [suggestHead, cantCombine], driver: '', driverCommand: '', ...over })

const merging = { operation: 'merge', step: 0, total: 0, actions: ['continue', 'abort'], conflicts: ['README.md', 'internal/tides/forecast.go'], ours: 'main', theirs: 'feature/tide-step', commit: 'c'.repeat(40), subject: 'Forecast readings every step', origHead: 'd'.repeat(40), written: [] }
const scenario = (more: object = {}, second: Three = clash) => ({
  diffFile: 'internal/tides/forecast.go',
  responses: { ConflictFile: conflicted(second), RepoState: merging, ConflictContext: { mergeTool: '', clean: [] }, ConflictOrigin: { commit: '', author: '', subject: '' }, ConflictSuggestions: reading(), ...more },
})

const text = (page: Page) => page.evaluate(() => [...document.querySelectorAll('.editor .cm-line')].map((l) => l.textContent).join('\n'))
const place = (page: Page, n: number) => page.getByRole('group', { name: new RegExp(`^Place ${n} ·`) })
const status = (page: Page) => page.getByRole('status')
const mark = (page: Page) => page.getByRole('button', { name: 'Mark resolved' })
const saved = async (page: Page) => ((await calls(page, 'ResolveConflict'))[0] as [string, string, string])?.[2]

test('a place whose sides combine is pre-filled and marked, but still counts as undecided', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · suggested: both sides combined')
  await expect(place(page, 1)).toContainText('ours added ctx int,; theirs added , step int')
  // The tokens it names are set as code, not between backticks.
  await expect(place(page, 1).locator('.place-why code')).toHaveText(['ctx int,', ', step int'])
  await expect.poll(() => text(page)).toContain(combined.trimEnd())
  // Each part of it is washed by the side it came from.
  await expect(page.locator('.editor .origin.ours')).toHaveText('ctx int,')
  await expect(page.locator('.editor .origin.theirs')).toHaveText(', step int')
  await expect(page.getByRole('button', { name: 'Read as Go' }).or(page.getByText('Read as Go'))).toBeVisible()
  // Nothing is kept on its own: Mark resolved waits, and says what for.
  await expect(status(page)).toHaveText('2 of 2 places left · 1 suggested')
  await expect(mark(page)).toHaveAttribute('title', '1 suggestion to keep, 1 place is still undecided')

  await place(page, 1).getByRole('button', { name: 'Keep the suggestion' }).click()
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · kept the suggestion')
  await expect(status(page)).toHaveText('1 of 2 places left')
})

test("a place that can't be combined says why, and can be written by hand from both sides", async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await expect(place(page, 2)).toHaveAccessibleName('Place 2 · choose a side, or type here')
  await expect(place(page, 2)).toContainText("can't be combined: both sides changed 48")
  await expect(place(page, 2).locator('.place-why code')).toHaveText('48')
  await place(page, 2).getByRole('button', { name: 'Write it myself' }).click()
  await expect(place(page, 2)).toHaveAccessibleName("Place 2 · you're writing this one by hand")
  await expect.poll(() => text(page)).toContain('const maxHours = 72\nconst maxHours = 96')
  // The cursor is in it, ready to edit.
  await page.keyboard.type(' // hours')
  await expect.poll(() => text(page)).toContain('const maxHours = 96 // hours')

  await place(page, 1).getByRole('button', { name: 'Keep the suggestion' }).click()
  await expect(status(page)).toHaveText('All 2 places are decided')
  await mark(page).click()
  await expect.poll(() => saved(page)).toBe(lines('package tides', '', combined.trimEnd(), '\treturn n', '}', '', 'const maxHours = 72', 'const maxHours = 96 // hours'))
})

test('Keep all keeps every waiting suggestion, and Start again puts one back', async ({ page }) => {
  await open(page, 'conflictView', scenario({ ConflictSuggestions: reading({ suggestions: [suggestHead, suggestNotes] }) }, notes))
  await expect(status(page)).toHaveText('2 of 2 places left · 2 suggested')
  await page.getByRole('button', { name: 'Keep all 2 suggestions' }).click()
  await expect(status(page)).toHaveText('All 2 places are decided')
  await expect(place(page, 2)).toHaveAccessibleName('Place 2 · kept the suggestion')
  await expect(page.getByRole('button', { name: /Keep all/ })).toHaveCount(0)

  await place(page, 1).getByRole('button', { name: 'Ours' }).click()
  await expect.poll(() => text(page)).toContain(head.ours)
  await place(page, 1).getByRole('button', { name: 'Start again' }).click()
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · suggested: both sides combined')
  await expect.poll(() => text(page)).toContain(combined.trimEnd())
  await expect(status(page)).toHaveText('1 of 2 places left · 1 suggested')
  // 3 keeps the current place's suggestion.
  await place(page, 1).click()
  await page.keyboard.press('Alt+3')
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · kept the suggestion')
})

test('the panes mark what each side changed against the original', async ({ page }) => {
  await open(page, 'conflictView', scenario())
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · suggested: both sides combined')
  // A change can span several highlighted tokens, each marked.
  const marked = (side: string) => page.locator(`.pane.${side} .changed`).allTextContents().then((t) => t.join(''))
  await expect.poll(() => marked('ours')).toBe('ctx int,')
  await expect.poll(() => marked('theirs')).toBe(', step int')
  await page.getByRole('button', { name: 'Three-way' }).click()
  await expect(page.locator('.pane.base .changed')).toHaveCount(0)
})

test("a file that couldn't be read keeps the line tool, and says why", async ({ page }) => {
  await open(page, 'conflictView', scenario({ ConflictSuggestions: reading({ fallback: 'errors', suggestions: [] }) }))
  const chip = page.getByText('Read by lines')
  await expect(chip).toBeVisible()
  await expect(chip).toHaveAttribute('title', 'No suggestions: more than a quarter of it doesn’t parse as Go.')
  // The reason is in view too, above the result.
  await expect(page.getByRole('note')).toHaveText('No suggestions: more than a quarter of it doesn’t parse as Go.')
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · choose a side, or type here')
  await expect(status(page)).toHaveText('2 of 2 places left')
})

test('a file not read as code is the tool as it was', async ({ page }) => {
  await open(page, 'conflictView', scenario({ ConflictSuggestions: null }))
  await expect(place(page, 1)).toHaveAccessibleName('Place 1 · choose a side, or type here')
  await expect(page.getByText(/^Read (as|by)/)).toHaveCount(0)
  await expect(page.locator('.legend')).toHaveCount(0)
})

test('names the merge driver git ran first, or offers the way to set one up', async ({ page }) => {
  await open(page, 'conflictView', scenario({ ConflictSuggestions: reading({ driver: 'mergiraf', driverCommand: 'mergiraf merge --git %O %A %B -s %S -x %X -y %Y -p %P' }) }))
  await expect(page.getByText('mergiraf ran first')).toBeVisible()
  await expect(page.getByText('mergiraf merge --git %O %A %B -s %S -x %X -y %Y -p %P')).toBeVisible()

  await open(page, 'conflictView', scenario())
  await expect(page.getByText('git used its own line merge.', { exact: false })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Using a merge driver' })).toBeVisible()
})

test("the file list says how the stop's other files stand, and what a merge driver settled", async ({ page }) => {
  const stop = {
    files: [
      { path: 'README.md', places: 2, suggested: 1, language: 'Markdown', driver: '' },
      { path: 'internal/tides/forecast.go', places: 2, suggested: 1, language: 'Go', driver: '' },
    ],
    settled: [{ path: 'go.sum', driver: 'mergiraf' }],
    drivers: [{ name: 'mergiraf', command: 'mergiraf merge' }],
  }
  await open(page, 'conflictView', scenario({ ConflictStop: stop }))
  const files = page.getByRole('complementary', { name: /Files in this merge/ })
  await expect(files.getByRole('listitem').filter({ hasText: 'README.md' })).toContainText('1 suggested, 1 undecided')
  await expect(files.getByRole('listitem').filter({ hasText: 'go.sum' })).toContainText('merged by mergiraf')
})

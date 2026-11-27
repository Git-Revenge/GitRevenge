import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// rerere in the conflict tool (#938): a file git resolved with a recorded resolution says so,
// whether it's still to mark resolved or rerere staged it, and its resolution can be forgotten.
// With rerere off, a conflict resolved here before brings a one-time offer to turn it on. The
// backend is stubbed.

const lines = (...l: string[]) => l.join('\n') + '\n'
const ours = lines('const FEED = 1', '// Wind in knots.', 'export function read(id) {', '  return wind / 1.852', '}')
const theirs = lines('const FEED = 1', '// Wind in metres per second.', 'export function read(id) {', '  return wind / 3.6', '}')
const base = lines('const FEED = 1', 'export function read(id) {', '  return wind', '}')
const conflicted = lines('const FEED = 1', '<<<<<<< HEAD', '// Wind in knots.', '=======', '// Wind in metres per second.', '>>>>>>> feature/metric-units', 'export function read(id) {', '  return wind / 1.852', '}')
// What rerere put in the file: a resolution, no markers.
const reusedText = lines('const FEED = 1', '// Wind in knots and metres per second.', 'export function read(id) {', '  return wind / 1.852', '}')

const side = (text: string) => ({ exists: true, text, size: text.length })
const file = (merged: string) => ({ path: 'src/feed.js', base: side(base), ours: side(ours), theirs: side(theirs), merged: side(merged), binary: false, diff3: merged })
const merging = (conflicts: string[]) => ({ operation: 'merge', step: 0, total: 0, actions: ['continue', 'abort'], conflicts, ours: 'main', theirs: 'feature/metric-units', commit: 'c'.repeat(40), subject: 'Report wind in metres per second', origHead: 'd'.repeat(40), written: [] })
const scenario = (rerere: object, more: object = {}) => ({
  responses: {
    ConflictFile: file(conflicted),
    RepoState: merging(['README.md', 'src/feed.js']),
    ConflictContext: { mergeTool: '', clean: [], rerere: { on: true, resolved: [], recurring: [], ...rerere } },
    ConflictOrigin: { commit: '', author: '', subject: '' },
    ForgetResolution: null,
    TurnOnRerere: null,
    DeclineRerere: null,
    ...more,
  },
})
const list = (page: Page) => page.getByRole('complementary', { name: /Files in this/ })

test('a file rerere resolved says so, and its resolution can be forgotten', async ({ page }) => {
  await open(page, 'conflictView', scenario({ resolved: ['src/feed.js'] }, { ConflictFile: file(reusedText) }))
  await expect(list(page).locator('li', { hasText: 'src/feed.js' })).toContainText('resolved as before')
  await expect(list(page)).toContainText('git’s rerere resolved this file as this conflict was resolved before. Check it, then mark it resolved.')
  await expect.poll(() => calls(page, 'ConflictFile')).toHaveLength(1)
  await list(page).getByRole('button', { name: 'Forget that resolution' }).click()
  await expect.poll(() => calls(page, 'ForgetResolution')).toEqual([['/r', 'src/feed.js']])
  // The file is read again, its conflict back.
  await expect.poll(async () => (await calls(page, 'ConflictFile')).length).toBe(2)
})

test('a file rerere staged isn’t taken for one that merged cleanly', async ({ page }) => {
  await open(page, 'conflictView', scenario({ resolved: ['src/units.js'] }))
  const row = list(page).locator('li', { hasText: 'src/units.js' })
  await expect(row).toContainText('resolved as before')
  await expect(list(page)).not.toContainText('Merged cleanly')
  await row.getByRole('button', { name: 'Forget' }).click()
  await expect.poll(() => calls(page, 'ForgetResolution')).toEqual([['/r', 'src/units.js']])
  // Its conflict is back: the tool opens it.
  await expect.poll(() => events(page, 'open')).toEqual([['src/units.js']])
})

test('a conflict resolved here before offers rerere, once', async ({ page }) => {
  await open(page, 'conflictView', scenario({ on: false, recurring: ['src/feed.js'] }))
  const offer = list(page).locator('.offer')
  await expect(offer).toContainText('You’ve resolved src/feed.js’s conflict before. Turn on rerere and git reuses a resolution when the same conflict comes back.')
  await offer.getByRole('button', { name: 'Turn on rerere' }).click()
  await expect.poll(() => calls(page, 'TurnOnRerere')).toEqual([['/r']])
  await expect(offer).toHaveCount(0)
})

test('turning the offer down asks no more', async ({ page }) => {
  await open(page, 'conflictView', scenario({ on: false, recurring: ['src/feed.js', 'README.md'] }))
  const offer = list(page).locator('.offer')
  await expect(offer).toContainText('You’ve resolved the conflicts in 2 files before.')
  await offer.getByRole('button', { name: 'No thanks' }).click()
  await expect.poll(() => calls(page, 'DeclineRerere')).toEqual([['/r']])
  await expect(offer).toHaveCount(0)
})

test('without rerere or a repeat, nothing about it shows', async ({ page }) => {
  await open(page, 'conflictView', scenario({ on: false }))
  await expect(list(page)).toContainText('2 files to settle')
  await expect(list(page)).not.toContainText('resolved as before')
  await expect(list(page).locator('.offer')).toHaveCount(0)
})

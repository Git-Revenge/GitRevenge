import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Two versions of a branch compared with git range-diff (#1038): Compare two versions of a
// branch…, its dialog, and the panel that lists what became of each commit. The backend is
// stubbed, with the demo's feature/offline-cache before and after an autosquash rebase.

const id = (c: string) => c.repeat(40)
const now = Math.floor(Date.now() / 1000)
const base = { Refs: [], CommitSignatures: [], CommitNotes: [], CommitRow: 0, WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } } }
const reflog = [
  { selector: 'feature/offline-cache@{0}', commit: id('e'), action: 'rebase (finish): refs/heads/feature/offline-cache onto ' + id('c'), time: now - 240 },
  { selector: 'feature/offline-cache@{1}', commit: id('2'), action: 'commit: debugging', time: now - 3600 },
]
// The demo's two changed pairs, as range-diff wrote them.
const CHANGED = [
  '1:  1637a6b ! 1:  f6df5ac Cache the last reading from each station',
  '    @@ src/cache.js (new)',
  '     +const cache = new Map()',
  '     +',
  '     +export function remember(id, reading) {',
  '    -+  cache.set(id, { reading, at: Date.now() })',
  '    ++  if (reading) cache.set(id, { reading, at: Date.now() })',
  '     +}',
].join('\n')
const AROUND = [
  '2:  eda7bec ! 2:  f16c299 wip',
  '    @@ Commit message',
  '      ## src/cache.js ##',
  '     @@ src/cache.js: const cache = new Map()',
  '      export function remember(id, reading) {',
  '    -   cache.set(id, { reading, at: Date.now() })',
  '    +   if (reading) cache.set(id, { reading, at: Date.now() })',
  '      }',
].join('\n')
const pair = (old: number, nw: number, kind: string, subject: string, more: object = {}) => ({ old, new: nw, oldId: old ? id(String(old)) : '', newId: nw ? id('a') : '', subject, kind, text: `${old || '-'}:  x ${kind} ${nw || '-'}:  y ${subject}`, ...more })
const compared = {
  old: { base: id('c'), tip: id('2') },
  new: { base: id('c'), tip: id('e') },
  oldCount: { commits: 5, merges: 0 },
  newCount: { commits: 4, merges: 0 },
  pairs: [
    pair(1, 1, 'changed', 'Cache the last reading from each station', { text: CHANGED }),
    pair(2, 2, 'around', 'wip', { text: AROUND }),
    pair(3, 0, 'folded', 'fixup! Cache the last reading from each station', { into: 1 }),
    pair(4, 3, 'same', 'Fall back to the cached reading when a station is down'),
    pair(5, 4, 'same', 'debugging'),
  ],
  text: '1:  1637a6b ! 1:  f6df5ac Cache the last reading from each station\n…',
  command: 'git range-diff ccccccc..2222222 ccccccc..eeeeeee',
  loose: false,
}
const scenario = (responses: object = {}) => ({
  graphRows: 3,
  current: 'feature/offline-cache',
  responses: { ...base, Reflog: reflog, VersionBase: id('c'), CountVersion: { commits: 5, merges: 0 }, CompareVersions: compared, ...responses },
})

async function openDialog(page: Page) {
  await expect.poll(() => page.evaluate(() => typeof (window as unknown as { __runCommand?: unknown }).__runCommand)).toBe('function')
  await page.evaluate(() => (window as unknown as { __runCommand: (id: string) => void }).__runCommand('repo.compareVersions'))
  return page.getByRole('dialog', { name: 'Compare two versions' })
}
const panel = (page: Page) => page.getByRole('complementary', { name: 'Two versions of feature/offline-cache' })
const rows = (page: Page) => panel(page).getByRole('list', { name: 'What became of each commit' }).locator('li')

test.describe('two versions of a branch', () => {
  test('the dialog starts on the branch, its last tip before now, and where each left its base', async ({ page }) => {
    await open(page, 'patchSources', scenario())
    const d = await openDialog(page)
    await expect(d.getByLabel('Up to').first()).toHaveValue(id('2'))
    await expect(d.locator('select option').first()).toHaveText(/^2222222, before the rebase · 4 min ago$/)
    await expect(d.getByLabel('Since').first()).toHaveValue('ccccccc')
    await expect(d.getByRole('textbox', { name: 'Up to' })).toHaveValue('feature/offline-cache')
    await expect(d).toContainText('5 commits, no merges')
    await expect(d).toContainText('git range-diff ccccccc..2222222 ccccccc..feature/offline-cache')
    expect((await calls(page, 'VersionBase'))[0]).toEqual(['/r', 'feature/offline-cache', '', id('2')])
  })

  test('Compare lists what became of each commit, in words', async ({ page }) => {
    await open(page, 'patchSources', scenario())
    const d = await openDialog(page)
    await expect(d).toContainText('5 commits, no merges')
    await d.getByRole('button', { name: 'Compare' }).click()
    await expect.poll(() => calls(page, 'CompareVersions')).toEqual([['/r', { base: 'ccccccc', tip: id('2') }, { base: 'ccccccc', tip: 'feature/offline-cache' }, false]])
    await expect(d).toHaveCount(0)
    const p = panel(page)
    await expect(p).toContainText('Before the rebase 2222222 → now eeeeeee')
    await expect(p).toContainText('5 commits then, 4 now: 1 changed, 1 changed only around it, 2 the same, 1 folded in.')
    await expect(rows(page).locator('.pill')).toHaveText(['changed', 'changed only around it', 'folded into 1', 'the same', 'the same'])
    await expect(rows(page).locator('.places')).toHaveText(['1 → 1', '2 → 2', '3 → –', '4 → 3', '5 → 4'])
    await expect(p).toContainText('As git range-diff ccccccc..2222222 ccccccc..eeeeeee pairs them.')
  })

  test('Hide the same, a row shows its commit, and Copy as text', async ({ page }) => {
    await open(page, 'patchSources', scenario())
    const d = await openDialog(page)
    await expect(d).toContainText('5 commits, no merges')
    await d.getByRole('button', { name: 'Compare' }).click()
    await panel(page).getByLabel('Hide the same').check()
    await expect(rows(page).locator('.pill')).toHaveText(['changed', 'folded into 1'])
    await expect(panel(page)).toContainText('3 the same, hidden.')
    // The folded fixup has no new commit: its old one shows in the graph.
    await rows(page).nth(1).getByRole('button').click()
    await expect.poll(async () => (await calls(page, 'CommitRow')).at(-1)).toEqual(['/r', id('3')])
    await panel(page).getByRole('button', { name: 'Copy as text' }).click()
    await expect(panel(page).getByRole('button', { name: 'Copied' })).toBeVisible()
    await panel(page).getByRole('button', { name: 'Close the comparison' }).click()
    await expect(panel(page)).toHaveCount(0)
  })

  test('says what it couldn’t find, and waits', async ({ page }) => {
    await open(page, 'patchSources', { ...scenario({ Reflog: [], VersionBase: '' }), failures: { CompareVersions: '“nowhere” isn\'t a branch, tag or commit here.' } })
    const d = await openDialog(page)
    await expect(d.getByRole('button', { name: 'Compare' })).toBeDisabled()
    await d.getByRole('textbox', { name: "The old version's tip" }).fill('nowhere')
    await d.getByLabel('Since').first().fill('main')
    await d.getByLabel('Since').nth(1).fill('main')
    await d.getByRole('button', { name: 'Compare' }).click()
    await expect(d.getByRole('alert')).toHaveText('“nowhere” isn\'t a branch, tag or commit here.')
  })
})

test.describe('a changed pair, read', () => {
  async function compare(page: Page) {
    await open(page, 'patchSources', scenario())
    const d = await openDialog(page)
    await expect(d).toContainText('5 commits, no merges')
    await d.getByRole('button', { name: 'Compare' }).click()
    await expect(rows(page)).toHaveCount(5)
  }
  const view = (page: Page) => page.getByRole('region', { name: /^Pair \d+:/ })

  test('Marked marks what is new and what is gone, and the words that changed', async ({ page }) => {
    await compare(page)
    await rows(page).first().getByRole('button').click()
    const v = view(page)
    await expect(v).toContainText('pair 1 of 5')
    await expect(v.getByRole('heading', { name: 'Cache the last reading from each station' })).toBeVisible()
    await expect(v).toContainText('the message is the same · 1 line differs')
    // As written: the app's global .meta would set it in capitals.
    await expect(v.locator('.said').first()).toHaveCSS('text-transform', 'none')
    const marked = v.getByRole('table', { name: 'src/cache.js, marked' })
    await expect(marked.locator('.row.gone .mark')).toHaveText('gone')
    await expect(marked.locator('.row.new .mark')).toHaveText('new')
    await expect(marked.locator('.row.new mark')).toHaveText('if (reading) ')
    await expect(marked.locator('.row.gone .text')).toHaveCSS('text-decoration-line', 'line-through')
    await expect(rows(page).first().getByRole('button')).toHaveClass(/picked/)
  })

  test('Side by side sets the old beside the new, and stays chosen', async ({ page }) => {
    await compare(page)
    await rows(page).first().getByRole('button').click()
    await view(page).getByRole('button', { name: 'Side by side' }).click()
    const side = view(page).getByRole('table', { name: 'src/cache.js, side by side' })
    await expect(side.locator('.half.gone .text')).toHaveText('  cache.set(id, { reading, at: Date.now() })')
    await expect(side.locator('.half.new .text')).toHaveText('  if (reading) cache.set(id, { reading, at: Date.now() })')
    await view(page).getByRole('button', { name: 'Close the pair' }).click()
    await rows(page).first().getByRole('button').click()
    await expect(view(page).getByRole('button', { name: 'Side by side' })).toHaveAttribute('aria-pressed', 'true')
  })

  test('steps through the pairs that changed, and says what moved the lines around one', async ({ page }) => {
    await compare(page)
    await rows(page).first().getByRole('button').click()
    await expect(view(page).getByRole('button', { name: '← Previous pair' })).toBeDisabled()
    await view(page).getByRole('button', { name: 'Next pair →' }).click()
    await expect(view(page)).toContainText('pair 2 of 5')
    await expect(view(page)).toContainText('Its own lines are the same in both. Only lines around it differ, because pair 1 changed them: Cache the last reading from each station.')
    await expect(view(page).getByRole('button', { name: 'Next pair →' })).toBeDisabled()
    await expect(rows(page).nth(1).getByRole('button')).toHaveClass(/picked/)
    await view(page).getByRole('button', { name: 'Cache the last reading from each station' }).click()
    await expect(view(page)).toContainText('pair 1 of 5')
    await view(page).getByRole('button', { name: 'Copy as text' }).click()
    await expect(view(page).getByRole('button', { name: 'Copied' })).toBeVisible()
  })
})

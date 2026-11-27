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
const pair = (old: number, nw: number, kind: string, subject: string, more: object = {}) => ({ old, new: nw, oldId: old ? id(String(old)) : '', newId: nw ? id('a') : '', subject, kind, text: `${old || '-'}:  x ${kind} ${nw || '-'}:  y ${subject}`, ...more })
const compared = {
  old: { base: id('c'), tip: id('2') },
  new: { base: id('c'), tip: id('e') },
  oldCount: { commits: 5, merges: 0 },
  newCount: { commits: 4, merges: 0 },
  pairs: [
    pair(1, 1, 'changed', 'Cache the last reading from each station'),
    pair(2, 2, 'around', 'wip'),
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
    await rows(page).first().getByRole('button').click()
    await expect.poll(async () => (await calls(page, 'CommitRow')).at(-1)).toEqual(['/r', id('a')])
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

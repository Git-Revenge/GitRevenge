import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Patches from a dropped file or the clipboard (#1025): the overlay while files are dragged
// over the window, and what a drop or the clipboard opens. The backend is stubbed: a drop
// arrives as the "patches:dropped" event, once the backend has marked the files readable.

const base = { Refs: [], CommitSignatures: [], CommitNotes: [], CommitRow: 0, WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } } }
const scenario = (responses: object = {}, more: object = {}) => ({ graphRows: 3, current: 'main', responses: { ...base, ...responses }, ...more })

const patch = (subject: string, number: number) => ({ subject, author: 'Tyler Brooks', email: 't@example.com', date: 'Wed, 21 Oct 2026 09:00:00 +0000', number, total: 2, version: 1, cover: false, hasPatch: true, files: [{ path: 'src/feed.js', added: 3, deleted: 1 }] })
const series = { token: 'patches-1', name: '0001-time-out.patch and 1 more', kind: 'mail', messages: [patch('Time out a station', 1), patch('Show wind in knots', 2)], staged: false, branch: 'main' }
const diff = { token: 'patches-2', name: 'the clipboard', kind: 'diff', messages: [], staged: false, branch: 'main', diff: { files: [{ path: 'src/feed.js', added: 1, deleted: 1, binary: false, created: false, removed: false }], named: true, unclean: [] } }

/** Drags a file over the window, as the system does, without letting go. */
async function dragOver(page: Page, names: string[]) {
  const data = await page.evaluateHandle((files) => {
    const d = new DataTransfer()
    for (const f of files) d.items.add(new File(['x'], f))
    return d
  }, names)
  await page.dispatchEvent('body', 'dragenter', { dataTransfer: data })
  await page.dispatchEvent('body', 'dragover', { dataTransfer: data })
}
const dropped = (page: Page, paths: string[]) => page.evaluate((p) => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('patches:dropped', { paths: p }), paths)
const overlay = (page: Page) => page.getByRole('status').filter({ hasText: /Let go|first/ })

test.describe('the drop overlay', () => {
  test('says what letting go does, and to which repository and branch', async ({ page }) => {
    await open(page, 'patchSources', scenario())
    await dragOver(page, ['0001-time-out.patch'])
    await expect(overlay(page)).toContainText('Let go to preview the file')
    await expect(overlay(page)).toContainText('as patches or a diff for tidewater, on main. Nothing changes until you apply.')
    // It goes when the drag stops coming.
    await expect(overlay(page)).toHaveCount(0)
    await dragOver(page, ['0001-a.patch', '0002-b.patch', '0003-c.patch'])
    await expect(overlay(page)).toContainText('Let go to preview the 3 files')
  })

  test('says to open a repository first, or to switch to one', async ({ page }) => {
    await open(page, 'patchSources', scenario({}, { tabs: 'none' }))
    await dragOver(page, ['fix.diff'])
    await expect(overlay(page)).toContainText('Open a repository first')
  })

  test('says to switch to a repository when another tab is on screen, and a drop opens nothing', async ({ page }) => {
    await open(page, 'patchSources', scenario({}, { tabs: 'other' }))
    await dragOver(page, ['fix.diff'])
    await expect(overlay(page)).toContainText('Switch to a repository’s tab first')
    await dropped(page, ['C:/x/fix.diff'])
    await page.waitForTimeout(300)
    expect(await calls(page, 'ReadPatchFiles')).toEqual([])
  })

  test('ignores a drag that holds no files', async ({ page }) => {
    await open(page, 'patchSources', scenario())
    const data = await page.evaluateHandle(() => {
      const d = new DataTransfer()
      d.setData('text/plain', 'some words')
      return d
    })
    await page.dispatchEvent('body', 'dragover', { dataTransfer: data })
    await page.waitForTimeout(100)
    await expect(overlay(page)).toHaveCount(0)
  })
})

test.describe('what a drop opens', () => {
  test('mail opens the series preview, several files read as one', async ({ page }) => {
    await open(page, 'patchSources', scenario({ ReadPatchFiles: series, TryPatches: [] }))
    await dropped(page, ['C:/p/0002-show-wind.patch', 'C:/p/0001-time-out.patch'])
    await expect.poll(() => calls(page, 'ReadPatchFiles')).toEqual([['/r', ['C:/p/0002-show-wind.patch', 'C:/p/0001-time-out.patch']]])
    const d = page.getByRole('dialog', { name: 'Time out a station' })
    await expect(d).toContainText('0001-time-out.patch and 1 more')
  })

  test('a plain diff opens the diff preview', async ({ page }) => {
    await open(page, 'patchSources', scenario({ ReadPatchFiles: { ...diff, name: 'fix.diff' }, TryDiff: [] }))
    await dropped(page, ['C:/p/fix.diff'])
    await expect(page.getByRole('dialog', { name: 'Apply a diff' })).toContainText('fix.diff · a plain diff')
  })

  test('something else says so, naming it, and opens nothing', async ({ page }) => {
    await open(page, 'patchSources', scenario({ ReadPatchFiles: { token: '', name: 'holiday.jpg', kind: '', messages: [], staged: false, branch: 'main' } }))
    await dropped(page, ['C:/p/holiday.jpg'])
    await expect(page.getByText('Not a patch or a diff: holiday.jpg. Nothing was opened.')).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })
})

test.describe('the clipboard', () => {
  test('Apply the patch on the clipboard opens the same preview', async ({ page }) => {
    await open(page, 'patchSources', scenario({ ReadClipboardPatches: diff, TryDiff: [] }))
    await expect.poll(() => page.evaluate(() => typeof (window as unknown as { __runCommand?: unknown }).__runCommand)).toBe('function')
    await page.evaluate(() => (window as unknown as { __runCommand: (id: string) => void }).__runCommand('repo.applyClipboard'))
    await expect.poll(() => calls(page, 'ReadClipboardPatches')).toEqual([['/r']])
    await expect(page.getByRole('dialog', { name: 'Apply a diff' })).toContainText('the clipboard · a plain diff')
  })

  test('text that is neither says so', async ({ page }) => {
    await open(page, 'patchSources', scenario({ ReadClipboardPatches: { token: '', name: 'the clipboard', kind: '', messages: [], staged: false, branch: 'main' } }))
    await expect.poll(() => page.evaluate(() => typeof (window as unknown as { __runCommand?: unknown }).__runCommand)).toBe('function')
    await page.evaluate(() => (window as unknown as { __runCommand: (id: string) => void }).__runCommand('repo.applyClipboard'))
    await expect(page.getByText('The clipboard doesn’t hold a patch or a diff.')).toBeVisible()
  })
})

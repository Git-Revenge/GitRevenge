import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Applying a plain diff with git apply (#1024): each file tried first, then applied to the
// files, the index or both, or taken back out; files with changes of yours wait, and Undo puts
// them back. The backend is stubbed.

const files = [
  { path: 'src/feed.js', added: 3, deleted: 1, binary: false, created: false, removed: false },
  { path: 'src/knots.js', added: 12, deleted: 0, binary: false, created: true, removed: false },
  { path: 'src/old-units.js', added: 0, deleted: 20, binary: false, created: false, removed: true },
  { path: 'config.yaml', added: 1, deleted: 1, binary: false, created: false, removed: false },
]
const file = (diff: object = {}) => ({ token: 'patches-1', name: 'knots.diff', kind: 'diff', messages: [], staged: false, branch: 'main', diff: { files, named: true, unclean: [], ...diff } })
const tried = [
  { path: 'src/feed.js', status: 'merges' },
  { path: 'src/knots.js', status: 'applies' },
  { path: 'src/old-units.js', status: 'applies' },
  { path: 'config.yaml', status: 'fails', error: 'patch failed: config.yaml:3' },
]
const rows = (page: Page) => page.getByRole('list', { name: 'The files it changes' }).locator('li')
const statusOf = (page: Page, n: number) => rows(page).nth(n).locator('.status')
const applied = { id: 'diff-1', files: 3, conflicts: [] }

test.describe('applying a diff', () => {
  test('tries each file and says how it goes', async ({ page }) => {
    await open(page, 'applyDiff', { patchFile: file(), responses: { TryDiff: tried } })
    const d = page.getByRole('dialog', { name: 'Apply a diff' })
    await expect(d).toContainText('knots.diff · a plain diff · 4 files · +16 −22')
    await expect(d).toContainText('It has no author or message, so its changes land in Changes')
    await expect(rows(page).locator('.path')).toHaveText(['src/feed.js', 'src/knots.js', 'src/old-units.js', 'config.yaml'])
    await expect(statusOf(page, 0)).toHaveText('applies by merging')
    await expect(statusOf(page, 1)).toHaveText('a new file')
    await expect(statusOf(page, 2)).toHaveText('deleted')
    await expect(statusOf(page, 3)).toHaveText('doesn’t fit')
    await expect(statusOf(page, 3)).toHaveAttribute('title', 'patch failed: config.yaml:3')
    // One that doesn't fit can't be ticked: git apply would refuse the lot.
    await expect(rows(page).nth(3).getByRole('checkbox')).toBeDisabled()
    await expect(rows(page).nth(3).getByRole('checkbox')).not.toBeChecked()
    await expect(d.getByRole('button', { name: 'Apply to 3 files' })).toBeEnabled()
    expect(await calls(page, 'TryDiff')).toEqual([['/r', 'patches-1', false]])
  })

  test('applies the ticked files in the mode chosen, and Undo puts them back', async ({ page }) => {
    await open(page, 'applyDiff', { patchFile: file(), responses: { TryDiff: tried, ApplyDiff: applied, UndoDiff: null } })
    const d = page.getByRole('dialog', { name: 'Apply a diff' })
    await expect(statusOf(page, 0)).toHaveText('applies by merging')
    await rows(page).nth(2).getByRole('checkbox').uncheck()
    await d.getByLabel('The files, and stage them').check()
    await d.getByRole('button', { name: 'Apply to 2 files' }).click()
    await expect.poll(() => calls(page, 'ApplyDiff')).toEqual([['/r', 'patches-1', ['src/feed.js', 'src/knots.js'], 'index', false]])
    await expect(page.getByText('Applied the diff to 3 files.')).toBeVisible()
    await expect.poll(async () => (await events(page, 'close')).length).toBe(1)
    await page.getByRole('button', { name: 'Undo' }).click()
    await expect.poll(() => calls(page, 'UndoDiff')).toEqual([['/r', 'diff-1']])
    await expect(page.getByText('Its files are back as they were.')).toBeVisible()
  })

  test('in reverse, tries taking it back out', async ({ page }) => {
    await open(page, 'applyDiff', { patchFile: file(), responses: { TryDiff: tried } })
    const d = page.getByRole('dialog', { name: 'Apply a diff' })
    await expect(statusOf(page, 1)).toHaveText('a new file')
    await respond(page, { TryDiff: [{ path: 'src/feed.js', status: 'applied' }, { path: 'src/knots.js', status: 'applies' }, { path: 'src/old-units.js', status: 'applies' }, { path: 'config.yaml', status: 'applies' }] })
    await d.getByLabel('In reverse: take it back out').check()
    await expect.poll(async () => (await calls(page, 'TryDiff')).at(-1)).toEqual(['/r', 'patches-1', true])
    await expect(statusOf(page, 0)).toHaveText('not here to take out')
    await expect(statusOf(page, 1)).toHaveText('deleted')
    await expect(statusOf(page, 2)).toHaveText('comes back')
    await expect(d.getByRole('button', { name: 'Take out of 3 files' })).toBeEnabled()
  })

  test('waits for files with changes of yours, and names them', async ({ page }) => {
    await open(page, 'applyDiff', { patchFile: file({ unclean: ['src/feed.js'] }), responses: { TryDiff: tried } })
    const d = page.getByRole('dialog', { name: 'Apply a diff' })
    await expect(statusOf(page, 0)).toHaveText('has your changes')
    await expect(d.getByRole('alert')).toHaveText('src/feed.js has changes of yours. Commit or stash them first, so the diff can be undone cleanly, or leave it out.')
    await expect(d.getByRole('button', { name: 'Apply to 3 files' })).toBeDisabled()
    // Left out, the rest can go.
    await rows(page).nth(0).getByRole('checkbox').uncheck()
    await expect(d.getByRole('alert')).toHaveCount(0)
    await expect(d.getByRole('button', { name: 'Apply to 2 files' })).toBeEnabled()
  })

  test('staged only refuses a conflict, and a conflict elsewhere offers to put the files back', async ({ page }) => {
    const clash = [{ path: 'src/feed.js', status: 'conflicts', files: ['src/feed.js'] }, ...tried.slice(1)]
    await open(page, 'applyDiff', { patchFile: file(), responses: { TryDiff: clash, ApplyDiff: { id: 'diff-2', files: 3, conflicts: ['src/feed.js'] }, UndoDiff: null } })
    const d = page.getByRole('dialog', { name: 'Apply a diff' })
    await expect(statusOf(page, 0)).toHaveText('merges with a conflict')
    await d.getByLabel('Staged only').check()
    await expect(d.getByRole('alert')).toContainText('src/feed.js would merge with a conflict, which staging only can’t hold.')
    await expect(d.getByRole('button', { name: 'Apply to 3 files' })).toBeDisabled()
    await d.getByLabel('The files', { exact: true }).check()
    await d.getByRole('button', { name: 'Apply to 3 files' }).click()
    await expect(page.getByText('Applied the diff to 3 files; src/feed.js has a conflict to resolve in Changes.')).toBeVisible()
    await page.getByRole('button', { name: 'Put these files back' }).click()
    await expect.poll(() => calls(page, 'UndoDiff')).toEqual([['/r', 'diff-2']])
  })

  test('says when a diff names no versions, so it only fits exactly', async ({ page }) => {
    await open(page, 'applyDiff', { patchFile: file({ named: false }), responses: { TryDiff: tried } })
    await expect(page.getByRole('dialog', { name: 'Apply a diff' })).toContainText('It doesn’t name the versions it was made from, as diff -u doesn’t, so it applies only where it fits exactly.')
  })
})

const respond = (page: Page, responses: object) => page.evaluate((r) => (window as unknown as { __respond: (r: object) => void }).__respond(r), responses)

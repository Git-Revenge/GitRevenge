import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Applying a series of patches with git am, previewed first (#1022): the newest version of a
// thread in [n/m] order, each patch tried before anything changes, what's in already left
// out, and git am with the ones ticked. The backend is stubbed.

const msg = (subject: string, number = 0, total = 0, version = 1, kind: 'patch' | 'cover' | 'reply' = 'patch') => ({
  subject,
  author: kind === 'reply' ? 'Reviewer' : 'Tyler Brooks',
  email: 't@example.com',
  date: 'Wed, 21 Oct 2026 09:00:00 +0000',
  number,
  total,
  version,
  cover: kind === 'cover',
  hasPatch: kind === 'patch',
  files: kind === 'patch' ? [{ path: 'src/feed.js', added: 3, deleted: 1 }] : [],
})
// A thread saved from a list archive: v1, v2 with a cover letter and its patches out of order,
// and a reply.
const messages = [
  msg('Time out a station', 1, 2),
  msg('Show wind in knots', 2, 2),
  msg('Wind speeds in knots', 0, 3, 2, 'cover'),
  msg('Show wind in knots', 2, 3, 2),
  msg('Time out a station that stops answering', 1, 3, 2),
  msg('Looks good', 1, 3, 2, 'reply'),
  msg('Set the alert limit in knots too', 3, 3, 2),
]
const file = (more: object = {}) => ({ token: 'patches-1', name: 'knots.mbox', kind: 'mail', messages, staged: false, branch: 'main', ...more })
const tried = [
  { token: 'patches-1', run: 1, index: 4, status: 'merges' },
  { token: 'patches-1', run: 1, index: 3, status: 'applied', already: 'abcdef0123456789abcdef0123456789abcdef01' },
  { token: 'patches-1', run: 1, index: 6, status: 'conflicts', files: ['config.yaml'] },
]
const rows = (page: Page) => page.getByRole('list', { name: 'The patches, in the order they apply' }).locator('li')
const statusOf = (page: Page, n: number) => rows(page).nth(n).locator('.status')

test.describe('applying patches', () => {
  test('shows the newest version of a thread in order, with its cover letter, and counts the rest', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file(), responses: { TryPatches: tried } })
    const d = page.getByRole('dialog', { name: 'Wind speeds in knots' })
    await expect(d).toContainText('knots.mbox · 7 messages: a cover letter, 3 patches, 2 messages of other versions and 1 reply left out')
    await expect(d.getByLabel('v2 · 3 patches')).toBeChecked()
    await expect(rows(page).locator('.n')).toHaveText(['1/3', '2/3', '3/3'])
    await expect(rows(page).locator('.subject')).toHaveText(['Time out a station that stops answering', 'Show wind in knots', 'Set the alert limit in knots too'])
    await expect(d).toContainText('the cover letter isn’t applied')
    await d.getByLabel('v1 · 2 patches').check()
    await expect(rows(page).locator('.subject')).toHaveText(['Time out a station', 'Show wind in knots'])
    await expect.poll(async () => (await calls(page, 'TryPatches')).at(-1)?.[2]).toEqual([0, 1])
  })

  test('fills each row as its try finishes, and leaves out what’s in already', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file(), responses: { TryPatches: tried }, delays: { TryPatches: 2000 } })
    await expect.poll(async () => (await calls(page, 'TryPatches')).at(-1)?.slice(1)).toEqual(['patches-1', [4, 3, 6], 1])
    await expect(statusOf(page, 0)).toHaveText('trying…')
    await page.evaluate((row) => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('patches:tried', row), tried[0])
    await expect(statusOf(page, 0)).toHaveText('applies by merging')
    await expect(statusOf(page, 1)).toHaveText('trying…')
    // The rest arrive with the answer.
    await expect(statusOf(page, 1)).toHaveText('already here as abcdef0')
    await expect(rows(page).nth(1).getByRole('checkbox')).not.toBeChecked()
    await expect(statusOf(page, 2)).toHaveText('conflicts in config.yaml')
    await expect(page.getByText('Patch 3/3 stops for config.yaml; you resolve it there.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Apply 2 patches' })).toBeEnabled()
    // An automatic untick tries nothing again.
    expect(await calls(page, 'TryPatches')).toHaveLength(1)
  })

  test('applies the ticked ones onto a new branch named after the series', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file(), responses: { TryPatches: tried } })
    await expect(statusOf(page, 1)).toHaveText('already here as abcdef0')
    await page.getByLabel('A new branch from here').check()
    await expect(page.getByLabel("The new branch's name")).toHaveValue('wind-speeds-in-knots')
    await page.getByLabel('Add Signed-off-by').check()
    await page.getByRole('button', { name: 'Apply 2 patches' }).click()
    expect((await calls(page, 'ApplyPatches')).at(-1)).toEqual(['/r', 'patches-1', [4, 6], 'wind-speeds-in-knots', true])
    await expect(page.getByText('Applied 2 patches onto wind-speeds-in-knots.')).toBeVisible()
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('unticking one tries the rest again', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file(), responses: { TryPatches: tried } })
    await expect(statusOf(page, 0)).toHaveText('applies by merging')
    await rows(page).nth(0).getByRole('checkbox').uncheck()
    await expect.poll(async () => (await calls(page, 'TryPatches')).at(-1)?.[2]).toEqual([6])
    await expect(statusOf(page, 0)).toHaveText('left out')
  })

  test('staged changes keep Apply off, and say why', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file({ staged: true }), responses: { TryPatches: tried } })
    await expect(page.getByRole('alert')).toHaveText('git am needs nothing staged. Commit or unstage your staged changes first.')
    await expect(page.getByRole('button', { name: /^Apply \d+ patch/ })).toBeDisabled()
  })

  test('in a partial clone, the preview asks to download first', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file({ missing: ['a'.repeat(40)] }), responses: { TryPatches: tried } })
    await expect(page.getByText('The preview needs 1 file version this partial clone hasn’t got yet.')).toBeVisible()
    await expect(statusOf(page, 0)).toHaveText('not tried')
    expect(await calls(page, 'TryPatches')).toEqual([])
    await page.getByRole('button', { name: 'Download them' }).click()
    await expect.poll(async () => (await calls(page, 'TryPatches')).length).toBe(1)
    expect((await calls(page, 'DownloadFiles')).at(-1)?.[2]).toEqual(['a'.repeat(40)])
  })

  test('when git am stops on a conflict, the strip over the graph carries on', async ({ page }) => {
    await open(page, 'applyPatches', { patchFile: file(), responses: { TryPatches: tried, RepoState: { operation: 'am', conflicts: ['config.yaml'] } }, failures: { ApplyPatches: 'A patch stopped on a conflict.' } })
    await expect(statusOf(page, 2)).toHaveText('conflicts in config.yaml')
    await page.getByRole('button', { name: 'Apply 2 patches' }).click()
    await expect.poll(async () => (await events(page, 'close')).length).toBe(1)
  })
})

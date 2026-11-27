import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Saving commits as patches, and copying them (#1021): the dialog lists the commits before
// anything is written, every choice is one of format-patch's, files already there are
// replaced only when asked, and a partial clone downloads first. The backend is stubbed.

const id = (i: number) => String(i).padStart(40, '0')
const sha = (c: string) => c.repeat(40)
const commit = (c: string, subject: string, first = '') => ({ id: sha(c), subject, author: 'Tyler Brooks', first })
const series = (commits: object[], more: object = {}) => ({ commits, count: commits.length, merges: 0, edge: false, base: sha('0'), ...more })
const three = series([commit('a', 'Time out a station that stops answering', sha('0')), commit('b', 'Show wind in knots'), commit('c', 'Set the alert limit in knots too')])
const saved = (files: string[]) => ({ files, folder: 'A:\\out' })
const dialog = (page: Page) => page.getByRole('dialog')
const revs = { since: 'origin/main', tip: 'knots', last: 0, ids: [] }

test.describe('Save as patches', () => {
  test('starts a branch where it left its base, and lists the commits oldest first', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'branch', branch: 'knots' }, responses: { BranchPatchBase: 'origin/main', PatchSeries: three } })
    await expect(dialog(page).getByRole('heading', { name: 'Save knots as patches' })).toBeVisible()
    await expect(dialog(page).getByLabel('Since')).toHaveValue('base')
    await expect(dialog(page).getByLabel('Since').locator('option:checked')).toHaveText('origin/main, where knots left it')
    await expect.poll(async () => (await calls(page, 'PatchSeries')).at(-1)).toEqual(['/r', revs])
    await expect(dialog(page).getByRole('list', { name: 'The commits, oldest first' }).locator('li')).toHaveText([/1\/3\s*aaaaaaa\s*Time out/, /2\/3\s*bbbbbbb\s*Show wind/, /3\/3\s*ccccccc\s*Set the alert/])
    await expect(dialog(page).locator('.example')).toHaveText('[PATCH 1/3] Time out a station that stops answering')
    // The last few, from the same tip.
    await dialog(page).getByLabel('Since').selectOption('last')
    await expect.poll(async () => (await calls(page, 'PatchSeries')).at(-1)).toEqual(['/r', { since: '', tip: 'knots', last: 3, ids: [] }])
  })

  test('saves with the options as format-patch takes them, then offers the folder', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'branch', branch: 'knots' }, responses: { BranchPatchBase: 'origin/main', PatchSeries: three, SavePatches: saved(['v2-0000-cover-letter.patch', 'v2-0001-a.patch', 'v2-0002-b.patch', 'v2-0003-c.patch']) } })
    const d = dialog(page)
    await expect(d.locator('.example')).toBeVisible()
    await d.getByLabel('Prefix').fill('RFC PATCH')
    await d.getByLabel('Version').fill('2')
    await expect(d.locator('.example')).toHaveText('[RFC PATCH v2 1/3] Time out a station that stops answering')
    await d.getByLabel('Into folder').fill('A:\\out')
    await d.getByLabel('A cover letter first').check()
    await d.getByLabel("The cover letter's subject").fill('Wind speeds in knots')
    await d.getByLabel("The cover letter's text").fill('Sailors asked for knots.')
    await d.getByLabel('Add Signed-off-by').check()
    await d.getByLabel('Say which commit they apply to').check()
    await d.getByRole('button', { name: 'Save 4 files' }).click()
    expect((await calls(page, 'SavePatches')).at(-1)).toEqual([
      '/r',
      revs,
      { prefix: 'RFC PATCH', version: 2, numbered: true, cover: true, coverSubject: 'Wind speeds in knots', coverText: 'Sailors asked for knots.', signoff: true, base: sha('0') },
      'A:\\out',
      '',
      false,
    ])
    await expect(page.getByText('Saved 4 files to A:\\out')).toBeVisible()
    expect(await events(page, 'close')).toHaveLength(1)
    await page.getByRole('button', { name: 'Show in folder' }).click()
    expect(await calls(page, 'ShowFolder')).toEqual([['A:\\out']])
  })

  test('files already there are replaced only when asked', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'commit', tip: sha('c'), subject: 'Set the alert limit' }, responses: { PatchSeries: series([commit('c', 'Set the alert limit', sha('b'))]), SavePatches: { files: [], folder: 'A:\\out', exists: ['0001-Set-the-alert-limit.patch'] } } })
    const d = dialog(page)
    await d.getByRole('button', { name: 'Save 1 file' }).click()
    await expect(d.getByRole('alert')).toContainText('0001-Set-the-alert-limit.patch is already in')
    await expect(d.getByRole('button', { name: 'Save 1 file' })).toBeDisabled()
    await page.evaluate(() => (window as unknown as { __respond: (r: object) => void }).__respond({ SavePatches: { files: ['0001-Set-the-alert-limit.patch'], folder: 'A:\\out' } }))
    await d.getByRole('button', { name: 'Replace it' }).click()
    expect((await calls(page, 'SavePatches')).map((c) => c[5])).toEqual([false, true])
    await expect(page.getByText('Saved 0001-Set-the-alert-limit.patch to A:\\out')).toBeVisible()
  })

  test('one mailbox file instead, named after the series', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'branch', branch: 'feature/knots' }, responses: { BranchPatchBase: 'origin/main', PatchSeries: three, SavePatches: saved(['feature-knots.mbox']) } })
    const d = dialog(page)
    await d.getByLabel('One mailbox file').check()
    await expect(d.getByLabel("The mailbox file's name")).toHaveValue('feature-knots.mbox')
    await d.getByRole('button', { name: 'Save feature-knots.mbox' }).click()
    expect((await calls(page, 'SavePatches')).at(-1)?.[4]).toBe('feature-knots.mbox')
  })

  test('says what it leaves out, and won’t save past a shallow clone’s edge', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'commit', tip: sha('c'), subject: 'x' }, responses: { PatchSeries: series([commit('c', 'Edge', '')], { edge: true, merges: 1, base: '' }) } })
    const d = dialog(page)
    await expect(d).toContainText('A merge is left out, as format-patch leaves merges out.')
    await expect(d.getByRole('alert')).toContainText('this shallow clone’s history stops')
    await expect(d.getByRole('button', { name: 'Save 1 file' })).toBeDisabled()
    await expect(d.getByLabel('Say which commit they apply to')).toHaveCount(0)
  })

  test('in a partial clone, a series downloads its files first, once Save says so', async ({ page }) => {
    await open(page, 'savePatches', { patchStart: { kind: 'branch', branch: 'knots' }, responses: { BranchPatchBase: 'origin/main', PatchSeries: { ...three, missing: [sha('d')] }, SavePatches: saved(['0001-a.patch']) }, delays: { DownloadFiles: 1500 } })
    const d = dialog(page)
    await expect(d).toContainText('They need 1 file version this partial clone hasn’t got yet.')
    await d.getByRole('button', { name: 'Download and save' }).click()
    await expect(d.getByRole('status')).toContainText('Downloading their files…')
    await expect.poll(async () => (await calls(page, 'SavePatches')).length).toBe(1)
    expect((await calls(page, 'DownloadFiles')).at(-1)?.[2]).toEqual([sha('d')])
  })
})

test.describe('from the graph', () => {
  test.use({ timezoneId: 'UTC' })
  const rows = (n: number) =>
    Array.from({ length: n }, (_, i) => ({ index: i, col: 0, color: 0, up: i ? [{ from: 0, to: 0, color: 0 }] : [], down: [{ from: 0, to: 0, color: 0 }], width: 1, id: id(i), parents: 1, author: `Author ${i}`, authorTime: 1793500000 - i * 60, subject: `Commit ${i}` }))
  const person = { name: 'A', email: 'a@example.com', time: 1793500000 }
  const base = {
    Refs: [],
    CommitSignatures: [],
    CommitNotes: [],
    WorkingTree: { status: { files: [] }, identity: { name: 'A', email: 'a@example.com' }, signing: { enabled: false } },
    CommitDetail: { commit: { id: id(1), subject: 'Commit 1', body: '', trailers: [], parents: [id(2)], author: person, committer: person }, files: [] },
    PatchSeries: { commits: [{ id: id(2), subject: 'Commit 2', author: 'A', first: id(3) }], count: 1, merges: 0, edge: false, base: id(3) },
    CopyPatches: 'From 000 Mon Sep 17 00:00:00 2001\nSubject: [PATCH] Commit 2\n',
  }
  async function rightClick(page: Page, i: number) {
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + i * 52 + 26, { button: 'right' })
  }

  test('a commit copies as a patch, and saves up to here', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(6), responses: base })
    await rightClick(page, 2)
    await page.getByRole('menuitem', { name: 'Copy as a patch' }).click()
    await expect.poll(async () => (await events(page, 'copy')).at(-1)?.[0]).toContain('Subject: [PATCH] Commit 2')
    expect((await calls(page, 'CopyPatches')).at(-1)?.[1]).toEqual({ since: '', tip: id(2), last: 1, ids: [] })

    await rightClick(page, 2)
    await page.getByRole('menuitem', { name: 'Save as patches, up to here…' }).click()
    await expect(page.getByRole('dialog', { name: 'Save as patches' })).toBeVisible()
    await expect(page.getByRole('dialog').getByLabel('Since')).toHaveValue('last')
    await expect(page.getByRole('dialog').getByLabel('How many commits')).toHaveValue('1')
  })

  test('a merge offers neither, and an edge commit says why not', async ({ page }) => {
    const data = rows(6)
    data[1].parents = 2
    ;(data[3] as Record<string, unknown>).edge = true
    await open(page, 'repoView', { graphData: data, responses: base })
    await rightClick(page, 1)
    await expect(page.getByRole('menuitem', { name: 'Copy as a patch' })).toHaveCount(0)
    await page.keyboard.press('Escape')
    await rightClick(page, 3)
    await expect(page.getByRole('menuitem', { name: /^Copy as a patch/ })).toBeDisabled()
    await expect(page.getByRole('menuitem', { name: /^Save as patches, up to here/ })).toContainText('its patch would add every file')
  })

  test('a selected line saves as a range, from before its oldest', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(6), responses: base })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 52 + 26)
    await page.keyboard.press('Shift+ArrowDown')
    await page.keyboard.press('Shift+ArrowDown')
    const panel = page.getByRole('complementary', { name: 'Several commits selected' })
    await panel.getByRole('button', { name: 'Save as patches…' }).click()
    await expect(page.getByRole('dialog', { name: 'Save as patches' })).toBeVisible()
    await expect(page.getByRole('dialog').getByLabel('Since')).toHaveCount(0)
    await expect.poll(async () => (await calls(page, 'PatchSeries')).at(-1)).toEqual(['/r', { since: id(4), tip: id(1), last: 0, ids: [] }])
  })
})

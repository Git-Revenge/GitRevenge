import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// #1023: the strip while git am is applying a series. It names the patch in hand and who wrote
// it, says when one didn't apply at all, shows that patch, and counts what went in at the end.

const id = (c: string) => c.repeat(40)

const am = (over: object = {}) => ({
  operation: 'am',
  step: 4,
  total: 5,
  actions: ['continue', 'skip', 'abort'],
  conflicts: ['src/alerts.js'],
  ours: 'main',
  theirs: '',
  commit: '',
  subject: 'Set the alert limit in knots too',
  origHead: id('d'),
  written: [],
  author: 'Tyler Brooks',
  applied: 3,
  stuck: false,
  stuckReason: '',
  madeBranch: '',
  ...over,
})

const stuck = (over: object = {}) =>
  am({ step: 2, applied: 1, conflicts: [], actions: ['skip', 'abort'], stuck: true, stuckReason: 'src/depth.js: patch does not apply', subject: 'Read the depth in metres', author: 'Jessica Nguyen', ...over })

const idle = { operation: '', step: 0, total: 0, actions: [], conflicts: [], ours: '', theirs: '', commit: '', subject: '', origHead: '', written: [] }

const PATCH = ['diff --git a/src/depth.js b/src/depth.js', 'index 1234567..89abcde 100644', '--- a/src/depth.js', '+++ b/src/depth.js', '@@ -1,2 +1,2 @@', ' const unit = 1', '-export const depth = (ft) => ft', '+export const depth = (m) => m', ''].join('\n')

const scenario = (state: object, more: object = {}) => ({
  graphRows: 3,
  responses: {
    Refs: [],
    CommitSignatures: [],
    CommitNotes: [],
    CommitRow: 0,
    WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
    RepoState: state,
    Bisect: null,
    UndoState: { undo: '', redo: '' },
    CurrentPatch: PATCH,
    ...more,
  },
})

const strip = (page: Page) => page.getByRole('status').filter({ hasText: 'Applying patches' })
const respond = (page: Page, responses: object) => page.evaluate((r) => (window as unknown as { __respond: (r: object) => void }).__respond(r), responses)
const becomes = async (page: Page, responses: object) => {
  await respond(page, responses)
  await page.evaluate(() => (window as unknown as { __changed: (path: string) => void }).__changed('/r'))
}

test('a patch stopped on a conflict is named, with who wrote it, and a bead for each', async ({ page }) => {
  await open(page, 'operation', scenario(am()))
  const s = strip(page)
  await expect(s).toContainText('Applying patches onto main')
  await expect(s).toContainText('Stopped at patch 4 of 5: Set the alert limit in knots too · Tyler Brooks')
  await expect(s.getByRole('img', { name: '3 of 5 done, stopped at 4' })).toBeVisible()
  await expect(s.getByRole('button', { name: 'Resolve 1 file' })).toBeVisible()
  await expect(s.getByRole('button', { name: 'Continue' })).toHaveAttribute('title', 'Resolve src/alerts.js first')
  await expect(s.getByRole('button', { name: 'Show the patch' })).toHaveCount(0)

  // Resolved: Continue says how many are left.
  await becomes(page, { RepoState: am({ conflicts: [] }) })
  await expect(s).toContainText('Patch 4 of 5 is resolved and staged.')
  await s.getByRole('button', { name: /Continue with 1 more/ }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'continue']])
})

test("a patch that didn't apply says why, shows the patch, and offers only Skip and Abort", async ({ page }) => {
  await open(page, 'operation', scenario(stuck()))
  const s = strip(page)
  await expect(s).toContainText('Patch 2 of 5 didn’t apply: Read the depth in metres · Jessica Nguyen')
  await expect(s).toContainText('Git says “src/depth.js: patch does not apply”, and there’s nothing in conflict to resolve.')
  await expect(s.getByRole('button', { name: /Continue/ })).toHaveCount(0)
  await expect(s.getByRole('button', { name: 'Resolve 1 file' })).toHaveCount(0)

  await s.getByRole('button', { name: 'Show the patch' }).click()
  await expect.poll(() => calls(page, 'CurrentPatch')).toEqual([['/r']])
  const view = page.getByRole('dialog', { name: "The patch that didn't apply" })
  await expect(view).toContainText('Read the depth in metres')
  const shown = view.getByLabel('The patch')
  await expect(shown.locator('.l-add')).toHaveText('+export const depth = (m) => m')
  await expect(shown.locator('.l-del')).toHaveText('-export const depth = (ft) => ft')
  await expect(shown.locator('.l-hunk')).toHaveText('@@ -1,2 +1,2 @@')
  // A file's header reads as git wrote it: the app's global .meta would set it in capitals.
  await expect(shown.locator('.l-meta').first()).toHaveText('index 1234567..89abcde 100644')
  await expect(shown.locator('.l-meta').first()).toHaveCSS('text-transform', 'none')
  await view.getByRole('button', { name: 'Close' }).click()
  await expect(view).toHaveCount(0)

  // Changed by hand and staged, it isn't stuck: Continue is back.
  await becomes(page, { RepoState: stuck({ stuck: false, stuckReason: '', actions: ['continue', 'skip', 'abort'] }) })
  await expect(s.getByRole('button', { name: /Continue with 3 more/ })).toBeVisible()
  await expect(s.getByRole('button', { name: 'Show the patch' })).toHaveCount(0)
})

test('skipping the last patch says how many went in and how many were skipped', async ({ page }) => {
  // Patch 5 of 5, with 3 in: one was skipped before this.
  await open(page, 'operation', scenario(stuck({ step: 5, applied: 3 })))
  const s = strip(page)
  await respond(page, { RepoState: idle, UndoState: { undo: 'Apply patches', redo: '' } })
  await s.getByRole('button', { name: 'Skip this patch' }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'skip']])
  await expect(page.getByText('Applied 3 patches onto main; skipped 2.')).toBeVisible()
  await expect(s).toHaveCount(0)
})

test('aborting takes away the branch made for the patches, and says so first', async ({ page }) => {
  await open(page, 'operation', scenario(am({ step: 1, applied: 0, ours: 'wind/knots', madeBranch: 'wind/knots' })))
  const s = strip(page)
  await expect(s).toContainText('Applying patches onto wind/knots')
  await s.getByRole('button', { name: 'Abort…' }).click()
  await expect(s).toContainText('wind/knots goes, as it was made for them, and you’re back where you were. Your work on the conflicts is thrown away.')
  await respond(page, { RepoState: idle })
  await s.getByRole('button', { name: 'Abort the patches' }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'abort']])
  await expect(page.getByText('Applying the patches stopped, and wind/knots, made for them, is gone.')).toBeVisible()
})

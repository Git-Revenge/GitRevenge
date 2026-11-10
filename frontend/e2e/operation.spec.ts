import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open } from './support'

// #593: one strip for a merge, rebase, cherry-pick or revert in progress. The backend is
// stubbed; these check what the strip says for each state and what it asks git to do.

const id = (c: string) => c.repeat(40)

const rebase = (over: object = {}) => ({
  operation: 'rebase',
  step: 3,
  total: 5,
  actions: ['continue', 'skip', 'abort'],
  conflicts: ['src/feed.js'],
  ours: 'feature/offline-cache',
  theirs: 'main',
  commit: id('c'),
  subject: 'Fall back to the cached reading',
  origHead: id('d'),
  written: [id('b'), id('a')],
  ...over,
})

const merge = (over: object = {}) => ({ ...rebase(), operation: 'merge', step: 0, total: 0, actions: ['continue', 'abort'], ours: 'main', theirs: 'feature/metric-units', written: [], ...over })
const side = (text: string) => ({ text, exists: true })
const idle = { operation: '', step: 0, total: 0, actions: [], conflicts: [], ours: '', theirs: '', commit: '', subject: '', origHead: '', written: [] }

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
    ...more,
  },
})

const strip = (page: Page) => page.getByRole('status').filter({ hasText: /Rebasing|Merging|Cherry-picking|Reverting/ })
/** Changes what the stubbed backend answers, then tells the app the repository changed. */
const respond = (page: Page, responses: object) => page.evaluate((r) => (window as unknown as { __respond: (r: object) => void }).__respond(r), responses)
const becomes = async (page: Page, responses: object) => {
  await respond(page, responses)
  await page.evaluate(() => (window as unknown as { __changed: (path: string) => void }).__changed('/r'))
}

test('a stopped rebase names both sides, the commit in hand, and what is left', async ({ page }) => {
  await open(page, 'operation', scenario(rebase()))
  const s = strip(page)
  await expect(s).toContainText('Rebasing feature/offline-cache onto main')
  await expect(s).toContainText('Stopped at commit 3 of 5: Fall back to the cached reading')
  await expect(s.getByRole('img', { name: '2 of 5 done, stopped at 3' })).toBeVisible()
  // Continue is there to say why it's off, and does nothing.
  const go = s.getByRole('button', { name: 'Continue' })
  await expect(go).toHaveAttribute('aria-disabled', 'true')
  await expect(go).toHaveAttribute('title', 'Resolve src/feed.js first')
  await go.click({ force: true })
  expect(await calls(page, 'RunOperationAction')).toEqual([])
})

test('Resolve opens the first conflicted file in the conflict tool', async ({ page }) => {
  await open(page, 'operation', scenario(rebase(), { ConflictFile: { path: 'src/feed.js', base: side('base'), ours: side('ours'), theirs: side('theirs'), merged: side('merged'), binary: false } }))
  await strip(page).getByRole('button', { name: 'Resolve 1 file' }).click()
  await expect.poll(() => calls(page, 'ConflictFile')).toContainEqual(['/r', 'src/feed.js'])
  const tool = page.getByRole('button', { name: 'Mark resolved' })
  await expect(tool).toBeVisible()
  // Resolved somewhere else: the tool has nothing true left to show, so it closes.
  await becomes(page, { RepoState: rebase({ conflicts: [] }) })
  await expect(tool).toHaveCount(0)
})

test('once resolved, Continue says how many are left and runs on Ctrl+Enter', async ({ page }) => {
  await open(page, 'operation', scenario(rebase()))
  const s = strip(page)
  await expect(s).toContainText('Stopped at commit 3 of 5')
  await becomes(page, { RepoState: rebase({ conflicts: [] }) })
  await expect(s).toContainText('Commit 3 of 5 is resolved and staged.')
  await expect(s.getByRole('button', { name: /Continue with 2 more/ })).toBeVisible()
  await expect(s.getByRole('button', { name: /Resolve/ })).toHaveCount(0)
  await page.keyboard.press('Control+Enter')
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'continue']])
})

test('a merge counts its conflicts down, then offers to commit', async ({ page }) => {
  await open(page, 'operation', scenario(merge({ conflicts: ['src/feed.js', 'README.md'] })))
  const s = strip(page)
  await expect(s).toContainText('Merging feature/metric-units into main')
  await expect(s).toContainText('2 files need resolving')
  await expect(s.getByRole('button', { name: 'Skip this commit' })).toHaveCount(0)

  await becomes(page, { RepoState: merge({ conflicts: ['README.md'] }) })
  await expect(s).toContainText('1 of 2 files resolved.')
  await becomes(page, { RepoState: merge({ conflicts: [] }) })
  await expect(s).toContainText('All 2 conflicts are resolved.')
  await s.getByRole('button', { name: /Commit the merge/ }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'continue']])
})

test('Abort asks first and says what it undoes', async ({ page }) => {
  await open(page, 'operation', scenario(rebase()))
  const s = strip(page)
  await s.getByRole('button', { name: 'Abort…' }).click()
  await expect(s).toContainText('feature/offline-cache goes back to ddddddd, where it was before.')
  await expect(s).toContainText('The 2 commits already written again and your work on the conflicts are thrown away.')
  expect(await calls(page, 'RunOperationAction')).toEqual([])
  await s.getByRole('button', { name: 'Keep going' }).click()
  await expect(s.getByRole('button', { name: 'Abort…' })).toBeVisible()

  await s.getByRole('button', { name: 'Abort…' }).click()
  await respond(page, { RepoState: idle })
  await s.getByRole('button', { name: 'Abort the rebase' }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'abort']])
  await expect(page.getByText('Rebase aborted. feature/offline-cache is back at ddddddd.')).toBeVisible()
  await expect(s).toHaveCount(0)
})

test('finishing says what happened, offers Undo, and a force push when the branch was pushed', async ({ page }) => {
  const pushed = branch('feature/offline-cache', { head: true, upstream: 'origin/feature/offline-cache', ahead: 5, behind: 5 })
  await open(page, 'operation', scenario(rebase({ conflicts: [], step: 5 }), { Refs: [pushed] }))
  const s = strip(page)
  await respond(page, { RepoState: idle, UndoState: { undo: 'Rebase onto main', redo: '' } })
  await s.getByRole('button', { name: /Finish the rebase/ }).click()
  await expect(page.getByText('Rebased feature/offline-cache onto main.')).toBeVisible()
  await expect(page.getByText('origin/feature/offline-cache still has the commits from before the rebase.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Force push…' })).toBeVisible()
  await page.getByRole('button', { name: 'Undo', exact: true }).last().click()
  await expect.poll(() => calls(page, 'Undo')).toEqual([['/r']])
})

test('a cherry-pick names its commit and can be skipped', async ({ page }) => {
  await open(page, 'operation', scenario(rebase({ operation: 'cherry-pick', step: 0, total: 0, ours: 'main', theirs: '', written: [] })))
  const s = strip(page)
  await expect(s).toContainText('Cherry-picking ccccccc onto main')
  await expect(s).toContainText('Stopped at ccccccc: Fall back to the cached reading')
  await s.getByRole('button', { name: 'Skip this commit' }).click()
  await expect.poll(() => calls(page, 'RunOperationAction')).toEqual([['/r', 'skip']])
})

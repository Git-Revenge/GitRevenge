import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open } from './support'

// #595: a merge or a rebase shows what it would do before it runs. The backend is stubbed; these
// check what the dialog says for each forecast and what it asks git to do.

const id = (c: string) => c.repeat(40)
const commit = (c: string, subject: string, fixupOf = '') => ({ id: id(c), subject, author: 'Sam Ortiz', fixupOf })

const merge = (over: object = {}) => ({
  into: 'main',
  alreadyMerged: false,
  fastForward: false,
  commits: [commit('a', 'Document the units'), commit('b', 'Report wind in metres per second'), commit('c', 'Add a units module')],
  commitCount: 3,
  intoOnly: 2,
  files: 5,
  conflicts: [],
  unknown: false,
  ...over,
})

const rebase = (over: object = {}) => ({
  branch: 'main',
  commits: [commit('a', 'Cache the last reading'), commit('b', 'wip'), commit('c', 'fixup! Cache the last reading', id('a'))],
  commitCount: 3,
  fixups: 1,
  upToDate: false,
  stopsAt: '',
  stopsAtSubject: '',
  stopsAtNumber: 0,
  conflicts: [],
  incomplete: false,
  pushed: false,
  upstream: '',
  ...over,
})

const integrate = (page: Page) => calls(page, 'Integrate')

test.describe('merge', () => {
  test('a clean merge says so, lists what comes in, and merges with a merge commit', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge() } })
    const dialog = page.getByRole('dialog', { name: 'Merge feature/metric-units into main' })
    await expect(dialog.getByText('No conflicts expected')).toBeVisible()
    await expect(dialog.getByText('Coming in · 3 commits · 5 files')).toBeVisible()
    await expect(dialog.getByText('Report wind in metres per second')).toBeVisible()
    // The branch has commits of its own, so it can't simply move forward.
    await expect(dialog.getByRole('button', { name: 'Fast-forward only' })).toBeDisabled()
    await expect(dialog.getByRole('button', { name: 'Merge commit' })).toHaveAttribute('aria-pressed', 'true')
    await dialog.getByRole('button', { name: 'Merge', exact: true }).click()
    expect(await integrate(page)).toEqual([['/r', 'merge', ['feature/metric-units', 'no-ff']]])
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('a fast-forward is the default when one is possible, and the button says so', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge({ fastForward: true, intoOnly: 0 }) } })
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByRole('button', { name: 'Fast-forward only' })).toHaveAttribute('aria-pressed', 'true')
    await expect(dialog.getByText('no merge commit is made')).toBeVisible()
    await dialog.getByRole('button', { name: 'Squash into one' }).click()
    await expect(dialog.getByText('Nothing is committed yet')).toBeVisible()
    await dialog.getByRole('button', { name: 'Fast-forward only' }).click()
    await dialog.getByRole('button', { name: 'Fast-forward main' }).click()
    expect(await integrate(page)).toEqual([['/r', 'merge', ['feature/metric-units', 'ff-only']]])
  })

  test('conflicts are named with their places, and the button says how many', async ({ page }) => {
    const conflicts = [
      { path: 'src/feed.js', places: 2 },
      { path: 'README.md', places: 1 },
    ]
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge({ conflicts }) } })
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText('2 files will conflict')).toBeVisible()
    await expect(dialog.getByText('src/feed.js')).toBeVisible()
    await expect(dialog.getByText('2 places')).toBeVisible()
    await expect(dialog.getByText('1 place', { exact: true })).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Squash into one' })).toBeDisabled()
    await dialog.getByRole('button', { name: 'Merge, and resolve 2 conflicts' }).click()
    expect(await integrate(page)).toEqual([['/r', 'merge', ['feature/metric-units', 'no-ff']]])
  })

  test('stopping on the conflicts it named is not reported as an error, but any other failure is', async ({ page }) => {
    const sessionError = () => page.evaluate(() => (window as unknown as { __sessionError: () => string }).__sessionError())
    const conflicts = [{ path: 'src/feed.js', places: 2 }]
    const stopped = { operation: 'merge', step: 0, total: 0, actions: ['abort'], conflicts: ['src/feed.js'] }
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge({ conflicts }), RepoState: stopped }, failures: { Integrate: 'There are conflicts to resolve.' } })
    await page.getByRole('button', { name: 'Merge, and resolve 1 conflict' }).click()
    await expect.poll(() => calls(page, 'RepoState')).toHaveLength(1)
    expect(await sessionError()).toBe('')

    await open(page, 'previewDialog', { responses: { PreviewMerge: merge() }, failures: { Integrate: 'Your local changes would be overwritten.' } })
    await page.getByRole('button', { name: 'Merge', exact: true }).click()
    await expect.poll(sessionError).toBe('Your local changes would be overwritten.')
  })

  test('nothing to merge offers only Close', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge({ alreadyMerged: true, commits: [], commitCount: 0, files: 0 }) } })
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText("There's nothing to merge.")).toBeVisible()
    await expect(dialog.getByRole('button', { name: /Merge/ })).toHaveCount(0)
    await dialog.getByRole('button', { name: 'Close' }).click()
    expect(await integrate(page)).toEqual([])
  })

  test('a forecast that could not be made still lets the merge run', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge({ unknown: true }) } })
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText("Conflicts couldn't be checked ahead of time")).toBeVisible()
    await dialog.getByRole('button', { name: 'Merge', exact: true }).click()
    expect(await integrate(page)).toHaveLength(1)
  })

  test('Enter runs it', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: merge() } })
    await expect(page.getByText('No conflicts expected')).toBeVisible()
    await page.getByRole('button', { name: 'Merge', exact: true }).focus()
    await page.keyboard.press('Enter')
    expect(await integrate(page)).toHaveLength(1)
  })
})

test.describe('rebase', () => {
  const scenario = (over: object = {}) => ({ previewKind: 'rebase' as const, responses: { PreviewRebase: rebase(over) } })

  test('lists the commits oldest first, tags fixups, and folds them by default', async ({ page }) => {
    await open(page, 'previewDialog', scenario())
    const dialog = page.getByRole('dialog', { name: 'Rebase main onto feature/metric-units' })
    await expect(dialog.getByText('3 commits will be written again, oldest first')).toBeVisible()
    await expect(dialog.getByRole('listitem').first()).toContainText('Cache the last reading')
    await expect(dialog.locator('.tag', { hasText: 'fixup' })).toHaveCount(1)
    await expect(dialog.getByLabel('Fold the 1 fixup into the commit it names')).toBeChecked()
    await dialog.getByRole('button', { name: 'Rebase 3 commits' }).click()
    expect(await integrate(page)).toEqual([['/r', 'rebase', ['feature/metric-units', 'autosquash']]])
  })

  test('without folding it is a plain rebase, and with no fixups the choice is not shown', async ({ page }) => {
    await open(page, 'previewDialog', scenario())
    await page.getByLabel(/Fold the 1 fixup/).uncheck()
    await page.getByRole('button', { name: 'Rebase 3 commits' }).click()
    expect(await integrate(page)).toEqual([['/r', 'rebase', ['feature/metric-units']]])

    await open(page, 'previewDialog', scenario({ fixups: 0, commits: [commit('a', 'One')], commitCount: 1 }))
    await expect(page.getByText('1 commit will be written again')).toBeVisible()
    await expect(page.getByLabel(/Fold/)).toHaveCount(0)
  })

  test('says which commit it will stop on, and on what', async ({ page }) => {
    await open(page, 'previewDialog', scenario({ stopsAt: id('b'), stopsAtSubject: 'wip', stopsAtNumber: 2, conflicts: [{ path: 'src/cache.js', places: 1 }] }))
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText('It will stop at commit 2: 1 file conflicts')).toBeVisible()
    await expect(dialog.getByText('src/cache.js')).toBeVisible()
    await expect(dialog.getByText('stops here')).toBeVisible()
  })

  test('warns that a pushed branch will need a force push', async ({ page }) => {
    await open(page, 'previewDialog', scenario({ pushed: true, upstream: 'origin/main' }))
    await expect(page.getByText('This branch is already pushed')).toBeVisible()
    await expect(page.getByText('origin/main')).toBeVisible()
  })

  test('choosing to reorder first opens the rebase editor instead of rebasing', async ({ page }) => {
    await open(page, 'previewDialog', scenario())
    await page.getByLabel('Let me reorder, reword or drop commits first').check()
    await page.getByRole('button', { name: 'Choose what to do with each commit' }).click()
    expect(await integrate(page)).toEqual([])
    expect(await events(page, 'edit')).toEqual([['feature/metric-units']])
  })

  test('says so when not every commit could be tried', async ({ page }) => {
    await open(page, 'previewDialog', scenario({ incomplete: true }))
    await expect(page.getByText('Not every commit could be checked')).toBeVisible()
    await expect(page.getByText('None of the commits that were tried conflict.')).toBeVisible()
  })
})

test('the menu opens the preview and runs nothing itself', async ({ page }) => {
  await open(page, 'commitMenu', { responses: { Refs: [branch('main', { head: true }), branch('topic')] } })
  await page.getByRole('menuitem', { name: 'topic', exact: true }).hover()
  await page.getByRole('menuitem', { name: 'Merge topic into main…' }).click()
  expect(await events(page, 'preview')).toEqual([['merge', 'topic', 'topic']])
  expect(await integrate(page)).toEqual([])
})

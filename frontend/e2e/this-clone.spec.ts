import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// This clone, in the Repository panel (#1005): a clone that's shallow, partial, sparse or one
// branch says so, and each row has the way to the rest, run as a fetch is. The backend is
// stubbed: these check what the section says and what it asks git to do.

const NOV_26_2025 = Date.UTC(2025, 10, 26, 12) / 1000
const every = {
  shallow: { since: NOV_26_2025, commits: 92758 },
  partial: { remote: 'origin', filter: 'blob:none' },
  sparse: { cone: true, folders: ['arch/x86', 'include', 'kernel', 'mm'], files: 9147, total: 97310 },
  oneBranch: { remote: 'origin', branch: 'master' },
}
const base = { Worktrees: [], Submodules: [] }
const section = (page: Page) => page.getByRole('region', { name: 'This clone' })

test.describe('This clone', () => {
  test('isn’t there for a whole clone', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: {} } })
    await expect(page.getByRole('heading', { name: /^Worktrees/ })).toBeVisible()
    await expect(section(page)).toHaveCount(0)
  })

  test('says what each way the clone differs leaves out', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: every } })
    const s = section(page)
    await expect(s).toContainText('History since Nov 26, 2025')
    await expect(s).toContainText('92,758 commits · shallow')
    await expect(s).toContainText("File contents when they're needed")
    await expect(s).toContainText('partial · from origin')
    await expect(s).toContainText('4 folders checked out')
    await expect(s).toContainText('9,147 of 97,310 files · sparse')
    await expect(s).toContainText('arch/x86 · include · kernel · mm')
    await expect(s).toContainText('Only master from origin')
  })

  test('fetches more history as a fetch, with progress and Cancel in the toolbar', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { shallow: every.shallow } }, delays: { FetchMoreHistory: 10_000 } })
    await section(page).getByRole('button', { name: 'Fetch more history' }).click()
    await expect(page.getByRole('menuitem', { name: 'A year more, since Nov 26, 2024' })).toBeVisible()
    await expect(page.getByRole('menuitem', { name: 'All of it' })).toBeVisible()
    await page.getByRole('menuitem', { name: '1,000 commits more' }).click()
    const toolbar = page.getByRole('toolbar', { name: 'Repository actions' })
    await expect(toolbar.getByRole('status')).toContainText('Fetching history')
    await expect(toolbar.getByRole('button', { name: 'Cancel' })).toBeVisible()
    await expect(section(page).getByRole('button', { name: 'Fetch more history' })).toBeDisabled()
    await expect(toolbar.getByRole('button', { name: 'Fetch', exact: true })).toBeDisabled()
    expect(await calls(page, 'FetchMoreHistory')).toEqual([['/r', { commits: 1000, since: 0, all: false }]])
    await toolbar.getByRole('button', { name: 'Cancel' }).click()
    expect(await calls(page, 'CancelNetwork')).toEqual([['/r']])
  })

  test('says what downloading every file costs before it starts', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { partial: every.partial }, BackfillCost: { size: 2.36 * (1 << 30), missing: 1442 } } })
    await section(page).getByRole('button', { name: 'Download them all…' }).click()
    const ask = page.getByRole('dialog', { name: 'Download every file?' })
    await expect(ask).toContainText('1,442 versions to download. The clone takes 2.4 GB now')
    await ask.getByRole('button', { name: 'Download them all' }).click()
    await expect(ask).toHaveCount(0)
    expect(await calls(page, 'Backfill')).toEqual([['/r']])
  })

  test('closed while it counts, the count stops; a long count says so', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { partial: every.partial }, BackfillCost: { size: 1, missing: 1 } }, delays: { BackfillCost: 20_000 } })
    await section(page).getByRole('button', { name: 'Download them all…' }).click()
    const ask = page.getByRole('dialog', { name: 'Download every file?' })
    await expect(ask).toContainText('Counting what it would download…')
    await expect(ask).toContainText('Still counting', { timeout: 12_000 })
    await expect(ask.getByRole('button', { name: 'Download them all' })).toBeEnabled()
    await ask.getByRole('button', { name: 'Not now' }).click()
    expect(await calls(page, 'StopBackfillCost')).toEqual([['/r']])
    expect(await calls(page, 'Backfill')).toEqual([])
  })

  test('changes folders from the folder list, the current ones ticked', async ({ page }) => {
    const counts = [
      { path: '', files: 97310, direct: 30 },
      { path: 'arch', files: 40000, direct: 10 },
      { path: 'arch/x86', files: 5000, direct: 5 },
      { path: 'include', files: 2000, direct: 0 },
      { path: 'kernel', files: 600, direct: 600 },
      { path: 'mm', files: 200, direct: 200 },
      { path: 'net', files: 3000, direct: 0 },
    ]
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { sparse: every.sparse }, RepoFolders: counts } })
    await section(page).getByRole('button', { name: 'Change folders…' }).click()
    const picker = page.getByRole('dialog', { name: 'Change folders' })
    await expect(picker.getByRole('checkbox', { name: 'x86/' })).toBeChecked()
    await expect(picker.getByRole('checkbox', { name: 'mm/' })).toBeChecked()
    await picker.getByRole('checkbox', { name: 'mm/' }).uncheck()
    await picker.getByRole('checkbox', { name: 'net/' }).check()
    await picker.getByRole('button', { name: 'Check out 4 folders' }).click()
    await expect(picker).toHaveCount(0)
    const [[repo, folders]] = (await calls(page, 'SetSparseFolders')) as [string, string[]][]
    expect([repo, [...folders].sort()]).toEqual(['/r', ['arch/x86', 'include', 'kernel', 'net']])
  })

  test('every folder asks first, saying how many files it writes', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { sparse: every.sparse, partial: every.partial } } })
    await section(page).getByRole('button', { name: 'Every folder' }).click()
    const ask = page.getByRole('dialog', { name: 'Check out every folder?' })
    await expect(ask).toContainText('This writes the 88,163 files the sparse checkout leaves out, downloading the ones this partial clone hasn’t got.')
    await ask.getByRole('button', { name: 'Check out every folder' }).click()
    expect(await calls(page, 'SetSparseFolders')).toEqual([['/r', []]])
  })

  test('patterns that aren’t folders can’t be changed here', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { sparse: { cone: false, folders: ['/*.md', '!/docs/'], files: 3, total: 10 } } } })
    await expect(section(page)).toContainText('2 patterns checked out')
    await expect(section(page).getByRole('button', { name: 'Change folders…' })).toBeDisabled()
  })

  test('fetches every branch from a one-branch remote', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { ...base, CloneShape: { oneBranch: every.oneBranch } } })
    await section(page).getByRole('button', { name: 'Fetch every branch' }).click()
    expect(await calls(page, 'FetchEveryBranch')).toEqual([['/r', 'origin']])
  })
})

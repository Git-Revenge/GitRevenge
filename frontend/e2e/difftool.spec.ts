import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Open in diff tool (#939): offered only when git difftool has a tool, naming it, from the diff
// view and the files' menus, for the comparison on screen. The backend is stubbed.

const meld = { name: 'meld', key: 'diff.tool' }
const none = { name: '', key: '' }
const commit = 'c0ffee'.padEnd(40, '0')

test.describe('in the diff view', () => {
  test('opens the comparison on screen in the tool git difftool names', async ({ page }) => {
    await open(page, 'diff', { diffTarget: { mode: 'commit', commit, file: 'src/feed.js', oldPath: 'src/old-feed.js' }, responses: { DiffTool: meld, OpenDiffTool: null } })
    const button = page.getByRole('button', { name: 'Open in meld' })
    await expect(button).toHaveAttribute('title', /diff\.tool/)
    await button.click()
    await expect.poll(() => calls(page, 'OpenDiffTool')).toEqual([['/r', { mode: 'commit', file: 'src/feed.js', oldPath: 'src/old-feed.js', commit }]])
  })

  test('isn’t offered without a tool, or for a view difftool can’t open', async ({ page }) => {
    await open(page, 'diff', { responses: { DiffTool: none } })
    await expect(page.getByRole('button', { name: 'Back' })).toBeVisible()
    await expect.poll(() => calls(page, 'DiffTool')).toHaveLength(1)
    await expect(page.getByRole('button', { name: /^Open in / })).toHaveCount(0)
  })

  test('a file Find in files found has no Open in', async ({ page }) => {
    await open(page, 'diff', { diffTarget: { mode: 'found', file: 'src/feed.js', commit }, responses: { DiffTool: meld } })
    await expect(page.getByRole('button', { name: 'Back' })).toBeVisible()
    await expect.poll(() => calls(page, 'DiffTool')).toHaveLength(1)
    await expect(page.getByRole('button', { name: /^Open in / })).toHaveCount(0)
  })
})

test.describe('in the files’ menus', () => {
  const identity = { name: 'A', email: 'a@example.com' }
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const file = (path: string, index: string, workTree: string) => ({ path, index, workTree, untracked: false, conflicted: false })
  const workingTree = (files: unknown[]) => ({ status: { files }, identity, signing: { enabled: false } })
  const menu = (page: Page) => page.getByRole('menu')

  test('a changed file offers its changes, and says which when both are', async ({ page }) => {
    const files = [file('src/feed.js', '.', 'M'), file('src/both.js', 'M', 'M'), { ...file('notes.md', '?', '?'), untracked: true }]
    await open(page, 'repoView', { graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: workingTree(files), DiffTool: meld, OpenDiffTool: null } })
    const row = (name: string) => page.locator('.lists button.name', { hasText: name })
    await expect(row('feed.js')).toBeVisible()

    await row('feed.js').click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Open in meld' }).click()
    await expect.poll(() => calls(page, 'OpenDiffTool')).toEqual([['/r', { mode: 'unstaged', file: 'src/feed.js', oldPath: '', commit: '' }]])

    await row('both.js').first().click({ button: 'right' })
    await expect(menu(page).getByRole('menuitem', { name: 'Open the unstaged changes in meld' })).toBeVisible()
    await menu(page).getByRole('menuitem', { name: 'Open the staged changes in meld' }).click()
    await expect.poll(async () => (await calls(page, 'OpenDiffTool')).at(-1)).toEqual(['/r', { mode: 'staged', file: 'src/both.js', oldPath: '', commit: '' }])

    // A new file has nothing to compare it with yet.
    await row('notes.md').click({ button: 'right' })
    await expect(menu(page).getByRole('menuitem', { name: 'Copy path' })).toBeVisible()
    await expect(menu(page).getByRole('menuitem', { name: /^Open .*in meld$/ })).toHaveCount(0)
  })

  test('a commit’s file offers its change against the first parent', async ({ page }) => {
    // The stubbed graph's first row is commit 0, all zeros.
    const row0 = '0'.repeat(40)
    const c = { id: row0, subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
    const files = [{ status: 'R', path: 'src/feed.js', oldPath: 'src/old-feed.js', added: 1, deleted: 1 }]
    await open(page, 'repoView', {
      graphRows: 3,
      responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: workingTree([]), CommitDetail: { commit: c, files }, CommitRow: 1, DiffTool: meld, OpenDiffTool: null },
    })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    const tab = page.getByRole('tab', { name: 'Files', exact: true })
    if (await tab.count()) await tab.click()
    await page.locator('.files .file-row', { hasText: 'feed.js' }).click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Open in meld' }).click()
    await expect.poll(() => calls(page, 'OpenDiffTool')).toEqual([['/r', { mode: 'commit', file: 'src/feed.js', oldPath: 'src/old-feed.js', commit: row0 }]])
  })
})

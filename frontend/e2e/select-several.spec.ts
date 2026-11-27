import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open } from './support'

// Several commits selected in the graph (#1018): Shift+click takes the rows between, Ctrl+click
// adds or takes out one, Shift and an arrow grow it, Escape goes back to one. The panel beside
// the graph lists them oldest first, and the menu acts on them all. The backend is stubbed.

const id = (i: number) => String(i).padStart(40, '0')
/** Rows by different authors a minute apart, on one day in any time zone: each takes a whole row in
 * the timeline, with no day line between them to move the rows the clicks aim at. */
const rows = (n: number) =>
  Array.from({ length: n }, (_, i) => ({
    index: i,
    col: 0,
    color: 0,
    up: i ? [{ from: 0, to: 0, color: 0 }] : [],
    down: [{ from: 0, to: 0, color: 0 }],
    width: 1,
    id: id(i),
    parents: 1,
    author: `Author ${i}`,
    authorTime: 1793500000 - i * 60,
    subject: `Commit ${i}`,
  }))
const person = { name: 'Author 0', email: 'a@example.com', time: 1793500000 }
const detail = (i: number) => ({ commit: { id: id(i), subject: `Commit ${i}`, body: '', trailers: [], parents: [id(i + 1)], author: person, committer: person }, files: [] })
const base = {
  Refs: [{ ...branch('main', { head: true }), commit: id(0) }],
  CommitSignatures: [],
  CommitNotes: [],
  WorkingTree: { status: { files: [] }, identity: { name: 'A', email: 'a@example.com' }, signing: { enabled: false } },
  CommitDetail: detail(1),
}

const ROW = 52
/** Clicks commit i's row, with any keys held. */
async function click(page: Page, i: number, modifiers: ('Shift' | 'ControlOrMeta')[] = [], button: 'left' | 'right' = 'left') {
  const box = (await page.locator('.scroller').boundingBox())!
  await page.mouse.move(box.x + 500, box.y + ROW + i * ROW + 26)
  for (const m of modifiers) await page.keyboard.down(m)
  await page.mouse.click(box.x + 500, box.y + ROW + i * ROW + 26, { button })
  for (const m of modifiers) await page.keyboard.up(m)
}
const panel = (page: Page) => page.getByRole('complementary', { name: 'Several commits selected' })
const commitsIn = (page: Page) => panel(page).getByRole('region', { name: 'The commits' }).locator('li .subject')

test.describe('several commits selected', () => {
  test.use({ timezoneId: 'UTC' })
  test('Shift+click takes every row between, and the panel lists them oldest first', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 1)
    await expect(page.getByRole('heading', { name: 'Commit 1' })).toBeVisible()
    await click(page, 3, ['Shift'])
    await expect(panel(page).getByRole('heading', { name: '3 commits' })).toBeVisible()
    await expect(commitsIn(page)).toHaveText(['Commit 3', 'Commit 2', 'Commit 1'])
    await expect(panel(page)).toContainText('3 authors')
    const asked = await calls(page, 'SelectedCommits')
    expect(asked.at(-1)).toEqual(['/r', [{ lo: 1, hi: 3, loId: id(1), hiId: id(3) }]])
    await expect(page.getByRole('status').filter({ hasText: 'commits selected' })).toHaveText('3 commits selected')
  })

  test('Ctrl+click adds one and takes one out, and the last one stays', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 0)
    await click(page, 2, ['ControlOrMeta'])
    await click(page, 4, ['ControlOrMeta'])
    await expect(commitsIn(page)).toHaveText(['Commit 4', 'Commit 2', 'Commit 0'])
    await click(page, 2, ['ControlOrMeta'])
    await expect(commitsIn(page)).toHaveText(['Commit 4', 'Commit 0'])
    await click(page, 0, ['ControlOrMeta'])
    // One left: it's one commit again, in the commit panel.
    await expect(panel(page)).toHaveCount(0)
    await click(page, 4, ['ControlOrMeta'])
    await expect(panel(page)).toHaveCount(0)
  })

  test('Shift and an arrow grow it, and Escape goes back to the one last clicked', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 1)
    await page.keyboard.press('Shift+ArrowDown')
    await page.keyboard.press('Shift+ArrowDown')
    await expect(commitsIn(page)).toHaveText(['Commit 3', 'Commit 2', 'Commit 1'])
    await page.keyboard.press('Shift+ArrowUp')
    await expect(commitsIn(page)).toHaveText(['Commit 2', 'Commit 1'])
    await page.keyboard.press('Escape')
    await expect(panel(page)).toHaveCount(0)
    // Escape keeps the one the keys last moved to.
    expect((await calls(page, 'CommitDetail')).at(-1)).toEqual(['/r', id(2)])
  })

  test('the menu on a selected row acts on them all, oldest first', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base, current: 'main' })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(commitsIn(page)).toHaveCount(3)
    // A line on main: Squash is on once the backend says so.
    await expect(panel(page).getByRole('button', { name: 'Squash into one…' })).toBeEnabled()
    await click(page, 2, [], 'right')
    const menu = page.getByRole('menu')
    await expect(menu.getByRole('menuitem')).toHaveText(['Cherry-pick 3 onto main', 'Revert 3…', 'Squash into one…', 'Copy their ids'])
    await menu.getByRole('menuitem', { name: /^Cherry-pick 3 onto main/ }).click()
    await expect.poll(() => calls(page, 'Integrate')).toEqual([['/r', 'cherry-pick', [id(3), id(2), id(1)]]])

    await click(page, 3, [], 'right')
    await page.getByRole('menu').getByRole('menuitem', { name: /^Revert 3…/ }).click()
    const dialog = page.getByRole('dialog', { name: 'Revert 3 commits' })
    await expect(dialog).toContainText('newest first')
    await dialog.getByRole('button', { name: 'Revert' }).click()
    await expect.poll(() => calls(page, 'Integrate')).toContainEqual(['/r', 'revert-all', [id(1), id(2), id(3)]])
  })

  test('right-click outside the selection selects that row alone', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 1)
    await click(page, 2, ['Shift'])
    await expect(panel(page)).toBeVisible()
    await click(page, 5, [], 'right')
    await expect(page.getByRole('menu').getByRole('menuitem', { name: 'Create branch here…' })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(panel(page)).toHaveCount(0)
  })

  test('a merge among them keeps Revert off, and the panel says how many', async ({ page }) => {
    const merged = rows(8)
    merged[2].parents = 2
    await open(page, 'repoView', { graphData: merged, responses: base })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(panel(page)).toContainText('1 merge')
    await expect(panel(page).getByRole('button', { name: 'Revert 3…' })).toBeDisabled()
  })

  test('the panel takes one out, shows one alone, and opens a file in one of them', async ({ page }) => {
    const files = { files: [{ path: 'src/feed.js', commits: [id(5), id(1)] }, { path: 'README.md', commits: [id(3)] }], total: 2, read: 3 }
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, SelectionFiles: files } })
    // Picked here and there: a line would show as one change instead.
    await click(page, 1)
    await click(page, 3, ['ControlOrMeta'])
    await click(page, 5, ['ControlOrMeta'])
    const changed = panel(page).getByRole('region', { name: 'Files they change' })
    await expect(changed.getByRole('button', { name: /src\/feed\.js/ })).toContainText('2 of 3')
    await changed.getByRole('button', { name: /src\/feed\.js/ }).click()
    await changed.getByRole('button', { name: /Commit 5$/ }).click()
    await expect.poll(async () => (await calls(page, 'FileDiff')).at(-1)?.slice(0, 4)).toEqual(['/r', 'commit', id(5), 'src/feed.js'])

    await panel(page).getByRole('button', { name: 'Take Commit 3 out of the selection' }).click()
    await expect(commitsIn(page)).toHaveText(['Commit 5', 'Commit 1'])
    await panel(page).getByRole('region', { name: 'The commits' }).getByRole('button', { name: /Commit 1$/ }).click()
    await expect(panel(page)).toHaveCount(0)
    await expect(page.getByRole('heading', { name: 'Commit 1' })).toBeVisible()
  })

  test('a reload follows the commits to their rows, and drops those that are gone', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(commitsIn(page)).toHaveCount(3)
    // Commit 2 was rebased away; the others moved down a row.
    await page.evaluate(() => (window as unknown as { __respond: (r: Record<string, unknown>) => void }).__respond({ CommitRows: [4, -1, 2] }))
    await page.evaluate(() => (window as unknown as { __emit: (n: string, d: unknown) => void }).__emit('graph:progress', { path: '/r', rows: 8, done: true, reset: true }))
    await expect.poll(async () => (await calls(page, 'CommitRows')).at(-1)).toEqual(['/r', [id(3), id(2), id(1)]])
    await expect.poll(async () => (await calls(page, 'SelectedCommits')).at(-1)?.[1]).toEqual([
      { lo: 2, hi: 2, loId: id(2), hiId: id(2) },
      { lo: 4, hi: 4, loId: id(4), hiId: id(4) },
    ])
  })
})

test.describe('two commits, and a line of them (#1019)', () => {
  test.use({ timezoneId: 'UTC' })
  const compared = {
    files: [
      { path: 'src/feed.js', status: 'M', added: 3, deleted: 1 },
      { path: 'old.js', status: 'D', added: 0, deleted: 5 },
    ],
    onlyFrom: 0,
    onlyTo: 3,
  }
  const panelHead = (page: Page) => panel(page).locator('header')

  test('two are compared: their difference, which way round, and Swap sides', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, CompareCommits: compared } })
    await click(page, 1)
    await click(page, 4, ['ControlOrMeta'])
    await expect(panelHead(page)).toContainText('two, side by side')
    await expect(panelHead(page)).toContainText('is 3 commits after')
    // Oldest first: from row 4's commit to row 1's.
    await expect.poll(async () => (await calls(page, 'CompareCommits')).at(-1)).toEqual(['/r', id(4), id(1)])
    const diff = panel(page).getByRole('region', { name: 'Their difference' })
    await expect(diff.getByRole('button')).toHaveText([/src\/feed\.js\s*\+3\s*−1/, /old\.js\s*deleted/])
    await diff.getByRole('button', { name: /src\/feed\.js/ }).click()
    await expect.poll(async () => (await calls(page, 'FileDiff')).at(-1)?.slice(0, 4)).toEqual(['/r', 'compare', `${id(4)}..${id(1)}`, 'src/feed.js'])

    await panel(page).getByRole('button', { name: 'Swap sides' }).click()
    await expect.poll(async () => (await calls(page, 'CompareCommits')).at(-1)).toEqual(['/r', id(1), id(4)])
    // The ones selected are still there to act on.
    await expect(panel(page).getByRole('button', { name: 'Cherry-pick 2 onto main' })).toHaveCount(0)
    await expect(panel(page).getByRole('button', { name: 'Revert 2…' })).toBeVisible()
  })

  test('a line of three shows as one change, from before the oldest', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, CompareCommits: compared } })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(panelHead(page)).toContainText('one line of history')
    await expect(panel(page).getByRole('region', { name: 'Changed, all together' })).toContainText('src/feed.js')
    await expect(panel(page)).toContainText('what squashing them would make')
    // From row 3's parent, row 4, to the newest, row 1.
    await expect.poll(async () => (await calls(page, 'CompareCommits')).at(-1)).toEqual(['/r', id(4), id(1)])
    await panel(page).getByRole('region', { name: 'Changed, all together' }).getByRole('button', { name: /old\.js/ }).click()
    await expect.poll(async () => (await calls(page, 'FileDiff')).at(-1)?.slice(0, 4)).toEqual(['/r', 'compare', `${id(4)}..${id(1)}`, 'old.js'])
    expect(await calls(page, 'SelectionFiles')).toEqual([])
  })

  test('a line from the first commit compares from nothing, and says so', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, CompareCommits: compared } })
    await click(page, 5)
    await click(page, 7, ['Shift'])
    await expect(panel(page)).toContainText('the history’s first commit')
    await expect.poll(async () => (await calls(page, 'CompareCommits')).at(-1)).toEqual(['/r', '', id(5)])
  })

  test('commits picked here and there list the files each changes, and compare nothing', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, CompareCommits: compared } })
    await click(page, 0)
    await click(page, 2, ['ControlOrMeta'])
    await click(page, 4, ['ControlOrMeta'])
    await expect(panelHead(page)).toContainText('picked here and there')
    await expect(panel(page).getByRole('region', { name: 'Files they change' })).toBeVisible()
    // Two picked on the way there compared; the three don't.
    expect((await calls(page, 'CompareCommits')).some((c) => c.includes(id(4)))).toBe(false)
  })

  test('in a partial clone the line counts download by themselves, with Stop', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, CompareCommits: { ...compared, countsMissing: ['a'.repeat(40)] } }, delays: { DownloadFiles: 10_000 } })
    await click(page, 1)
    await click(page, 2, ['ControlOrMeta'])
    const note = panel(page).getByRole('status').filter({ hasText: 'line counts' })
    await expect(note).toContainText('Downloading the files for their line counts…')
    expect((await calls(page, 'DownloadFiles')).at(-1)?.[2]).toEqual(['a'.repeat(40)])
    await note.getByRole('button', { name: 'Stop' }).click()
    await expect(note).toContainText('hasn’t got yet')
    await expect(note.getByRole('button', { name: 'Download' })).toBeVisible()
    expect(await calls(page, 'CancelDownload')).toHaveLength(1)
  })
})

test.describe('a line squashed into one (#1020)', () => {
  test.use({ timezoneId: 'UTC' })
  const planOf = (...rows: number[]) => rows.map((i) => ({ commit: id(i), subject: `Commit ${i}`, message: `Commit ${i}` }))
  const editor = { RebasePlan: planOf(3, 2, 1, 0), PreviewRebasePlan: { stopsAt: '', conflicts: [], incomplete: false, pushed: false, upstream: '', dropped: [] }, RepoState: { operation: '', conflicts: [] } }
  const what = (page: Page, subject: string) => page.getByRole('button', { name: new RegExp(`^What to do with ${subject}:`) })

  test('opens the rebase editor with the line marked to squash into its oldest', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, ...editor }, current: 'main' })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    const squash = panel(page).getByRole('button', { name: 'Squash into one…' })
    await expect(squash).toBeEnabled()
    // Asked of the oldest and the newest.
    expect((await calls(page, 'SquashOffer')).at(-1)).toEqual(['/r', id(3), id(1)])
    await squash.click()
    // From the oldest one's parent: the line and what's after it on main.
    await expect.poll(async () => (await calls(page, 'RebasePlan')).at(-1)).toEqual(['/r', id(4)])
    await expect(what(page, 'Commit 3')).toHaveAccessibleName(/: Keep$/)
    await expect(what(page, 'Commit 2')).toHaveAccessibleName(/: Squash$/)
    await expect(what(page, 'Commit 1')).toHaveAccessibleName(/: Squash$/)
    await expect(what(page, 'Commit 0')).toHaveAccessibleName(/: Keep$/)
    // Nothing runs until the plan is started.
    expect(await calls(page, 'InteractiveRebase')).toEqual([])
    await expect(page.getByRole('button', { name: 'Reset the plan' })).toBeEnabled()
  })

  test('says why not: commits on another branch, a gap, the first commit', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: { ...base, SquashOffer: { ok: false, reason: '0000003 is on origin/main too.' } }, current: 'main' })
    const squash = panel(page).getByRole('button', { name: 'Squash into one…' })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(squash).toBeDisabled()
    await expect(squash).toHaveAttribute('title', '0000003 is on origin/main too.')

    await click(page, 1)
    await click(page, 3, ['ControlOrMeta'])
    await click(page, 5, ['ControlOrMeta'])
    await expect(squash).toBeDisabled()
    await expect(squash).toHaveAttribute('title', 'Only for one line on main.')

    await click(page, 5)
    await click(page, 7, ['Shift'])
    await expect(squash).toHaveAttribute('title', 'It starts at the history’s first commit.')
  })

  test('isn’t offered with no branch checked out', async ({ page }) => {
    await open(page, 'repoView', { graphData: rows(8), responses: base })
    await click(page, 1)
    await click(page, 3, ['Shift'])
    await expect(panel(page).getByRole('button', { name: 'Revert 3…' })).toBeVisible()
    await expect(panel(page).getByRole('button', { name: 'Squash into one…' })).toHaveCount(0)
    expect(await calls(page, 'SquashOffer')).toEqual([])
  })
})

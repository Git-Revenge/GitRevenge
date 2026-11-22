import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open } from './support'

// The graph filter's finer points (#863): the selected commit stays when the filter hides its
// row, the way back says when the checked-out commit isn't in the filter, and a commit's
// details mark the trailer the filter picked.

const id = (n: number) => String(n).padStart(40, '0')
const elsewhere = 'f'.repeat(40)
const emit = (page: Page, rows: number) =>
  page.evaluate((n) => (window as unknown as { __emit: (name: string, d: object) => void }).__emit('graph:progress', { path: '/repo', rows: n, done: true, paused: false, estimate: n }), rows)
const respond = (page: Page, r: object) => page.evaluate((x) => (window as unknown as { __respond: (r: object) => void }).__respond(x), r)
const selects = async (page: Page) => (await events(page, 'select')).map((e) => (e[0] as { id?: string } | undefined)?.id)
/** Swaps the graph for one of these commits, in this order, as a filter that changes does. */
const graphOf = (page: Page, commits: number[]) => page.evaluate((c) => (window as unknown as { __graph: (c: number[]) => void }).__graph(c), commits)
const upTo = (n: number) => Array.from({ length: n }, (_, i) => i)

test.describe('under a filter that takes commits out', () => {
  const head = (commit: string) => ({ Refs: [{ ...branch('main', { head: true, branchKind: 'trunk' }), commit }], HeadCommit: { branch: 'main', commit }, CommitSignatures: [] })

  test('the way back says when the checked-out commit is not in the filter', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, sparse: true, responses: head(elsewhere) })
    const note = page.getByRole('note')
    await expect(note).toHaveText(/main\s+isn’t in this filter/)
    // It goes nowhere: there's no button to press.
    await expect(page.getByRole('button', { name: /^Go to the checked-out commit/ })).toHaveCount(0)
    await expect(note).toHaveAttribute('title', /Clear the filter, or switch it to Highlight/)
  })

  test('says nothing of the kind when the checked-out commit is there, or with no filter', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, sparse: true, responses: head(id(0)) })
    await expect.poll(() => calls(page, 'HeadCommit')).not.toEqual([])
    await page.waitForTimeout(400)
    await expect(page.getByRole('note')).toHaveCount(0)

    await open(page, 'graph', { graphRows: 30, responses: head(elsewhere) })
    await expect.poll(() => calls(page, 'HeadCommit')).not.toEqual([])
    await page.waitForTimeout(400)
    await expect(page.getByRole('note')).toHaveCount(0)
  })

  test('a selected commit the filter hides stays selected, and is picked up when it comes back', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, sparse: true, responses: head(id(0)) })
    const grid = page.getByRole('grid', { name: 'Commit history' })
    await grid.focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await expect.poll(async () => (await selects(page)).at(-1)).toBe(id(2))
    const before = (await selects(page)).length

    // The filter changes and commit 2 isn't in the graph any more.
    await graphOf(page, upTo(30).filter((n) => n % 3 === 0))
    await respond(page, { CommitRow: -1 })
    await emit(page, 10)
    await expect.poll(async () => (await calls(page, 'CommitRow')).some((c) => c[1] === id(2))).toBe(true)
    await page.waitForTimeout(500)
    // Nothing else was selected in its place: whoever shows the commit still shows it.
    expect((await selects(page)).length).toBe(before)

    // It's back, at another row: it's selected there.
    await graphOf(page, [9, 8, 7, 6, 5, 2, ...upTo(30).filter((n) => n > 9)])
    await respond(page, { CommitRow: 5 })
    await emit(page, 26)
    await expect.poll(async () => (await selects(page)).length).toBeGreaterThan(before)
    expect((await events(page, 'select')).at(-1)?.[0]).toMatchObject({ index: 5, id: id(2) })
  })

  // Where a commit is, and the row that's there, are asked for one after the other, and a
  // reload can swap the graph for another in between. The row at that place is then some
  // other commit's (#881).
  test('a commit that comes back is never taken for whatever else is at its row', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, sparse: true, responses: head(id(0)) })
    const grid = page.getByRole('grid', { name: 'Commit history' })
    await grid.focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await expect.poll(async () => (await selects(page)).at(-1)).toBe(id(2))

    // The filter hides commit 2.
    await graphOf(page, upTo(30).filter((n) => n % 3 === 0))
    await respond(page, { CommitRow: -1 })
    await emit(page, 10)
    await expect.poll(async () => (await calls(page, 'CommitRow')).some((c) => c[1] === id(2))).toBe(true)
    await page.waitForTimeout(400)
    const before = (await selects(page)).length

    // The filter goes, and the answer about where commit 2 is comes from the whole history
    // while the rows are still the filter's: row 2 of those is commit 6.
    await respond(page, { CommitRow: 2 })
    await emit(page, 10)
    await page.waitForTimeout(700)
    expect(await selects(page)).not.toContain(id(6))
    expect((await selects(page)).length).toBe(before)

    // The whole history arrives, and commit 2 is selected where it is.
    await graphOf(page, upTo(30))
    await emit(page, 30)
    await expect.poll(async () => (await selects(page)).at(-1)).toBe(id(2))
    expect(await selects(page)).not.toContain(id(6))
  })

  test('without a filter, a commit that is gone gives its place to the next, as before', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, responses: head(id(0)) })
    const grid = page.getByRole('grid', { name: 'Commit history' })
    await grid.focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await expect.poll(async () => (await selects(page)).at(-1)).toBe(id(2))
    const before = (await selects(page)).length
    await respond(page, { CommitRow: -1 })
    await emit(page, 30)
    await expect.poll(async () => (await selects(page)).length).toBeGreaterThan(before)
  })
})

test.describe('the ruler under a filter', () => {
  // How much of the ruler's lower half is drawn on: the hatching is the only thing there.
  const inked = (page: Page) =>
    page.locator('.ruler canvas').evaluate((c: HTMLCanvasElement) => {
      const half = Math.floor(c.height / 2)
      const px = c.getContext('2d')!.getImageData(0, half, c.width, c.height - half).data
      let n = 0
      for (let i = 3; i < px.length; i += 4) if (px[i] > 0) n++
      return n
    })
  const base = { graphRows: 400, responses: { Refs: [], CommitSignatures: [] } }

  test('hatches its foot for the history whose trailers are not read yet', async ({ page }) => {
    await open(page, 'graph', { ...base, highlight: [1] })
    await expect(page.locator('.ruler canvas')).toBeVisible()
    await page.waitForTimeout(400)
    const read = await inked(page)

    await open(page, 'graph', { ...base, highlight: [1], trailersUnread: 0.5 })
    await expect(page.locator('.ruler canvas')).toBeVisible()
    await expect.poll(() => inked(page)).toBeGreaterThan(read + 200)

    // With no filter there's nothing whose matches are unknown: no hatching.
    await open(page, 'graph', { ...base, trailersUnread: 0.5 })
    await expect(page.locator('.ruler canvas')).toBeVisible()
    await page.waitForTimeout(400)
    expect(await inked(page)).toBeLessThanOrEqual(read)
  })
})

test.describe('a commit’s trailers under a filter', () => {
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const trailers = [
    { key: 'Closes', value: '#12' },
    { key: 'Reviewed-by', value: 'Sam Ortiz <sam@tidewater.example>' },
    { key: 'Reviewed-by', value: 'Sarah Lind <sarah@tidewater.example>' },
  ]
  const commit = { id: id(0), subject: 'Commit 0', body: 'The long story.', trailers, parents: ['a'.repeat(40)], author: person, committer: person }
  const pick = { key: 'reviewed-by', value: 'person:Sam Ortiz <sam@tidewater.example>', keyLabel: 'Reviewed-by', valueLabel: 'Sam Ortiz <sam@tidewater.example>', kind: 'person' }
  const scenario = (filter: object, lines: object[]) => ({
    graphRows: 3,
    responses: {
      Refs: [],
      CommitSignatures: [],
      CommitNotes: [],
      WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
      CommitDetail: { commit, files: [] },
      CommitRow: 1,
      TrailerFilter: filter,
      TrailerFilterRows: { rows: [1], total: 1 },
      TrailerFilterLines: lines,
    },
  })
  const ROW = 78
  const openTrailers = async (page: Page, s: object) => {
    await open(page, 'repoView', s)
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + ROW + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    await page.getByRole('tab', { name: 'Trailers', exact: true }).click()
  }

  test('marks the line the filter picked, grouped or as written', async ({ page }) => {
    await openTrailers(page, scenario({ picks: [pick], only: false, kept: 1, total: 3 }, [trailers[1]]))
    await expect.poll(() => calls(page, 'TrailerFilterLines')).toContainEqual(['/r', id(0)])
    const picked = page.locator('.trailer.picked')
    await expect(picked).toHaveCount(1)
    await expect(picked).toContainText('Sam Ortiz')
    await expect(picked).toHaveAttribute('title', 'The smart filter picked this')
    // Sarah reviewed it too, and isn't what the filter is for.
    await expect(page.locator('.trailer').filter({ hasText: 'Sarah Lind' })).not.toHaveClass(/picked/)

    await page.getByRole('button', { name: /Grouped|Raw/ }).click()
    await expect(page.locator('dl.as-written .trailer.picked')).toHaveCount(1)
    await expect(page.locator('dl.as-written .trailer.picked')).toContainText('Sam Ortiz')
  })

  test('marks nothing, and asks nothing, with no filter', async ({ page }) => {
    await openTrailers(page, scenario({ picks: [], only: true, kept: 0, total: 3 }, [trailers[1]]))
    await expect(page.locator('.trailer').first()).toBeVisible()
    await expect(page.locator('.trailer.picked')).toHaveCount(0)
    expect(await calls(page, 'TrailerFilterLines')).toEqual([])
  })
})

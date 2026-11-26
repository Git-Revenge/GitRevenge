import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Where a shallow clone's history stops (#1006): the graph ends with a band that says so and
// offers more, its count says "since", blame says "before" for lines from past the edge, and a
// merge whose branches part past it waits for more history. The backend is stubbed.

const SINCE = Date.UTC(2025, 10, 26, 12) / 1000
const shallow = { shallow: { since: SINCE, commits: 3 } }
const workingTree = { status: { files: [] }, identity: { name: 'A', email: 'a@example.com' }, signing: { enabled: false } }
const base = { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: workingTree, CommitRow: 1 }
const respond = (page: Page, responses: Record<string, unknown>) => page.evaluate((r) => (window as unknown as { __respond: (r: Record<string, unknown>) => void }).__respond(r), responses)

test.describe('the end of a shallow history', () => {
  test('the graph ends with the band, and its count says since when', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses: { ...base, CloneShape: shallow }, delays: { FetchMoreHistory: 10_000 } })
    const band = page.getByRole('note', { name: 'Where this history stops' })
    await expect(band).toContainText('history stops here')
    await expect(band).toContainText('a shallow clone · since Nov 26, 2025')
    await expect(band).toContainText('this isn’t where the project began')
    // The bar is squeezed here, so the count is in the search field.
    await expect(page.getByPlaceholder(/^Search /)).toHaveAttribute('placeholder', 'Search 3 commits since Nov 26, 2025')
    await band.getByRole('button', { name: 'Fetch 1,000 commits more' }).click()
    expect(await calls(page, 'FetchMoreHistory')).toEqual([['/r', { commits: 1000, since: 0, all: false }]])
    await expect(band.getByRole('button', { name: 'Fetch the whole history' })).toBeDisabled()
  })

  test('a whole history has neither', async ({ page }) => {
    await open(page, 'repoView', { graphRows: 3, responses: { ...base, CloneShape: {} } })
    await expect(page.getByPlaceholder(/^Search /)).toHaveAttribute('placeholder', 'Search 3 commits')
    await expect(page.getByRole('note', { name: 'Where this history stops' })).toHaveCount(0)
  })

  test('an edge commit says its parent wasn’t fetched', async ({ page }) => {
    const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
    const commit = { id: '0'.repeat(40), subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
    await open(page, 'repoView', { graphRows: 3, responses: { ...base, CloneShape: shallow, CommitDetail: { commit, files: [], edge: true } } })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    await expect(page.getByText('parent not fetched')).toBeVisible()
    await expect(page.getByRole('button', { name: /^parent / })).toHaveCount(0)
  })
})

test.describe('blame in a shallow clone', () => {
  test('lines from past the edge say before when, not who', async ({ page }) => {
    const line = (commit: string, text: string, edge: boolean) => ({ commit: commit.repeat(40), author: 'Ana Cruz', time: SINCE, summary: 'S', line: text, origLine: 1, edge })
    await open(page, 'blame', { responses: { BlameNeeds: { partial: false, versions: 0, missing: [] }, Blame: [line('e', 'old', true), line('e', 'older', true), line('f', 'new', false)] } })
    await expect(page.getByText('before Nov 26, 2025')).toBeVisible()
    await expect(page.locator('.who')).toHaveText(['before Nov 26, 2025', 'Ana Cruz'])
  })
})

test.describe('a merge past the edge', () => {
  const forecast = { into: 'main', alreadyMerged: false, fastForward: false, commits: [], commitCount: 3, intoOnly: 2, files: 5, conflicts: [], unknown: false }

  test('waits for more history, then forecasts', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: { ...forecast, pastEdge: true, commitCount: 0, files: 0 }, CloneShape: shallow }, delays: { FetchMoreHistory: 2500 } })
    const band = page.getByRole('note', { name: 'Where this history stops' })
    await expect(band).toContainText('history stops before where main and feature/metric-units part')
    const mergeButton = page.getByRole('button', { name: 'Merge', exact: true })
    await expect(mergeButton).toBeDisabled()
    await band.getByRole('button', { name: 'Fetch the whole history' }).click()
    expect(await calls(page, 'FetchMoreHistory')).toEqual([['/r', { commits: 0, since: 0, all: true }]])
    await respond(page, { PreviewMerge: forecast })
    await expect(page.getByText('No conflicts expected')).toBeVisible()
    await expect(page.locator('button[type="submit"]')).toBeEnabled()
  })
})

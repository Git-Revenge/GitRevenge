import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Reads across many commits in a partial clone (#1010): git would download a file at a time
// behind them, so they read with downloads off. What they need comes as one download, shown
// with Stop: by itself for a commit's line counts, a merge forecast and a document's
// pictures; only when asked for Insights' lines and for following a rename further back. The
// backend is stubbed: these check what the views ask of it and what they say.

const IDS = ['b2'.repeat(20), 'c3'.repeat(20)]

const respond = (page: Page, responses: Record<string, unknown>) => page.evaluate((r) => (window as unknown as { __respond: (r: Record<string, unknown>) => void }).__respond(r), responses)
const downloads = async (page: Page) => ((await calls(page, 'DownloadFiles')) as [string, string, string[]][]).map(([repo, , ids]) => [repo, ids])

test.describe('a commit’s line counts', () => {
  const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
  const commit = { id: '0'.repeat(40), subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
  const workingTree = { status: { files: [] }, identity: { name: 'A', email: 'a@example.com' }, signing: { enabled: false } }
  const named = { commit, files: [{ status: 'M', path: 'internal/tides/forecast.go', added: 0, deleted: 0 }], countsMissing: IDS }
  const counted = { commit, files: [{ status: 'M', path: 'internal/tides/forecast.go', added: 12, deleted: 3 }] }
  const base = { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: workingTree, CommitRow: 1 }
  const openCommit = async (page: Page, scenario: object) => {
    await open(page, 'repoView', { graphRows: 3, ...scenario })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
  }

  test('names the files at once, then downloads what the counts need by itself', async ({ page }) => {
    // Long enough to answer differently before it's in.
    await openCommit(page, { responses: { ...base, CommitDetail: named }, delays: { DownloadFiles: 2500 } })
    await page.getByRole('tab', { name: 'Files', exact: true }).click()
    await expect(page.getByText('Downloading this commit’s files for their line counts…')).toBeVisible()
    await expect(page.locator('.files .file-row', { hasText: 'forecast.go' })).toBeVisible()
    expect(await downloads(page)).toEqual([['/r', IDS]])
    // In, the panel asks again and has its counts.
    await respond(page, { CommitDetail: counted })
    const row = page.locator('.files .file-row', { hasText: 'forecast.go' })
    await expect(row.locator('.added')).toHaveText('+12')
    await expect(row.locator('.deleted')).toHaveText('−3')
    await expect(page.getByText(/Downloading this commit’s files/)).toHaveCount(0)
  })

  test('Stop leaves the names, and Download starts it again', async ({ page }) => {
    await openCommit(page, { responses: { ...base, CommitDetail: named }, delays: { DownloadFiles: 10_000 } })
    await page.getByRole('tab', { name: 'Files', exact: true }).click()
    await page.getByRole('button', { name: 'Stop' }).click()
    await expect(page.getByText('Line counts need this commit’s files, which this partial clone hasn’t got yet.')).toBeVisible()
    const [[, token]] = (await calls(page, 'DownloadFiles')) as [string, string][]
    expect(await calls(page, 'CancelDownload')).toEqual([[token]])
    await page.getByRole('button', { name: 'Download' }).click()
    await expect(page.getByText(/Downloading this commit’s files/)).toBeVisible()
    expect(await calls(page, 'DownloadFiles')).toHaveLength(2)
  })
})

test.describe('a merge forecast', () => {
  const forecast = { into: 'main', alreadyMerged: false, fastForward: false, commits: [], commitCount: 3, intoOnly: 2, files: 5, conflicts: [], unknown: false }
  const needs = { ...forecast, missing: IDS, commitCount: 0, files: 0 }

  test('downloads the files both sides changed, shown, then forecasts', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: needs }, delays: { DownloadFiles: 2500 } })
    await expect(page.getByText('This partial clone hasn’t got the files both sides changed yet. Downloading…')).toBeVisible()
    await expect(page.getByRole('button', { name: /^Merge/ })).toBeDisabled()
    expect(await downloads(page)).toEqual([['/r', IDS]])
    await respond(page, { PreviewMerge: forecast })
    await expect(page.getByText('No conflicts expected')).toBeVisible()
    expect(await calls(page, 'PreviewMerge')).toHaveLength(2)
  })

  test('stopped, it goes on without a forecast and still merges', async ({ page }) => {
    await open(page, 'previewDialog', { responses: { PreviewMerge: needs }, delays: { DownloadFiles: 10_000 } })
    await page.getByRole('button', { name: 'Stop' }).click()
    await expect(page.getByText("Conflicts couldn't be checked ahead of time")).toBeVisible()
    await expect(page.getByRole('button', { name: 'Download' })).toBeVisible()
    await page.getByRole('button', { name: /^Merge/ }).click()
    expect(await calls(page, 'Integrate')).toEqual([['/r', 'merge', ['feature/metric-units', 'no-ff']]])
  })
})

test.describe('Insights in a partial clone', () => {
  const win = { target: 'origin/main', targetSha: '74cec43b701e879058ce439c48bc400b600b0d2f', asOf: 1793026800, since: 1791849600, until: 1793026800, range: '', kinds: {}, botAuthors: [], references: [] }
  const item = { sha: 'e5'.repeat(20), title: 'Cache readings', author: 'Ana Cruz', email: 'ana@example.com', committed: 1793000000, merge: false, files: 4, added: 0, removed: 0, refs: [], cycle: 0, revert: false, fixup: false, bot: false }
  const responses = {
    Groups: [],
    InsightsShipped: { window: win, items: [item], linesMissing: 1442 },
    InsightsBranches: { window: win, branches: [] },
    InsightsFetchedAt: 0,
    InsightsThresholds: { stale: 7, longLived: 14, behind: 50, size: 500 },
    InsightsTags: [],
    InsightsReleaseBranches: [],
  }

  test('counts commits and files, and downloads the window’s changes only when asked', async ({ page }) => {
    await open(page, 'insights', { responses, delays: { InsightsDownload: 2500 } })
    const note = page.getByRole('status').filter({ hasText: 'Lines changed aren’t counted' })
    await expect(note).toContainText('it’s a partial clone without the files the lines need. Commits and files are.')
    expect(await calls(page, 'InsightsDownload')).toEqual([])
    await note.getByRole('button', { name: 'Download two weeks of changes' }).click()
    await expect(page.getByText('Downloading two weeks of changes…')).toBeVisible()
    const [[repo, token, inputs]] = (await calls(page, 'InsightsDownload')) as [string, string, { window: string }][]
    expect([repo, inputs.window]).toEqual(['/r', '14d'])
    expect(token).not.toBe('')
    // In, Insights measures again, now with lines.
    const before = (await calls(page, 'InsightsShipped')).length
    await respond(page, { InsightsShipped: { window: win, items: [{ ...item, added: 40, removed: 2 }], linesMissing: 0 } })
    await expect.poll(async () => (await calls(page, 'InsightsShipped')).length).toBeGreaterThan(before)
    await expect(page.getByText(/Lines changed aren’t counted/)).toHaveCount(0)
  })

  test('Stop stops it and says so', async ({ page }) => {
    await open(page, 'insights', { responses, delays: { InsightsDownload: 10_000 } })
    await page.getByRole('button', { name: 'Download two weeks of changes' }).click()
    await page.getByRole('button', { name: 'Stop' }).click()
    await expect(page.getByText(/and the download was stopped/)).toBeVisible()
    const [[, token]] = (await calls(page, 'InsightsDownload')) as [string, string][]
    expect(await calls(page, 'CancelDownload')).toEqual([[token]])
  })
})

test.describe('a file’s history in a partial clone', () => {
  const four = { id: 'f4'.repeat(20), author: 'Ana Cruz', time: 1793000000, subject: 'four', path: 'new.txt', status: 'M' }
  const renamed = { id: 'e3'.repeat(20), author: 'Ana Cruz', time: 1792000000, subject: 'rename and change', path: 'new.txt', oldPath: 'old.txt', status: 'R' }

  test('says where it stops, and follows the rename further only when asked', async ({ page }) => {
    await open(page, 'fileHistory', { responses: { FileHistory: { commits: [four], stop: { commit: renamed.id, missing: IDS } } }, delays: { DownloadFiles: 2500 } })
    const stop = page.getByRole('status').filter({ hasText: 'The history stops at' })
    await expect(stop).toContainText('The history stops at e3e3e3e, where the file was renamed. Following it further back compares 2 files this partial clone hasn’t downloaded.')
    expect(await calls(page, 'DownloadFiles')).toEqual([])
    await stop.getByRole('button', { name: 'Download and follow' }).click()
    await expect(page.getByText('This partial clone hasn’t got the files this rename compares yet. Downloading…')).toBeVisible()
    expect(await downloads(page)).toEqual([['/r', IDS]])
    await respond(page, { FileHistory: { commits: [four, renamed] } })
    await expect(page.getByText('rename and change')).toBeVisible()
    await expect(page.getByText(/The history stops at/)).toHaveCount(0)
  })
})

test.describe('a document’s pictures in a partial clone', () => {
  const doc = { contents: { path: 'README.md', kind: 'text', old: { exists: true, text: '', size: 0 }, new: { exists: true, text: '# Tidewater\n\n![Tide chart](docs/tide.png)\n\n![Map](docs/map.png)\n', size: 60 } }, diff: null }
  const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII='

  test('show as not downloaded, and come in one download, shown', async ({ page }) => {
    await open(page, 'diff', {
      diffTarget: { mode: 'commit', commit: 'c0ffee'.padEnd(40, '0'), file: 'README.md' },
      responses: { FileDiff: doc, FileImage: { url: '', missing: [IDS[0]] } },
      delays: { DownloadFiles: 2500 },
    })
    await page.getByRole('button', { name: 'File', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Tidewater' })).toBeVisible()
    // The stub gives both pictures the same version: one to download, in one request.
    await expect(page.getByText('This partial clone hasn’t got a picture this preview shows yet. Downloading…')).toBeVisible()
    await expect(page.getByRole('img', { name: 'Tide chart' })).toHaveAttribute('src', /^data:image\/svg\+xml/)
    await expect(page.getByRole('img', { name: 'Map' })).toHaveAttribute('src', /^data:image\/svg\+xml/)
    await expect.poll(() => downloads(page)).toEqual([['/r', [IDS[0]]]])
    await respond(page, { FileImage: { url: PNG } })
    await expect(page.getByRole('img', { name: 'Tide chart' })).toHaveAttribute('src', PNG)
    await expect(page.getByText(/pictures this preview shows/)).toHaveCount(0)
  })
})

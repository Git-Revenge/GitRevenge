import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Downloads in a partial clone (#1003): one file a view opens downloads by itself, with
// Stop; blame, which reads every version of a file, asks first. The backend is stubbed:
// these check what the views ask of it and what they say.

const ID = 'a1'.repeat(20)
const IDS = ['b2'.repeat(20), 'c3'.repeat(20), 'd4'.repeat(20)]

const missing = { contents: { path: 'internal/tides/forecast.go', kind: 'missing', old: { exists: true, text: '', size: 0 }, new: { exists: true, text: '', size: 0 }, missing: [ID] }, diff: null }
const arrived = { contents: { path: 'internal/tides/forecast.go', kind: 'binary', old: { exists: true, text: '', size: 10 }, new: { exists: true, text: '', size: 12 } }, diff: null }

const respond = (page: Page, responses: Record<string, unknown>) => page.evaluate((r) => (window as unknown as { __respond: (r: Record<string, unknown>) => void }).__respond(r), responses)

test.describe('a version a partial clone never downloaded', () => {
  test('downloads by itself, says so, and shows the file when it’s in', async ({ page }) => {
    await open(page, 'diff', { responses: { FileDiff: missing }, delays: { DownloadFiles: 800 } })
    await expect(page.getByText('This partial clone hasn’t got this version of the file yet. Downloading…')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Stop' })).toBeVisible()
    const [[repo, token, ids]] = (await calls(page, 'DownloadFiles')) as [string, string, string[]][]
    expect([repo, ids]).toEqual(['/r', [ID]])
    expect(token).not.toBe('')
    // The download finishes, and the view loads the file again.
    await respond(page, { FileDiff: arrived })
    await expect(page.getByText('Binary file. Sizes: 10 → 12 bytes.')).toBeVisible()
  })

  test('Stop stops it, and Download starts it again', async ({ page }) => {
    await open(page, 'diff', { responses: { FileDiff: missing }, delays: { DownloadFiles: 10_000 } })
    await page.getByRole('button', { name: 'Stop' }).click()
    await expect(page.getByText(/The download was stopped/)).toBeVisible()
    const [[, token]] = (await calls(page, 'DownloadFiles')) as [string, string][]
    expect(await calls(page, 'CancelDownload')).toEqual([[token]])
    await page.getByRole('button', { name: 'Download' }).click()
    await expect(page.getByText(/Downloading…/)).toBeVisible()
    expect(await calls(page, 'DownloadFiles')).toHaveLength(2)
  })

  test('a download that fails says why, and can be tried again', async ({ page }) => {
    await open(page, 'diff', { responses: { FileDiff: missing }, failures: { DownloadFiles: 'Could not resolve host: git.example.com' } })
    await expect(page.getByText('The download failed: Could not resolve host: git.example.com')).toBeVisible()
    await page.getByRole('button', { name: 'Try again' }).click()
    await expect.poll(async () => (await calls(page, 'DownloadFiles')).length).toBe(2)
  })
})

test.describe('blame in a partial clone', () => {
  const line = { commit: 'e5'.repeat(20), author: 'Ana Cruz', time: 1784214240, summary: 'S', line: 'text', origLine: 1 }

  test('asks before downloading the versions it reads, then blames', async ({ page }) => {
    await open(page, 'blame', { responses: { BlameNeeds: { partial: true, versions: 1119, missing: IDS }, BlameDownloading: [line] } })
    const ask = page.getByRole('group', { name: 'Blame a.txt?' })
    await expect(ask).toContainText('this partial clone hasn’t got 3 of its 1,119 versions yet')
    expect(await calls(page, 'Blame')).toEqual([])
    await ask.getByRole('button', { name: 'Download and blame' }).click()
    await expect(page.locator('.who')).toHaveText('Ana Cruz')
    const [[repo, , ids]] = (await calls(page, 'DownloadFiles')) as [string, string, string[]][]
    expect([repo, ids]).toEqual(['/r', IDS])
    expect(await calls(page, 'BlameDownloading')).toEqual([['/r', 'a.txt', '']])
    expect(await calls(page, 'Blame')).toEqual([])
  })

  test('Not now leaves it', async ({ page }) => {
    await open(page, 'blame', { responses: { BlameNeeds: { partial: true, versions: 2, missing: IDS.slice(0, 2) } } })
    const ask = page.getByRole('group', { name: 'Blame a.txt?' })
    await expect(ask).toContainText('hasn’t got any of its 2 versions yet')
    await ask.getByRole('button', { name: 'Not now' }).click()
    expect(await events(page, 'close')).toHaveLength(1)
    expect(await calls(page, 'DownloadFiles')).toEqual([])
  })

  test('a whole clone blames straight away', async ({ page }) => {
    await open(page, 'blame', { responses: { BlameNeeds: { partial: false, versions: 0, missing: [] }, Blame: [line] } })
    await expect(page.locator('.who')).toHaveText('Ana Cruz')
    await expect(page.getByRole('group', { name: 'Blame a.txt?' })).toHaveCount(0)
  })
})

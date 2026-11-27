import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// Save as an archive (#936): what goes in is said before anything is written, every choice is
// one of git archive's, a file already there is replaced only when asked, and a partial clone
// downloads first. The backend is stubbed.

const plan = (more: object = {}) => ({ commit: 'a'.repeat(40), files: 128, bytes: 3 << 20, ignored: 2, subst: 1, missing: [], ...more })
const dialog = (page: Page) => page.getByRole('dialog')
const saved = { file: 'tidewater-v1.1.0.zip', folder: 'A:\\' }

test.describe('Save as an archive', () => {
  test('says what goes in, and saves with git archive’s choices', async ({ page }) => {
    await open(page, 'archive', { responses: { ArchivePlan: plan(), SaveArchive: saved } })
    const d = dialog(page)
    await expect(d.getByRole('heading', { name: 'Save v1.1.0 as an archive' })).toBeVisible()
    await expect(d).toContainText('128 files, 3 MB before compressing; 2 left out by export-ignore; 1 with export-subst placeholders filled in.')
    await expect.poll(async () => (await calls(page, 'ArchivePlan')).at(-1)).toEqual(['A:\\tidewater', 'v1.1.0', ''])
    await expect(d.getByLabel('Everything in')).toHaveValue('tidewater-v1.1.0/')
    await expect(d.getByLabel("The archive's name")).toHaveValue('tidewater-v1.1.0.zip')
    await expect(d.locator('.example')).toHaveText('git archive --format=zip --prefix=tidewater-v1.1.0/ -o tidewater-v1.1.0.zip v1.1.0')

    // The name follows the format until it's typed in.
    await d.getByLabel('tar.gz').check()
    await expect(d.getByLabel("The archive's name")).toHaveValue('tidewater-v1.1.0.tar.gz')
    await d.getByLabel('zip').check()

    await d.getByLabel('Into folder').fill('A:\\')
    await d.getByRole('button', { name: 'Save tidewater-v1.1.0.zip' }).click()
    await expect.poll(async () => (await calls(page, 'SaveArchive')).at(-1)).toEqual(['A:\\tidewater', 'v1.1.0', { format: 'zip', prefix: 'tidewater-v1.1.0/', sub: '' }, 'A:\\', 'tidewater-v1.1.0.zip', false])
    await expect(page.getByText('Saved tidewater-v1.1.0.zip to A:\\')).toBeVisible()
    expect(await events(page, 'close')).toHaveLength(1)
    await page.getByRole('button', { name: 'Show in folder' }).click()
    expect(await calls(page, 'ShowFolder')).toEqual([['A:\\']])
  })

  test('one folder of it, read again as it’s typed', async ({ page }) => {
    await open(page, 'archive', { archive: { rev: 'feature/offline-cache', label: 'feature/offline-cache' }, responses: { ArchivePlan: plan({ files: 4, ignored: 0, subst: 0 }), SaveArchive: saved } })
    const d = dialog(page)
    await expect(d.getByLabel("The archive's name")).toHaveValue('tidewater-feature-offline-cache.zip')
    await d.getByLabel('Only').fill('src')
    await expect.poll(async () => (await calls(page, 'ArchivePlan')).at(-1)).toEqual(['A:\\tidewater', 'feature/offline-cache', 'src'])
    await expect(d.locator('.example')).toHaveText('git archive --format=zip --prefix=tidewater-feature-offline-cache/ -o tidewater-feature-offline-cache.zip feature/offline-cache -- src')
    await expect(d).toContainText('4 files, 3 MB before compressing.')
  })

  test('a file already there is replaced only when asked', async ({ page }) => {
    await open(page, 'archive', { responses: { ArchivePlan: plan(), SaveArchive: { ...saved, exists: true } } })
    const d = dialog(page)
    await expect(d).toContainText('128 files')
    await d.getByRole('button', { name: 'Save tidewater-v1.1.0.zip' }).click()
    await expect(d.getByRole('alert')).toContainText('tidewater-v1.1.0.zip is already in')
    expect(await events(page, 'close')).toHaveLength(0)
    await d.getByRole('button', { name: 'Replace it' }).click()
    await expect.poll(async () => (await calls(page, 'SaveArchive')).at(-1)?.at(-1)).toBe(true)
  })

  test('a folder export-ignore leaves empty can’t be saved', async ({ page }) => {
    await open(page, 'archive', { responses: { ArchivePlan: plan({ files: 0, ignored: 5, bytes: 0, subst: 0 }) } })
    const d = dialog(page)
    await expect(d).toContainText('Nothing goes in: export-ignore in .gitattributes leaves every file out.')
    await expect(d.getByRole('button', { name: /^Save / })).toBeDisabled()
  })

  test('a folder that isn’t one says so', async ({ page }) => {
    await open(page, 'archive', { failures: { ArchivePlan: '“nope” isn’t a folder in that commit.' } })
    const d = dialog(page)
    await expect(d.getByRole('alert')).toHaveText('“nope” isn’t a folder in that commit.')
    await expect(d.getByRole('button', { name: /^Save / })).toBeDisabled()
  })

  test('a partial clone downloads its files first, in one download', async ({ page }) => {
    const missing = ['b'.repeat(40), 'c'.repeat(40)]
    await open(page, 'archive', { responses: { ArchivePlan: plan({ bytes: -1, missing }), DownloadFiles: null, SaveArchive: saved } })
    const d = dialog(page)
    await expect(d).toContainText('128 files; 2 left out by export-ignore')
    await expect(d).toContainText('this partial clone hasn’t got yet. Save downloads them first, in one download.')
    await d.getByRole('button', { name: 'Download and save' }).click()
    await expect.poll(async () => (await calls(page, 'DownloadFiles'))[0]?.[2]).toEqual(missing)
    await expect.poll(async () => (await calls(page, 'SaveArchive')).length).toBe(1)
  })
})

import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// Storage, in the Repository panel (#937): what the objects take and how they're kept, git
// maintenance on request (suggested only when git's own thresholds say so), and git's
// maintenance schedule. The backend is stubbed.

const base = { Worktrees: [], Submodules: [], CloneShape: {} }
const tidy = { loose: 120, looseBytes: 480 * 1024, packs: 2, packBytes: 3 * 2 ** 30, garbage: 0, garbageBytes: 0, prunePackable: 0, gcAuto: 6700, autoPackLimit: 50, registered: false, bytes: 3 * 2 ** 30 + 480 * 1024, due: false }
const section = (page: Page) => page.getByRole('region', { name: 'Storage' })

test('says what the objects take, and offers maintenance without pressing it', async ({ page }) => {
  await open(page, 'repoPanel', { responses: { ...base, RepoStorage: tidy } })
  const s = section(page)
  await expect(s).toContainText('3.0 GB on disk')
  await expect(s).toContainText('120 loose objects · 2 packs')
  await expect(s).not.toContainText('so git would')
  // Offered, quietly: nothing calls for it.
  await expect(s.getByRole('button', { name: 'Run maintenance' })).toHaveClass(/quiet/)
  await expect(s).toContainText('Not on git’s maintenance schedule')
})

test('suggests maintenance when git’s own thresholds are passed, and runs it with Stop in the toolbar', async ({ page }) => {
  await open(page, 'repoPanel', { responses: { ...base, RepoStorage: { ...tidy, loose: 9000, due: true }, RunMaintenance: null }, delays: { RunMaintenance: 10_000 } })
  const s = section(page)
  await expect(s).toContainText('More than 6,700 objects are loose (gc.auto), so git would pack them.')
  const run = s.getByRole('button', { name: 'Run maintenance' })
  await expect(run).toHaveClass(/secondary/)
  await run.click()
  const toolbar = page.getByRole('toolbar', { name: 'Repository actions' })
  await expect(toolbar.getByRole('status')).toContainText('Running maintenance')
  await expect(run).toBeDisabled()
  await expect.poll(() => calls(page, 'RunMaintenance')).toEqual([['/r']])
})

test('puts it on git’s schedule after saying what that does, and takes it off', async ({ page }) => {
  await open(page, 'repoPanel', { responses: { ...base, RepoStorage: tidy, ScheduleMaintenance: null } })
  const s = section(page)
  await s.getByRole('button', { name: 'Put it on the schedule…' }).click()
  const d = page.getByRole('dialog')
  await expect(d).toContainText('your system’s scheduler')
  await d.getByRole('button', { name: 'Put it on the schedule' }).click()
  await expect.poll(() => calls(page, 'ScheduleMaintenance')).toEqual([['/r', true]])
})

test('on the schedule, it says so and can come off', async ({ page }) => {
  await open(page, 'repoPanel', { responses: { ...base, RepoStorage: { ...tidy, registered: true }, ScheduleMaintenance: null } })
  const s = section(page)
  await expect(s).toContainText('On git’s maintenance schedule')
  await expect(s).toContainText('git runs maintenance on it hourly, daily and weekly.')
  await s.getByRole('button', { name: 'Take it off the schedule' }).click()
  await expect.poll(() => calls(page, 'ScheduleMaintenance')).toEqual([['/r', false]])
})

test('stray files in the object folder are named', async ({ page }) => {
  await open(page, 'repoPanel', { responses: { ...base, RepoStorage: { ...tidy, garbage: 1 } } })
  await expect(section(page)).toContainText('1 stray file in the object folder, such as a pack left half written.')
})

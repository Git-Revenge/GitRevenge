import { expect, test } from '@playwright/test'
import { calls, open } from './support'

const tab = (path: string, name: string) => ({ path, name, head: 'main' })

/** A session with repositories open, as the backend would restore it. */
const restored = (tabs: object[], active: string) => ({
  tabs,
  active,
  failed: [],
  configTabId: 'gitrevenge:gitconfig',
  configTab: '',
  insightsTabId: 'gitrevenge:insights',
  insightsTab: '',
  ignoreTabId: 'gitrevenge:ignore',
  ignoreTab: '',
  peopleTabId: 'gitrevenge:people',
  peopleTab: '',
})

const shell = (tabs: object[], active: string, slow: Record<string, number>) => ({
  graphRows: 12,
  delays: slow,
  responses: {
    Environment: { gitVersion: '2.50.0', gitPath: 'git', lfsVersion: '', minVersion: '2.30.0', problem: '' },
    Version: '1.2.3',
    RestoreSession: restored(tabs, active),
    Refs: [],
    CommitSignatures: [],
  },
})

// What a repository's view was still reading when its tab stopped being the one on screen.
// Each of these reads the repository's path again once it has its answer.
const slow = { RepoState: 600, HeadCommit: 600, Refs: 600 }

const selectTab = (page: import('@playwright/test').Page, id: string) => page.evaluate((to) => (window as unknown as { __select: (id: string) => Promise<void> }).__select(to), id)

test.describe('leaving a repository’s tab while it is still reading (#849)', () => {
  // open() fails the test on any uncaught error in the page, which is what these are about.
  test('for the start page throws nothing', async ({ page }) => {
    await open(page, 'app', shell([tab('/r', 'tidewater')], '/r', slow))
    await expect(page.getByRole('navigation', { name: 'Open repositories' }).getByText('tidewater')).toBeVisible()
    await expect.poll(async () => (await calls(page, 'RepoState')).length).toBeGreaterThan(0)

    await selectTab(page, '')
    await expect(page.getByRole('button', { name: 'Start page' })).toHaveClass(/active/)
    // Long enough for every slow answer to arrive at a view that's gone.
    await page.waitForTimeout(1500)
  })

  test('for another repository throws nothing, and shows the other one', async ({ page }) => {
    await open(page, 'app', shell([tab('/r', 'tidewater'), tab('/s', 'saltmarsh')], '/r', slow))
    await expect(page.getByRole('navigation', { name: 'Open repositories' }).getByText('saltmarsh')).toBeVisible()
    await expect.poll(async () => (await calls(page, 'RepoState')).length).toBeGreaterThan(0)

    await selectTab(page, '/s')
    await expect.poll(async () => (await calls(page, 'RepoState')).some((c) => c[0] === '/s')).toBe(true)
    await page.waitForTimeout(1500)
    // Nothing that was asked for after the switch was asked of a repository with no path.
    for (const method of ['RepoState', 'HeadCommit', 'Refs']) {
      expect((await calls(page, method)).filter((c) => c[0] !== '/r' && c[0] !== '/s')).toEqual([])
    }
  })
})

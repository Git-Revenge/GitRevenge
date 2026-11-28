import { expect, test } from '@playwright/test'
import { open } from './support'

// A timeline focused on a folder or file (#1059): the working tree row counts only what's
// changed under it. The backend is stubbed.

const identity = { name: 'A', email: 'a@example.com' }
const file = (path: string) => ({ path, index: '.', workTree: 'M', untracked: false, conflicted: false })
const workingTree = { status: { files: [file('src/alerts/sms.js'), file('src/alerts/gusts.js'), file('README.md')] }, identity, signing: { enabled: false } }
const scenario = (focus: object) => ({ graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: workingTree, GraphFocus: focus } })
const row = (page: import('@playwright/test').Page) => page.locator('button.wip')

test('without a focus, the working tree row counts every change', async ({ page }) => {
  await open(page, 'repoView', scenario({ branch: '', stack: false, path: '' }))
  await expect(row(page).locator('.wip-title')).toHaveText('3 files changed')
})

test('focused on a folder, it counts only the changes under it', async ({ page }) => {
  await open(page, 'repoView', scenario({ branch: '', stack: false, path: 'src/alerts/' }))
  await expect(row(page).locator('.wip-title')).toHaveText('2 files changed in src/alerts/')
  await expect(row(page)).toContainText('Review')
})

test('focused on a folder with nothing changed under it, it says so', async ({ page }) => {
  await open(page, 'repoView', scenario({ branch: '', stack: false, path: 'docs/' }))
  await expect(row(page).locator('.wip-title')).toHaveText('Nothing changed in docs/')
  await expect(row(page)).not.toContainText('Review')
  await expect(row(page)).not.toContainText('clean')
})

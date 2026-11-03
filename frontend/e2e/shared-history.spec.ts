import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open } from './support'

// #473: Undo only changes this copy, and force pushing changes everyone's.
const commit = (id: string, subject: string, merge = false) => ({ id, short: id.slice(0, 7), author: 'Someone Else', subject, merge })
const two = { upstream: 'origin/topic', total: 2, commits: [commit('bbbbbbb1', 'add b'), commit('aaaaaaa1', 'add a')] }

const scenario = (responses: Record<string, unknown>) => ({
  responses: {
    Refs: [branch('topic', { head: true, upstream: 'origin/topic' })],
    ShowingHidden: false,
    UndoState: { undo: 'Merge feature', redo: 'Reset' },
    ...responses,
  },
})
const toolbarButton = (page: Page, name: string) => page.getByRole('button', { name, exact: true }).first()

test.describe('undoing what is already pushed', () => {
  test('goes straight ahead when nothing is', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: { upstream: '', total: 0, commits: null } }))
    await toolbarButton(page, 'Undo').click()
    await expect.poll(() => calls(page, 'Undo')).toHaveLength(1)
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('goes ahead when it can not tell', async ({ page }) => {
    await open(page, 'repoView', { ...scenario({}), failures: { PushedUndo: 'nope' } })
    await toolbarButton(page, 'Undo').click()
    await expect.poll(() => calls(page, 'Undo')).toHaveLength(1)
  })

  test('asks first, naming the commits', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: two }))
    await toolbarButton(page, 'Undo').click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toContainText('origin/topic keeps these 2 commits')
    await expect(dialog).toContainText('add b')
    await expect(dialog).toContainText('Someone Else')
    expect(await calls(page, 'Undo')).toEqual([])
    await dialog.getByRole('button', { name: 'Undo anyway' }).click()
    await expect.poll(() => calls(page, 'Undo')).toHaveLength(1)
    await expect(dialog).toHaveCount(0)
  })

  test('can be cancelled', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: two }))
    await toolbarButton(page, 'Undo').click()
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(await calls(page, 'Undo')).toEqual([])
  })

  test('can revert instead, newest first', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: two }))
    await toolbarButton(page, 'Undo').click()
    await page.getByRole('dialog').getByRole('button', { name: 'Revert' }).click()
    await expect.poll(() => calls(page, 'Integrate')).toHaveLength(2)
    expect(JSON.stringify(await calls(page, 'Integrate'))).toMatch(/bbbbbbb1.*aaaaaaa1/)
    expect(await calls(page, 'Undo')).toEqual([])
  })

  test('does not offer Revert for a merge, or for commits it can not list', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: { ...two, commits: [commit('ccccccc1', 'merge', true), two.commits[0]] } }))
    await toolbarButton(page, 'Undo').click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Revert' })).toHaveCount(0)
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()

    await page.evaluate(() => {
      ;(window as unknown as { __scenario: { responses: { PushedUndo: { total: number } } } }).__scenario.responses.PushedUndo.total = 40
    })
    await toolbarButton(page, 'Undo').click()
    await expect(page.getByRole('dialog')).toContainText('and 38 more')
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Revert' })).toHaveCount(0)
  })

  test('Redo asks too', async ({ page }) => {
    await open(page, 'repoView', scenario({ PushedUndo: two }))
    await toolbarButton(page, 'Redo').click()
    await page.getByRole('dialog').getByRole('button', { name: 'Redo anyway' }).click()
    await expect.poll(() => calls(page, 'Redo')).toHaveLength(1)
  })
})

test.describe('force pushing', () => {
  const forcePush = async (page: Page) => {
    await page.getByRole('button', { name: 'Push options' }).click()
    // A dangerous menu item takes a second press.
    await page.getByRole('menuitem', { name: 'Force push (with lease)' }).click()
    await page.getByRole('menuitem', { name: 'Press again to force push (with lease)' }).click()
  }

  test('goes straight ahead on an ordinary branch when it replaces nothing', async ({ page }) => {
    await open(page, 'repoView', scenario({ ForcePushLoss: { upstream: '', total: 0, commits: null } }))
    await forcePush(page)
    await expect.poll(() => calls(page, 'PushCurrent')).toHaveLength(1)
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('names what it would replace, on any branch', async ({ page }) => {
    await open(page, 'repoView', scenario({ ForcePushLoss: two }))
    await forcePush(page)
    const dialog = page.getByRole('dialog')
    await expect(dialog).toContainText('origin/topic would lose these 2 commits')
    await expect(dialog).toContainText('Someone Else')
    expect(await calls(page, 'PushCurrent')).toEqual([])
    await dialog.getByRole('button', { name: 'Force push anyway' }).click()
    await expect.poll(() => calls(page, 'PushCurrent')).toHaveLength(1)
  })

  test('still names the kind of a protected branch', async ({ page }) => {
    const refs = [branch('main', { head: true, branchKind: 'trunk', upstream: 'origin/main' })]
    await open(page, 'repoView', scenario({ Refs: refs, ForcePushLoss: { upstream: '', total: 0, commits: null } }))
    await forcePush(page)
    await expect(page.getByRole('dialog')).toContainText('main is a Trunk branch')
    await expect(page.getByRole('dialog')).not.toContainText('would lose')
  })
})

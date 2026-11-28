import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open } from './support'

// A timeline focused on a folder or file: the working tree row counts only what's changed
// under it (#1059), and the chip and the ways in (#1060). The backend is stubbed.

const identity = { name: 'A', email: 'a@example.com' }
const file = (path: string) => ({ path, index: '.', workTree: 'M', untracked: false, conflicted: false })
const workingTree = { status: { files: [file('src/alerts/sms.js'), file('src/alerts/gusts.js'), file('README.md')] }, identity, signing: { enabled: false } }
const scenario = (focus: object) => ({ graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: workingTree, GraphFocus: focus } })
const noFocus = { branch: '', stack: false, path: '' }
const row = (page: Page) => page.locator('button.wip')
const menu = (page: Page) => page.getByRole('menu')

test('without a focus, the working tree row counts every change', async ({ page }) => {
  await open(page, 'repoView', scenario(noFocus))
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

test.describe('the chip', () => {
  const graph = (focus: object, outOf = 0) => ({ graphRows: 3, responses: { CommitSignatures: [], Refs: [], GraphFocus: focus, GraphOutOf: outOf } })

  test('names the folder, counts against the whole history, and clears', async ({ page }) => {
    await open(page, 'graph', graph({ branch: '', stack: false, path: 'src/alerts/' }, 1410))
    await expect(page.locator('.focus-name')).toHaveText('Focused on src/alerts/')
    await expect(page.getByText('3 of 1,410 commits')).toBeVisible()
    await page.getByRole('button', { name: 'Stop focusing' }).click()
    await expect.poll(() => calls(page, 'ClearGraphFocus')).toEqual([['/repo']])
  })

  test('names a branch and a folder together', async ({ page }) => {
    await open(page, 'graph', graph({ branch: 'feature/offline-cache', stack: false, path: 'src/alerts/' }, 1410))
    await expect(page.locator('.focus-name')).toHaveText('Focused on feature/offline-cache, in src/alerts/')
    await expect(page.locator('.focus')).toHaveAttribute('title', 'Only the commits that changed something in src/alerts/ on feature/offline-cache are showing')
  })

  test('a branch alone counts as before', async ({ page }) => {
    await open(page, 'graph', graph({ branch: 'topic', stack: false, path: '' }))
    await expect(page.locator('.focus-name')).toHaveText('Focused on topic')
    await expect(page.getByText('3 commits')).toBeVisible()
    expect(await calls(page, 'GraphOutOf')).toEqual([])
  })
})

test.describe('the ways in', () => {
  const runCommand = async (page: Page, id: string) => {
    await expect.poll(() => page.evaluate(() => typeof (window as unknown as { __runCommand?: unknown }).__runCommand)).toBe('function')
    await page.evaluate((c) => (window as unknown as { __runCommand: (id: string) => void }).__runCommand(c), id)
  }

  test('a changed file’s menu, and its folder’s', async ({ page }) => {
    await open(page, 'repoView', scenario(noFocus))
    await page.locator('.lists button.name', { hasText: 'sms.js' }).click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Focus the timeline on sms.js' }).click()
    await expect.poll(() => calls(page, 'SetGraphPath')).toEqual([['/r', 'src/alerts/sms.js']])
    await page.locator('.lists .folder', { hasText: 'alerts' }).first().click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Focus the timeline on src/alerts/' }).click()
    await expect.poll(async () => (await calls(page, 'SetGraphPath')).at(-1)).toEqual(['/r', 'src/alerts/'])
  })

  test('a commit’s file, and its folder heading', async ({ page }) => {
    const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
    // The stubbed graph's first row is commit 0, all zeros.
    const row0 = '0'.repeat(40)
    const c = { id: row0, subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: person, committer: person }
    const files = [
      { status: 'M', path: 'src/alerts/sms.js', oldPath: '', added: 1, deleted: 1 },
      { status: 'M', path: 'README.md', oldPath: '', added: 1, deleted: 0 },
    ]
    await open(page, 'repoView', {
      graphRows: 3,
      responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { ...workingTree, status: { files: [] } }, CommitDetail: { commit: c, files }, CommitRow: 1 },
    })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + 52 + 26)
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    const tab = page.getByRole('tab', { name: 'Files', exact: true })
    if (await tab.count()) await tab.click()
    await page.locator('.files .file-row', { hasText: 'sms.js' }).click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Focus the timeline on sms.js' }).click()
    await expect.poll(() => calls(page, 'SetGraphPath')).toEqual([['/r', 'src/alerts/sms.js']])
    await page.locator('.files .folder', { hasText: 'src/alerts' }).first().click({ button: 'right' })
    await menu(page).getByRole('menuitem', { name: 'Focus the timeline on src/alerts/' }).click()
    await expect.poll(async () => (await calls(page, 'SetGraphPath')).at(-1)).toEqual(['/r', 'src/alerts/'])
  })

  test('the palette’s box suggests folders and files as you type', async ({ page }) => {
    await open(page, 'repoView', {
      ...scenario(noFocus),
      byArg: { FocusSuggestions: { index: 1, answers: { '': ['docs/', 'src/'], 'src/al': ['src/alerts/', 'src/alerts/gusts.js', 'src/alerts/sms.js'] } } },
    })
    await runCommand(page, 'graph.focusPath')
    const dialog = page.getByRole('dialog', { name: 'Focus the timeline on a folder or file' })
    const box = dialog.getByRole('combobox', { name: 'Folder or file' })
    await expect(box).toBeFocused()
    await expect(dialog.getByRole('option')).toHaveText(['docs/', 'src/'])
    await box.fill('src/al')
    await expect(dialog.getByRole('option')).toHaveText(['src/alerts/', 'src/alerts/gusts.js', 'src/alerts/sms.js'])
    await box.press('ArrowDown')
    await box.press('ArrowDown')
    await expect(dialog.getByRole('option', { name: 'src/alerts/gusts.js' })).toHaveAttribute('aria-selected', 'true')
    await box.press('Enter')
    await expect.poll(() => calls(page, 'SetGraphPath')).toEqual([['/r', 'src/alerts/gusts.js']])
    await expect(dialog).toHaveCount(0)
  })

  test('a folder typed without its slash is the folder; a refusal stays in the box', async ({ page }) => {
    await open(page, 'repoView', {
      ...scenario(noFocus),
      byArg: { FocusSuggestions: { index: 1, answers: { 'src/alerts': ['src/alerts/', 'src/alerts/sms.js'] } } },
    })
    await runCommand(page, 'graph.focusPath')
    const dialog = page.getByRole('dialog', { name: 'Focus the timeline on a folder or file' })
    const box = dialog.getByRole('combobox', { name: 'Folder or file' })
    await box.fill('src/alerts')
    await expect(dialog.getByRole('option')).toHaveCount(2)
    await dialog.getByRole('button', { name: 'Focus', exact: true }).click()
    await expect.poll(() => calls(page, 'SetGraphPath')).toEqual([['/r', 'src/alerts/']])
  })

  test('a refused path stays in the box with the reason', async ({ page }) => {
    await open(page, 'repoView', { ...scenario(noFocus), failures: { SetGraphPath: '“../x” isn’t a folder or file inside the repository.' } })
    await runCommand(page, 'graph.focusPath')
    const dialog = page.getByRole('dialog', { name: 'Focus the timeline on a folder or file' })
    await dialog.getByRole('combobox', { name: 'Folder or file' }).fill('../x')
    await expect(dialog).toContainText('No folder or file at the checked-out commit is called that.')
    await dialog.getByRole('button', { name: 'Focus', exact: true }).click()
    await expect(dialog.getByRole('alert')).toContainText('isn’t a folder or file inside the repository')
  })
})

// #1061: a branch whose tip changed nothing under the path is labelled, faded, where its
// history last did.
test.describe('a branch labelled where it last touched the path', () => {
  const id = '0'.repeat(40)
  const graphRefs = { [id]: [branch('fix/gusts'), branch('main', { later: 26, branchKind: 'trunk' })] }
  const focus = { branch: '', stack: false, path: 'src/alerts/' }

  test('is faded, after the refs really there, and its tooltip says how far back its tip is', async ({ page }) => {
    await open(page, 'graph', { graphRows: 5, graphRefs, responses: { Refs: graphRefs[id], CommitSignatures: [], GraphFocus: focus, GraphOutOf: 1410 } })
    await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '5')
    await page.waitForTimeout(150)
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.move(box.x + 30, box.y + 26 + 26)
    const list = page.locator('.ref-expand')
    await expect(list.locator('.chip')).toHaveCount(2)
    const later = list.locator('.chip').nth(1)
    await expect(later.locator('.name')).toHaveText('main · 26 later')
    await expect(later).toHaveClass(/later/)
    await expect(later).toHaveAttribute('title', "main's tip is 26 commits later; none of them touched src/alerts/")
    await expect(list.locator('.chip').first()).not.toHaveClass(/later/)
    await expect(list.locator('.chip').first()).not.toHaveAttribute('title', /.*/)
  })
})

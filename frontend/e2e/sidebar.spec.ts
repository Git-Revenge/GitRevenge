import { expect, test } from '@playwright/test'
import { branch, calls, events, expectOutline, open, remote, tag } from './support'

const REFS = [
  branch('main', { branchKind: 'trunk', behind: 2, upstream: 'origin/main' }),
  branch('feature/one', { branchKind: 'feature', head: true, ahead: 1, upstream: 'origin/one' }),
  branch('feature/two', { branchKind: 'feature', stackParent: 'feature/one' }),
  branch('fix/merged', { stackParentMerged: true, stackParent: 'gone-parent' }),
  branch('fix/typo'),
  branch('old-gone', { gone: true }),
  remote('origin/main'),
  remote('upstream/main'),
  tag('v1.0'),
  tag('v1.10'),
]
const scenario = { responses: { Refs: REFS, ShowingHidden: false, Worktrees: [{ path: 'A:/code/repo', head: 'abc', branch: 'main', main: true, current: true }] } }

test('leads with the checked-out branch, then Trunk, folders and loose branches', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  await expectOutline(page, ['feature/', '  one', '    feature/two', 'main', 'fix/', '  merged', '  typo', 'old-gone'])
})

// #463: every merged branch in a stack said "parent merged".
test('a branch known to be merged says so, once the check has answered', async ({ page }) => {
  await open(page, 'sidebar', { ...scenario, delays: { MergedBranches: 300 }, responses: { ...scenario.responses, MergedBranches: { 'fix/merged': 'main', 'old-gone': 'main' } } })
  const row = page.locator('.ref', { hasText: 'merged' }).filter({ hasNotText: 'old-gone' }).first()
  await expect(page.locator('.ref', { hasText: 'old-gone' })).toContainText('gone')
  await expect(row.locator('.track')).toHaveText(/parent merged/)
  await expect(page.locator('.ref', { hasText: 'old-gone' }).locator('.track')).toContainText('merged')
  await expect(row.locator('.track')).not.toContainText('parent')
})

test('without an answer, statuses stay as they were', async ({ page }) => {
  await open(page, 'sidebar', { ...scenario, failures: { MergedBranches: 'nope' } })
  await expect(page.locator('.ref', { hasText: 'old-gone' }).locator('.track')).toContainText('gone')
  await expect(page.locator('.track', { hasText: 'parent merged' })).toBeVisible()
})

test('mutes every branch but the checked-out one', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  const color = (name: string) => page.locator('.ref', { hasText: name }).first().evaluate((e) => getComputedStyle(e).color)
  expect(await color('one')).not.toBe(await color('typo'))
})

test('shows one status per row, and swaps it for More on hover', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  const row = page.locator('[role=treeitem]', { hasText: 'merged' })
  await expect(row.locator('.track')).toHaveCount(1)
  await expect(row.locator('.track')).toHaveCSS('opacity', '1')
  await row.hover()
  await expect(row.locator('.more')).toHaveCSS('opacity', '1')
  await expect(row.locator('.track')).toHaveCSS('opacity', '0')
  await row.locator('.more').click()
  expect((await events(page, 'context')).map((e) => e[0])).toEqual(['fix/merged'])
})

test('the rail switches panels, and a filter stays on across them', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  await page.getByRole('searchbox').fill('v1.1')
  await expect(page.getByRole('button', { name: 'Tags, 1 match' })).toBeVisible()
  await expect(page.locator('.scroll:not([hidden]) .empty')).toContainText('Tags has 1')
  await page.getByRole('button', { name: 'Tags, 1 match' }).click()
  await expectOutline(page, ['v1.10'])
  await expect(page.getByRole('searchbox')).toHaveValue('v1.1')
})

test('Needs care keeps only what needs it', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  await page.getByRole('button', { name: /Needs care/ }).click()
  await expectOutline(page, ['fix/', '  merged', 'old-gone'])
})

test('remembers the panel and folders across a reload', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  await page.getByRole('button', { name: 'Tags', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Tags' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Tags' })).toBeVisible()
})

test('dragging the edge resizes the panel, and double-clicking resets it', async ({ page }) => {
  await open(page, 'sidebar', scenario)
  const panel = page.locator('.panel')
  const start = (await panel.boundingBox())!.width
  const edge = (await page.locator('.edge').boundingBox())!
  await page.mouse.move(edge.x + 3, edge.y + 100)
  await page.mouse.down()
  await page.mouse.move(edge.x + 63, edge.y + 100)
  await page.mouse.up()
  expect((await panel.boundingBox())!.width).toBe(start + 60)
  await page.locator('.edge').dblclick()
  expect((await panel.boundingBox())!.width).toBe(start)
})

test.describe('keyboard', () => {
  const active = (page: import('@playwright/test').Page) =>
    page.evaluate(() => {
      const list = document.activeElement!
      return document.getElementById(list.getAttribute('aria-activedescendant') ?? '')?.querySelector('.name')?.textContent
    })

  test('the list is one Tab stop, and arrows move through it', async ({ page }) => {
    await open(page, 'sidebar', scenario)
    await page.getByRole('tree').focus()
    expect(await active(page)).toBe('feature')
    await page.keyboard.press('ArrowDown')
    expect(await active(page)).toBe('one')
    await page.keyboard.press('ArrowLeft')
    expect(await active(page)).toBe('feature')
    await page.keyboard.press('ArrowLeft')
    await expectOutline(page, ['feature/', 'main', 'fix/', '  merged', '  typo', 'old-gone'])
    await page.keyboard.press('ArrowRight')
    await expectOutline(page, ['feature/', '  one', '    feature/two', 'main', 'fix/', '  merged', '  typo', 'old-gone'])
  })

  test('Enter shows, Space checks out, Menu opens the menu', async ({ page }) => {
    await open(page, 'sidebar', scenario)
    await page.getByRole('tree').focus()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('Enter')
    await page.keyboard.press('Space')
    await page.keyboard.press('Shift+F10')
    expect(await events(page, 'pick')).toEqual([['feature/one']])
    expect(await events(page, 'checkout')).toEqual([['feature/one']])
    expect((await events(page, 'context')).map((e) => e[0])).toEqual(['feature/one'])
  })

  test('typing jumps to a name', async ({ page }) => {
    await open(page, 'sidebar', scenario)
    await page.getByRole('tree').focus()
    await page.keyboard.type('old')
    expect(await active(page)).toBe('old-gone')
  })
})

test('a large repository only draws the rows in view', async ({ page }) => {
  const many = Array.from({ length: 50_000 }, (_, i) => branch(`team${i % 40}/area${i % 400}/b-${i}`))
  await open(page, 'sidebar', { responses: { Refs: many, ShowingHidden: false } })
  await expect(page.locator('.row').first()).toBeVisible()
  expect(await page.locator('.row').count()).toBeLessThan(100)
  await page.getByRole('tree').focus()
  await page.keyboard.press('End')
  await expect(page.locator('[role=treeitem].active')).toBeVisible()
  expect(await page.locator('.row').count()).toBeLessThan(100)
})

test('the Repository panel lists worktrees, and LFS opens its dialog', async ({ page }) => {
  await open(page, 'sidebar', {
    responses: {
      Refs: REFS,
      Worktrees: [
        { path: 'A:/code/repo', head: 'abc', branch: 'main', main: true, current: true },
        { path: 'A:/code/repo-fix', head: 'def', branch: 'fix/typo' },
      ],
      Submodules: [{ path: 'vendor/x', commit: 'abcdef1', state: 'uninitialized' }],
      LFS: { available: true, status: { patterns: [{ pattern: '*.png', source: '.gitattributes' }], files: [{ name: 'a.png', downloaded: true }] } },
    },
  })
  await page.getByRole('button', { name: /^Repository/ }).click()
  await expect(page.locator('.repo-panel')).toContainText('repo-fix')
  await expect(page.locator('.repo-panel')).toContainText('not initialized')
  await page.getByRole('button', { name: /LFS/ }).click()
  await expect(page.getByRole('dialog')).toContainText('*.png')
  await page.getByRole('button', { name: 'Check locks' }).click()
  await expect.poll(() => calls(page, 'LFSLocks')).toHaveLength(1)
})

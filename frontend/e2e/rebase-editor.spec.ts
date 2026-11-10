import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// #596: the interactive rebase editor. The backend is stubbed; these check what the editor
// shows for a plan and what it asks git to do.

const id = (c: string) => c.repeat(40)
const commit = (c: string, subject: string) => ({ commit: id(c), subject, message: subject, action: 'pick' })
const plan = [commit('a', 'Cache the last reading'), commit('b', 'wip'), commit('c', 'fixup! Cache the last reading'), commit('d', 'Fall back to the cached reading'), commit('e', 'debugging')]
const two = [commit('a', 'First'), commit('c', 'Second')]

const clean = { stopsAt: '', conflicts: [], incomplete: false, pushed: false, upstream: '', dropped: [] }
const scenario = (preview: object = clean, commits: object[] = plan, more: object = {}) => ({ responses: { RebasePlan: commits, PreviewRebasePlan: preview, RepoState: { operation: '', conflicts: [] }, ...more } })

const rows = (page: Page) => page.locator('ol.plan > li')
const subjects = (page: Page) => rows(page).locator('.what .subject, .what input.subject').evaluateAll((els) => els.map((e) => (e as HTMLInputElement).value ?? e.textContent))
const select = (page: Page, subject: string) => page.getByRole('button', { name: new RegExp(`^What to do with ${subject}:`) })
const choose = async (page: Page, subject: string, option: string) => {
  await select(page, subject).click()
  await page.getByRole('option', { name: new RegExp(`^${option}`) }).click()
}
const started = async (page: Page) => (await calls(page, 'InteractiveRebase'))[0] as [string, string, { subject: string; action: string; message: string }[]]

test('lists the commits oldest first and says what the branch will hold', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario())
  await expect(page.getByRole('heading', { name: /Rewrite 5 commits/ })).toBeVisible()
  await expect(page.getByText('oldest first')).toBeVisible()
  expect(await subjects(page)).toEqual(plan.map((c) => c.subject))
  const after = page.getByRole('complementary', { name: 'What the branch will look like' })
  await expect(after.getByRole('heading')).toHaveText('5 commits, as now')
  await expect(after.getByText('No conflicts expected in this order.')).toBeVisible()
  await expect(after.getByText("These commits aren't pushed")).toBeVisible()
  await expect(page.getByRole('button', { name: 'Reset the plan' })).toBeDisabled()
})

test('the select says what each choice does, and its choice shows in the outcome', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario({ ...clean, dropped: [{ commit: id('e'), files: ['src/feed.js'], fileCount: 1, added: 3, deleted: 1 }] }))
  await select(page, 'debugging').click()
  const menu = page.getByRole('listbox')
  await expect(menu.getByRole('option')).toHaveCount(5)
  await expect(menu.getByRole('option', { name: /Squash/ })).toContainText('fold into the one above, keep both messages')
  await expect(menu.getByRole('option', { name: /Keep/ })).toHaveAttribute('aria-selected', 'true')
  await menu.getByRole('option', { name: /Drop/ }).click()
  await expect(menu).toHaveCount(0)

  await expect(rows(page).nth(4)).toContainText('its changes to src/feed.js are thrown away')
  const after = page.getByRole('complementary')
  await expect(after.getByRole('heading')).toHaveText('4 commits, where there were 5')
  await expect(after).toContainText('1 commit dropped')
  await expect(after).toContainText('4 lines in src/feed.js go with it.')
  await expect(page.getByRole('button', { name: /Rewrite as 4 commits/ })).toBeEnabled()
})

test('Fold it in for me moves a fixup under the commit it names', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario())
  await expect(page.getByText('1 commit is a fixup for another.')).toBeVisible()
  await page.getByRole('button', { name: 'Fold it in for me' }).click()
  expect(await subjects(page)).toEqual(['Cache the last reading', 'fixup! Cache the last reading', 'wip', 'Fall back to the cached reading', 'debugging'])
  await expect(rows(page).nth(1)).toContainText('folds into the commit above, message discarded')
  await expect(page.getByText('1 commit is a fixup for another.')).toHaveCount(0)
  const after = page.getByRole('complementary')
  await expect(after.getByText('2 commits in one')).toBeVisible()
  await expect(after.locator('.tag', { hasText: 'folded' })).toBeVisible()

  await page.getByRole('button', { name: /Rewrite as 4 commits/ }).click()
  const [path, base, items] = await started(page)
  expect([path, base]).toEqual(['/r', id('b')])
  expect(items.map((i) => `${i.action} ${i.subject}`)).toEqual(['pick Cache the last reading', 'fixup fixup! Cache the last reading', 'pick wip', 'pick Fall back to the cached reading', 'pick debugging'])
  expect(await events(page, 'close')).toHaveLength(1)
})

test('a letter sets what happens to the focused commit, and Alt with an arrow moves it', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario())
  await rows(page).nth(4).focus()
  await page.keyboard.press('d')
  await expect(select(page, 'debugging')).toHaveText(/Drop/)
  await page.keyboard.press('p')
  await expect(select(page, 'debugging')).toHaveText(/Keep/)

  await page.keyboard.press('Alt+ArrowUp')
  expect((await subjects(page)).slice(3)).toEqual(['debugging', 'Fall back to the cached reading'])
  // Focus goes with the commit, so it can keep moving.
  await page.keyboard.press('Alt+ArrowUp')
  expect((await subjects(page))[2]).toBe('debugging')
  await page.keyboard.press('Alt+ArrowDown')
  await page.keyboard.press('Alt+ArrowDown')
  expect(await subjects(page)).toEqual(plan.map((c) => c.subject))
})

test('a reword edits the subject in place, Esc puts it back, and the new message is sent', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario())
  await choose(page, 'wip', 'Reword')
  const field = page.getByLabel('New subject for wip')
  await expect(field).toBeFocused()
  await field.fill('Something else')
  await field.press('Escape')
  await expect(field).toHaveValue('wip')

  await field.fill('Keep readings for an hour')
  await field.press('Enter')
  const after = page.getByRole('complementary')
  await expect(after.getByText('Keep readings for an hour')).toBeVisible()
  await expect(after.getByText('was “wip”')).toBeVisible()
  await expect(after.locator('.tag', { hasText: 'reworded' })).toBeVisible()

  // Ctrl+Enter opens the whole message.
  await field.press('Control+Enter')
  await page.getByLabel('New message for wip').fill('Keep readings for an hour\n\nSo a dropped station still shows something.')
  await page.getByRole('button', { name: /Rewrite as 5 commits/ }).click()
  const [, , items] = await started(page)
  expect(items[1]).toMatchObject({ action: 'reword', message: 'Keep readings for an hour\n\nSo a dropped station still shows something.' })
})

test('says where this order would stop, and asks again when the order changes', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario({ ...clean, stopsAt: id('d'), conflicts: [{ path: 'src/cache.js', places: 2 }] }))
  const after = page.getByRole('complementary')
  await expect(after.getByText('It will stop at Fall back to the cached reading')).toBeVisible()
  await expect(after.getByText('src/cache.js')).toBeVisible()
  await expect(after.getByText('2 places')).toBeVisible()
  await expect.poll(async () => (await calls(page, 'PreviewRebasePlan')).length).toBe(1)

  await rows(page).nth(0).focus()
  await page.keyboard.press('Alt+ArrowDown')
  await expect.poll(async () => (await calls(page, 'PreviewRebasePlan')).length).toBe(2)
  const asked = (await calls(page, 'PreviewRebasePlan'))[1] as [string, string, { subject: string }[]]
  expect(asked[2].map((i) => i.subject).slice(0, 2)).toEqual(['wip', 'Cache the last reading'])
})

test('warns when the branch is already pushed', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario({ ...clean, pushed: true, upstream: 'origin/feature/offline-cache' }))
  await expect(page.getByText('This branch is already pushed')).toBeVisible()
  await expect(page.getByText('origin/feature/offline-cache')).toBeVisible()
})

test('a fold with nothing above it stops the plan, and Reset puts every row back', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario(clean, two))
  await choose(page, 'First', 'Fixup')
  await expect(page.getByRole('status').first()).toHaveText('First has nothing above it to fold into. Move it down, or keep it.')
  await expect(page.getByRole('button', { name: /Rewrite as/ })).toBeDisabled()
  await page.getByRole('button', { name: 'Reset the plan' }).click()
  await expect(select(page, 'First')).toHaveText(/Keep/)
  await expect(page.getByRole('button', { name: /Rewrite as 2 commits/ })).toBeEnabled()
})

test('dragging a commit by its grip reorders the plan', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario())
  const grip = rows(page).nth(4).locator('.grip')
  const from = (await grip.boundingBox())!
  const to = (await rows(page).nth(1).boundingBox())!
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2)
  await page.mouse.down()
  await page.mouse.move(to.x + 40, to.y + to.height / 2, { steps: 12 })
  await page.mouse.up()
  expect(await subjects(page)).toEqual(['Cache the last reading', 'debugging', 'wip', 'fixup! Cache the last reading', 'Fall back to the cached reading'])
})

test('closes when the rebase stops on a conflict, and stays open on any other failure', async ({ page }) => {
  await open(page, 'rebaseEditor', { ...scenario(clean, two, { RepoState: { operation: 'rebase', conflicts: ['a.txt'] } }), failures: { InteractiveRebase: 'The rebase stopped on a conflict.' } })
  await page.getByRole('button', { name: /Rewrite as 2 commits/ }).click()
  await expect.poll(() => events(page, 'close')).toHaveLength(1)

  await open(page, 'rebaseEditor', { ...scenario(clean, two), failures: { InteractiveRebase: 'Your local changes would be overwritten.' } })
  await page.getByRole('button', { name: /Rewrite as 2 commits/ }).click()
  await expect(page.getByRole('alert')).toHaveText('Your local changes would be overwritten.')
  expect(await events(page, 'close')).toHaveLength(0)
})

// #387: an empty list counted as every commit being dropped.
test('warns about nothing while the commits load', async ({ page }) => {
  await open(page, 'rebaseEditor', { delays: { RebasePlan: 500 }, responses: { RebasePlan: two, PreviewRebasePlan: clean } })
  await expect(page.getByText('Loading commits…')).toBeVisible()
  await expect(page.getByRole('status')).toHaveCount(0)
  await expect(page.getByRole('button', { name: /Rewrite as/ })).toBeDisabled()
  await expect(page.getByRole('heading', { name: /Rewrite 2 commits/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Rewrite as 2 commits/ })).toBeEnabled()
})

test('still warns when every commit really is dropped', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario(clean, two))
  await choose(page, 'First', 'Drop')
  await choose(page, 'Second', 'Drop')
  await expect(page.getByRole('status').first()).toHaveText('Every commit is dropped. Use Reset instead.')
})

test('says so when there is nothing to rebase', async ({ page }) => {
  await open(page, 'rebaseEditor', scenario(clean, []))
  await expect(page.getByRole('status')).toHaveText('There are no commits to rebase onto this one.')
})

test('keeps its buttons on one line in a narrow column, with the outcome underneath', async ({ page }) => {
  await page.setViewportSize({ width: 560, height: 700 })
  await open(page, 'rebaseEditor', scenario(clean, two))
  const box = async (name: string | RegExp) => (await page.getByRole('button', { name }).first().boundingBox())!
  const middle = async (name: string | RegExp) => {
    const b = await box(name)
    return b.y + b.height / 2
  }
  expect(await middle(/Rewrite as/)).toBeCloseTo(await middle('Cancel'), 0)
  expect(await middle(/Rewrite as/)).toBeCloseTo(await middle('Reset the plan'), 0)
  const go = await box(/Rewrite as/)
  expect(go.x + go.width).toBeLessThanOrEqual(560)
  const list = (await page.locator('ol.plan').boundingBox())!
  const after = (await page.getByRole('complementary').boundingBox())!
  expect(after.y).toBeGreaterThanOrEqual(list.y + list.height - 1)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

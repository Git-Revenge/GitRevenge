import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// The clone dialog's Options (#1002): every default is what plain `git clone URL` does, and
// each choice is one of git's own options. The backend is stubbed: these check what reaches it.

const URL = 'https://git.example.com/kernel/linux.git'
const plain = { depth: 0, since: 0, partial: false, branch: '', submodules: false }

const dialog = (page: Page) => page.getByRole('dialog')
const summary = (page: Page) => dialog(page).locator('details.options summary')
const cloneButton = (page: Page) => dialog(page).getByRole('button', { name: 'Clone', exact: true })

async function start(page: Page, scenario: object = {}) {
  await open(page, 'cloneDialog', { ...scenario, responses: { HomeDir: 'A:\\src', CloneName: 'linux', ...((scenario as { responses?: object }).responses ?? {}) } })
  await page.getByLabel('Repository URL').fill(URL)
  await page.getByLabel('Folder name').fill('linux')
}

/** The options the dialog sent with its one Clone. */
async function sent(page: Page) {
  const got = await calls(page, 'Clone')
  expect(got).toHaveLength(1)
  return got[0] as unknown[]
}

test.describe('the clone dialog’s options', () => {
  test('start as git clone does, and send nothing more', async ({ page }) => {
    await start(page)
    await expect(summary(page)).toHaveText(/Options\s*· as git clone does/)
    await cloneButton(page).click()
    expect(await sent(page)).toEqual([URL, 'A:\\src', 'linux', plain])
  })

  test('send each choice, and the folded line says what’s chosen', async ({ page }) => {
    await start(page)
    await summary(page).click()
    await dialog(page).getByLabel('How many commits').fill('50')
    await dialog(page).getByRole('radio', { name: 'Only when something needs them' }).check()
    await dialog(page).getByLabel('Which branch').fill('dev')
    await dialog(page).getByRole('checkbox', { name: 'Clone them too' }).check()
    // Typing in a choice's box picks that choice.
    await expect(dialog(page).getByRole('radio', { name: 'The last' })).toBeChecked()
    await expect(dialog(page).getByRole('radio', { name: 'Only', exact: true })).toBeChecked()
    await expect(summary(page)).toHaveText(/the last 50 commits · contents when needed · only dev · with submodules/)
    await expect(dialog(page).getByText(/A shallow clone/)).toBeVisible()
    await expect(dialog(page).getByText(/A partial clone/)).toBeVisible()
    await cloneButton(page).click()
    expect((await sent(page))[3]).toEqual({ depth: 50, since: 0, partial: true, branch: 'dev', submodules: true })
  })

  test('since a day sends the start of that day', async ({ page }) => {
    await start(page)
    await summary(page).click()
    await dialog(page).getByLabel('Since which day').fill('2025-11-26')
    await expect(dialog(page).getByRole('radio', { name: 'Since' })).toBeChecked()
    await expect(summary(page)).toHaveText(/since Nov 26, 2025/)
    await cloneButton(page).click()
    const since = await page.evaluate(() => Math.floor(new Date(2025, 10, 26).getTime() / 1000))
    expect((await sent(page))[3]).toEqual({ ...plain, since })
  })

  test('only one branch lists the server’s branches and starts on its default', async ({ page }) => {
    await start(page, { responses: { CloneBranches: { branches: ['main', 'release/7.4'], default: 'main' } } })
    await summary(page).click()
    await dialog(page).getByRole('radio', { name: 'Only', exact: true }).check()
    await expect(dialog(page).getByLabel('Which branch')).toHaveValue('main')
    expect(await calls(page, 'CloneBranches')).toEqual([[URL, 'A:\\src', 'linux']])
    const offered = await dialog(page).locator('datalist option').evaluateAll((o) => o.map((x) => (x as HTMLOptionElement).value))
    expect(offered).toEqual(['main', 'release/7.4'])
    await cloneButton(page).click()
    expect((await sent(page))[3]).toEqual({ ...plain, branch: 'main' })
  })

  test('branches that can’t be listed can still be typed', async ({ page }) => {
    await start(page, { failures: { CloneBranches: 'Authentication failed for the server.' } })
    await summary(page).click()
    await dialog(page).getByRole('radio', { name: 'Only', exact: true }).check()
    await expect(dialog(page).getByText(/The branches couldn't be listed: Authentication failed/)).toBeVisible()
    await dialog(page).getByLabel('Which branch').fill('master')
    await cloneButton(page).click()
    expect((await sent(page))[3]).toEqual({ ...plain, branch: 'master' })
  })

  test('a choice that can’t be sent says why and holds the clone', async ({ page }) => {
    await start(page)
    await summary(page).click()
    await dialog(page).getByLabel('How many commits').fill('0')
    await expect(dialog(page).getByRole('status').filter({ hasText: /whole number/ })).toBeVisible()
    await expect(cloneButton(page)).toBeDisabled()
    await dialog(page).getByLabel('How many commits').fill('1')
    await expect(summary(page)).toHaveText(/the last commit/)
    await expect(cloneButton(page)).toBeEnabled()
  })
})

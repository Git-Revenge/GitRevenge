import { expect, test, type Page } from '@playwright/test'
import { calls, events, open } from './support'

// #614: an error is a headline, a line on where that leaves you, next steps, and git's words
// behind Details.
const refused = {
  title: 'Push refused: main only takes changes through a pull request',
  detail: 'github.com turned the push down. Nothing was pushed, and your commits are still on your main.',
  quote: '',
  actions: [
    { kind: 'push-new-branch', label: 'Push to a new branch…', url: '' },
    { kind: 'open-url', label: 'See the rules', url: 'https://github.com/Git-Revenge/GitRevenge/rules?ref=refs%2Fheads%2Fmain' },
  ],
  output: 'remote: error: GH013: Repository rule violations found for refs/heads/main.\nTo github.com:Git-Revenge/GitRevenge.git\n ! [remote rejected] main -> main (push declined due to repository rule violations)\nerror: failed to push some refs',
  hidden: 14,
  full: 'Counting objects: 100% (8/8), done.\nremote: error: GH013: Repository rule violations found for refs/heads/main.',
  path: '/r',
}
const text = `${refused.title}\n${refused.detail}`

const call = (page: Page, method: string, ...args: unknown[]) =>
  page.evaluate(([m, a]) => (window as unknown as { __call: (m: string, ...a: unknown[]) => Promise<unknown> }).__call(m as string, ...(a as unknown[])), [method, args] as const)
const say = (page: Page, message: string) => page.evaluate((m) => (window as unknown as { __say: (m: string) => void }).__say(m), message)
const respond = (page: Page, responses: Record<string, unknown>) => page.evaluate((r) => (window as unknown as { __respond: (r: Record<string, unknown>) => void }).__respond(r), responses)
const banner = (page: Page) => page.getByRole('alert')

test.describe('error banner', () => {
  const failing = { failures: { PushCurrent: text }, responses: { TakeProblem: refused } }

  test('says what failed and what to do, without git output', async ({ page }) => {
    await open(page, 'errorBanner', failing)
    await call(page, 'PushCurrent', '/r', false)
    await expect(banner(page)).toContainText(refused.title)
    await expect(banner(page)).toContainText(refused.detail)
    await expect(banner(page)).not.toContainText('GH013')
    expect(await calls(page, 'TakeProblem')).toEqual([[text]])
    await expect(banner(page).getByRole('button')).toHaveText(['Push to a new branch…', 'See the rules', 'Details', ''])
  })

  test('Details shows what git said, marks the failing lines, and copies all of it', async ({ page }) => {
    await open(page, 'errorBanner', failing)
    await call(page, 'PushCurrent', '/r', false)
    const details = banner(page).getByRole('button', { name: 'Details' })
    await expect(details).toHaveAttribute('aria-expanded', 'false')
    await details.click()
    await expect(details).toHaveAttribute('aria-expanded', 'true')
    await expect(banner(page).locator('pre .line')).toHaveText(refused.output.split('\n').map((l) => l.trim()))
    await expect(banner(page).locator('pre .mark')).toHaveText(['remote: error:', ' ! [remote rejected]', 'error:'])
    await expect(banner(page)).toContainText('What git said, without its 14 lines of progress.')
    await banner(page).getByRole('button', { name: 'Copy everything' }).click()
    await expect(banner(page).getByRole('button', { name: 'Copied' })).toBeVisible()
    expect(await events(page, 'copy')).toEqual([[refused.full]])
    await details.click()
    await expect(banner(page).locator('pre')).toHaveCount(0)
  })

  test('See the rules opens the page the server named', async ({ page }) => {
    await open(page, 'errorBanner', failing)
    await call(page, 'PushCurrent', '/r', false)
    await banner(page).getByRole('button', { name: 'See the rules' }).click()
    expect(await events(page, 'open-url')).toEqual([[refused.actions[1].url]])
    await expect(banner(page)).toBeVisible()
  })

  test('Push to a new branch starts one here and pushes it', async ({ page }) => {
    await open(page, 'errorBanner', failing)
    await call(page, 'PushCurrent', '/r', false)
    await banner(page).getByRole('button', { name: 'Push to a new branch…' }).click()
    const dialog = page.getByRole('dialog', { name: 'Push to a new branch' })
    await dialog.getByLabel('Branch name').fill('readme-text')
    await dialog.getByRole('button', { name: 'Create and push' }).click()
    await expect(dialog).toHaveCount(0)
    await expect.poll(() => calls(page, 'PushBranch')).toEqual([['/r', 'readme-text']])
    expect(await calls(page, 'BranchAction')).toEqual([['/r', 'create', ['readme-text', '', 'true']]])
    await expect(banner(page)).toHaveCount(0)
  })

  test('a name git refuses keeps the dialog open and pushes nothing', async ({ page }) => {
    await open(page, 'errorBanner', { ...failing, failures: { ...failing.failures, BranchAction: "That name isn't allowed by git." } })
    await call(page, 'PushCurrent', '/r', false)
    await banner(page).getByRole('button', { name: 'Push to a new branch…' }).click()
    const dialog = page.getByRole('dialog', { name: 'Push to a new branch' })
    await dialog.getByLabel('Branch name').fill('bad name')
    await dialog.getByRole('button', { name: 'Create and push' }).click()
    await expect(dialog).toContainText("That name isn't allowed by git.")
    expect(await calls(page, 'PushBranch')).toEqual([])
  })

  test('Try again runs what failed once more', async ({ page }) => {
    const unreachable = { ...refused, title: "Fetch failed: couldn't reach github.com", detail: 'Check your network connection.', actions: [{ kind: 'retry', label: 'Try again', url: '' }] }
    await open(page, 'errorBanner', { failures: { Fetch: `${unreachable.title}\n${unreachable.detail}` }, responses: { TakeProblem: unreachable } })
    await call(page, 'Fetch', '/r', '')
    await banner(page).getByRole('button', { name: 'Try again' }).click()
    await expect.poll(() => calls(page, 'Fetch')).toEqual([
      ['/r', ''],
      ['/r', ''],
    ])
    await expect(banner(page)).toContainText("couldn't reach github.com")
  })

  test('Pull, then push runs the toolbar commands in turn', async ({ page }) => {
    const behind = { ...refused, title: "Push refused: github.com has commits you don't have", detail: 'Pull first, then push again. Nothing was pushed.', actions: [{ kind: 'pull-then-push', label: 'Pull, then push', url: '' }, { kind: 'force-push', label: 'Force push…', url: '' }] }
    await open(page, 'errorBanner', { toolbar: true, failures: { PushCurrent: `${behind.title}\n${behind.detail}` }, responses: { TakeProblem: behind } })
    await call(page, 'PushCurrent', '/r', false)
    await expect(banner(page).getByRole('button')).toHaveText(['Pull, then push', 'Force push…', 'Details', ''])
    // The remote takes the push once the pull is in.
    await banner(page).getByRole('button', { name: 'Pull, then push' }).click()
    await expect.poll(() => calls(page, 'Pull')).toEqual([['/r', '']])
    await expect.poll(async () => (await calls(page, 'PushCurrent')).length).toBe(2)
  })

  test('an error that is only text shows its first line, with the rest behind Details', async ({ page }) => {
    await open(page, 'errorBanner')
    await say(page, 'Nothing is staged.')
    await expect(banner(page)).toContainText('Nothing is staged.')
    await expect(banner(page).getByRole('button')).toHaveText([''])

    await say(page, 'Your local changes would be overwritten by checkout:\n\ta.txt\nCommit or stash them first.')
    await expect(banner(page).locator('.title')).toHaveText('Your local changes would be overwritten by checkout:')
    await expect(banner(page)).not.toContainText('a.txt')
    await banner(page).getByRole('button', { name: 'Details' }).click()
    await expect(banner(page).locator('pre')).toContainText('a.txt')
    await expect(banner(page)).toContainText('What git said.')
  })

  test('a failure the backend has no more to say about is shown as its text', async ({ page }) => {
    await open(page, 'errorBanner', { failures: { Fetch: 'Cancelled.' } })
    await call(page, 'Fetch', '/r', '')
    await expect(banner(page).locator('.title')).toHaveText('Cancelled.')
    await respond(page, {})
    await banner(page).getByRole('button', { name: 'Dismiss' }).click()
    await expect(banner(page)).toHaveCount(0)
  })
})

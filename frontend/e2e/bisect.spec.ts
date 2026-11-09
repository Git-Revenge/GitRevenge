import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// #592: a bisect you can run from the app. The backend is stubbed; these check what the
// interface shows for each state git can report and what it asks git to do.

const id = (c: string) => c.repeat(40)
const person = { name: 'Maya Okafor', email: 'maya@example.com', time: 1793500000 }
const detail = { commit: { id: id('0'), subject: 'Set the alert limit in knots too', body: '', trailers: [], parents: [id('a')], author: person, committer: person }, files: [{ status: 'M', path: 'src/alerts.js', added: 4, deleted: 1 }] }

const running = {
  running: true,
  current: id('0'),
  marks: [
    { commit: id('f'), subject: 'Ignore log files', term: 'bad', step: 0 },
    { commit: id('e'), subject: 'Release 0.2.0', term: 'good', step: 0 },
    { commit: id('d'), subject: 'Report wind in metres per second', term: 'bad', step: 1 },
  ],
  steps: 1,
  remaining: 1,
  stepsLeft: 2,
  firstBad: '',
  candidates: [],
  origin: 'main',
  inQuestion: [id('d'), id('0'), id('c')],
  rangeCapped: false,
}

const scenario = (bisect: object | null, more: object = {}) => ({
  graphRows: 3,
  responses: {
    Refs: [],
    CommitSignatures: [],
    CommitNotes: [],
    CommitRow: 0,
    WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } },
    CommitDetail: detail,
    RepoState: bisect ? { operation: 'bisect', step: 0, total: 0, actions: ['abort'], conflicts: [] } : { operation: '', step: 0, total: 0, actions: [], conflicts: [] },
    Bisect: bisect,
    ...more,
  },
})

const strip = (page: Page) => page.getByRole('status').filter({ hasText: /Bisecting|Found it|one of/ })
const last = async (page: Page, method: string) => (await calls(page, method)).at(-1)

test.describe('while a commit is under test', () => {
  test('the strip says where things stand and asks its question', async ({ page }) => {
    await open(page, 'bisect', scenario(running))
    await expect(strip(page)).toContainText('Bisecting')
    await expect(strip(page)).toContainText('Step 2 of about 3')
    await expect(strip(page)).toContainText('1 commit left after this')
    await expect(strip(page)).toContainText('Is it working at 0000000?')
  })

  test('each answer goes to git, by button and by key', async ({ page }) => {
    await open(page, 'bisect', scenario(running))
    await strip(page).getByRole('button', { name: /^Good/ }).click()
    await expect.poll(() => last(page, 'MarkBisect')).toEqual([expect.any(String), 'good'])
    await page.locator('.scroller').focus()
    await page.keyboard.press('b')
    await expect.poll(() => last(page, 'MarkBisect')).toEqual([expect.any(String), 'bad'])
    await page.keyboard.press('s')
    await expect.poll(() => last(page, 'MarkBisect')).toEqual([expect.any(String), 'skip'])
  })

  test('the last answer can be taken back, and stopping asks first', async ({ page }) => {
    await open(page, 'bisect', scenario(running))
    await strip(page).getByRole('button', { name: 'Undo last' }).click()
    await expect.poll(() => calls(page, 'UndoBisect')).toHaveLength(1)

    await strip(page).getByRole('button', { name: 'Stop…' }).click()
    expect(await calls(page, 'RunOperationAction')).toHaveLength(0)
    await expect(strip(page)).toContainText('go back to main?')
    await strip(page).getByRole('button', { name: 'Keep going' }).click()
    await strip(page).getByRole('button', { name: 'Stop…' }).click()
    await strip(page).getByRole('button', { name: 'Stop the bisect' }).click()
    await expect.poll(() => last(page, 'RunOperationAction')).toEqual([expect.any(String), 'abort'])
  })

  test('undo is off until an answer has been given', async ({ page }) => {
    await open(page, 'bisect', scenario({ ...running, steps: 0, marks: running.marks.slice(0, 2) }))
    await expect(strip(page).getByRole('button', { name: 'Undo last' })).toBeDisabled()
  })

  test('the panel on the right is about the commit under test', async ({ page }) => {
    await open(page, 'bisect', scenario(running))
    const panel = page.getByRole('complementary', { name: 'The commit under test' })
    await expect(panel).toContainText('under test')
    await expect(panel.getByRole('heading', { name: 'Set the alert limit in knots too' })).toBeVisible()
    await expect(panel).toContainText('src/alerts.js')
    await expect(panel).toContainText('step 1')
    await panel.getByRole('button', { name: "It's broken" }).click()
    await expect.poll(() => last(page, 'MarkBisect')).toEqual([expect.any(String), 'bad'])
  })
})

test.describe('once the answer is known', () => {
  const found = { ...running, current: '', firstBad: id('0'), steps: 3, remaining: 0, stepsLeft: 0, inQuestion: [], marks: [...running.marks, { commit: id('0'), subject: 'Set the alert limit in knots too', term: 'bad', step: 2 }, { commit: id('c'), subject: 'Show wind in knots', term: 'good', step: 3 }] }

  test('the strip says so, and Done goes back', async ({ page }) => {
    await open(page, 'bisect', scenario(found))
    await expect(strip(page)).toContainText('Found it in 3 steps')
    await expect(strip(page)).toContainText('The first bad commit is 0000000')
    await expect(strip(page).getByRole('button', { name: /^Good/ })).toHaveCount(0)
    await strip(page).getByRole('button', { name: 'Done, back to main' }).click()
    await expect.poll(() => last(page, 'RunOperationAction')).toEqual([expect.any(String), 'abort'])
  })

  test('the panel offers what to do next', async ({ page }) => {
    await open(page, 'bisect', scenario(found))
    const panel = page.getByRole('complementary', { name: 'The first bad commit' })
    await expect(panel).toContainText('the first bad commit')
    await expect(panel).toContainText('How it was found')
    for (const name of ['Open its changes', /Start a fix branch from main/, /Revert it on main/]) await expect(panel.getByRole('button', { name })).toBeVisible()
    await panel.getByRole('button', { name: 'Open its changes' }).click()
    await expect(page.getByRole('complementary', { name: 'Commit details' })).toBeVisible()
  })

  test('skipped commits can leave several candidates', async ({ page }) => {
    await open(page, 'bisect', scenario({ ...found, firstBad: '', candidates: [id('0'), id('d')], steps: 1 }))
    await expect(strip(page)).toContainText("It's one of 2 commits")
    await expect(page.getByRole('complementary', { name: 'The commit under test' })).toContainText('It broke in one of 2 commits')
  })
})

test.describe('starting one', () => {
  const plan = { bad: id('f'), good: id('e'), badSubject: 'Ignore log files', goodSubject: 'Release 0.2.0', suggested: 'v0.2', commits: 7, steps: 3, first: id('c'), firstSubject: 'Show wind in knots', problem: '' }

  async function openDialog(page: Page, more: object = {}) {
    await open(page, 'bisect', scenario(null, { PlanBisect: plan, ...more }))
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 400, box.y + 52 + 26, { button: 'right' })
    await page.getByRole('menuitem', { name: /Find what broke it/ }).click()
    return page.getByRole('dialog', { name: 'Find the commit that broke it' })
  }

  test("a commit's menu opens the dialog, with the last tag suggested as good", async ({ page }) => {
    const dialog = await openDialog(page)
    await expect(dialog).toBeVisible()
    await expect(dialog.getByLabel('A commit where it worked')).toHaveValue('v0.2')
    await expect(dialog).toContainText('the last tag')
    await expect(dialog).toContainText('7')
    await expect(dialog).toContainText('about 3 steps')
    await dialog.getByRole('button', { name: /Start with ccccccc/ }).click()
    await expect.poll(() => last(page, 'StartBisect')).toEqual([expect.any(String), id('f'), id('e')])
  })

  test('says why two commits cannot be bisected, and will not start', async ({ page }) => {
    const dialog = await openDialog(page, { PlanBisect: { ...plan, commits: 0, steps: 0, first: '', problem: "The good commit isn't in the bad commit's history, so there's nothing between them to search." } })
    await expect(dialog.getByRole('alert')).toContainText("isn't in the bad commit's history")
    await expect(dialog.getByRole('button', { name: /^Start/ })).toBeDisabled()
  })

  test('the menu entry goes while a bisect is running', async ({ page }) => {
    await open(page, 'bisect', scenario(running))
    await expect(strip(page)).toBeVisible()
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 400, box.y + 52 + 26, { button: 'right' })
    await expect(page.getByRole('menu')).toBeVisible()
    await expect(page.getByRole('menuitem', { name: /Find what broke it/ })).toHaveCount(0)
  })
})

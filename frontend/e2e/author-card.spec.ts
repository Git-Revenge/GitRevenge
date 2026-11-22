import { expect, test, type Page } from '@playwright/test'
import { calls, open } from './support'

// The card about a commit's author (#839): opened from the author's name in the commit panel
// or on a graph row, by Enter on a row, or from the row's menu. The backend is stubbed, so
// these check what the card says, what it asks for, and where each way on from it leads.

const ROW = 52
const sha = '0'.repeat(40)
const ana = { name: 'Ana Cruz', email: 'ana@example.com' }
const commit = { id: sha, subject: 'Commit 0', body: '', trailers: [], parents: ['a'.repeat(40)], author: { ...ana, time: 1793500000 }, committer: { ...ana, time: 1793500000 } }
const whoIs = { shown: ana, written: ana, lines: [], bot: false }
const anaAll = { ...ana, commits: 3, coAuthored: 6, bot: false, identities: [{ ...ana, commits: 3, coAuthored: 6, lines: [] }] }
const base = { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } }, CommitDetail: { commit, files: [] }, CommitRow: 1, AuthorCard: whoIs, PersonCard: anaAll }
const scenario = (responses: object = {}, more: object = {}) => ({ graphRows: 3, responses: { ...base, ...responses }, ...more })

const panel = (page: Page) => page.getByRole('complementary', { name: 'Commit details' })
const card = (page: Page, name = 'Ana Cruz') => page.getByRole('dialog', { name: `About ${name}` })
/** Selects the first commit, which shows it in the commit panel. */
async function selectCommit(page: Page) {
  const box = (await page.locator('.scroller').boundingBox())!
  await page.mouse.click(box.x + 500, box.y + ROW + 26)
  await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
}
type Win = { __peopleTab: () => { open: boolean; repo: string; ask: { person: { name: string; email: string; commits: number }; pick: boolean } | null } }
const peopleTab = (page: Page) => page.evaluate(() => (window as unknown as Win).__peopleTab())

test.describe("an author's card", () => {
  test('opens from the name in the commit panel, and says who and how much', async ({ page }) => {
    // The count reads the whole history the first time, so it arrives after the rest.
    await open(page, 'repoView', scenario({}, { delays: { PersonCard: 2000 } }))
    await selectCommit(page)
    const name = panel(page).getByRole('button', { name: 'Ana Cruz' })
    await expect(name).toHaveAttribute('aria-haspopup', 'dialog')
    await name.click()

    const c = card(page)
    await expect(c).toBeVisible()
    await expect(c).toBeFocused()
    await expect(c.locator('.n')).toHaveText('Ana Cruz')
    await expect(c.locator('.mail')).toHaveText('ana@example.com')
    await expect(c.locator('.face')).toHaveText('AC')
    await expect(c.getByRole('status')).toHaveText('Counting their commits…')
    // Joining needs every name read as them, so it waits for the count.
    await expect(c.getByRole('button', { name: 'Same person as…' })).toBeDisabled()
    await expect(c.getByRole('status')).toHaveText('3 commits, co-author on 6')
    await expect(c.getByRole('button', { name: 'Same person as…' })).toBeEnabled()
    // Shown as written: nothing more to say about this commit, and no bot mark.
    await expect(c.locator('.says')).toHaveCount(0)
    await expect(c.locator('.tag')).toHaveCount(0)
    expect(await calls(page, 'AuthorCard')).toEqual([['/r', sha]])
    expect(await calls(page, 'PersonCard')).toEqual([['/r', ana]])

    // It sits under the name, inside the window.
    const [at, from] = [(await c.boundingBox())!, (await name.boundingBox())!]
    expect(at.y).toBeGreaterThanOrEqual(from.y + from.height)
    expect(at.x + at.width).toBeLessThanOrEqual(page.viewportSize()!.width)
  })

  test('says what the commit has when the mailmap reads it as someone else', async ({ page }) => {
    const written = { name: 'ana cruz', email: '1234567+ana-cruz-ito@users.noreply.github.example' }
    const shown = { name: 'Ana Cruz-Ito', email: 'ana@tidewater.example' }
    const responses = {
      CommitDetail: { commit: { ...commit, author: { ...written, time: 1793500000 } }, files: [], author: shown, committer: shown },
      AuthorCard: { shown, written, lines: [{ source: '.mailmap', line: 6 }], bot: false },
      PersonCard: { ...shown, commits: 9, coAuthored: 0, bot: false, identities: [] },
    }
    await open(page, 'repoView', scenario(responses))
    await selectCommit(page)
    await panel(page).getByRole('button', { name: 'Ana Cruz-Ito' }).click()

    const c = card(page, 'Ana Cruz-Ito')
    await expect(c.locator('.says .label')).toHaveText('This commit says')
    await expect(c.locator('.says .as')).toHaveText('ana cruz <1234567+ana-cruz-ito@users.noreply.github.example>')
    await expect(c.locator('.says .by')).toHaveText('Read as Ana Cruz-Ito by line 6 of .mailmap.')
    await expect(c.getByRole('status')).toHaveText('9 commits')
    // The long address wraps inside the card rather than widening it.
    const [inner, outer] = [(await c.locator('.says .as').boundingBox())!, (await c.boundingBox())!]
    expect(inner.x + inner.width).toBeLessThanOrEqual(outer.x + outer.width)
  })

  test('marks a bot', async ({ page }) => {
    const bot = { name: 'dependabot[bot]', email: 'bot@users.noreply.example' }
    const responses = { CommitDetail: { commit: { ...commit, author: { ...bot, time: 1793500000 } }, files: [] }, AuthorCard: { shown: bot, written: bot, lines: [], bot: true }, PersonCard: { ...bot, commits: 40, coAuthored: 0, bot: true, identities: [] } }
    await open(page, 'repoView', scenario(responses))
    await selectCommit(page)
    await panel(page).getByRole('button', { name: 'dependabot[bot]' }).click()
    await expect(card(page, 'dependabot[bot]').locator('.tag')).toHaveText('bot')
  })

  test('Escape closes it and gives the focus back; so does a click elsewhere', async ({ page }) => {
    await open(page, 'repoView', scenario())
    await selectCommit(page)
    const name = panel(page).getByRole('button', { name: 'Ana Cruz' })
    await name.click()
    await expect(card(page)).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(card(page)).toHaveCount(0)
    await expect(name).toBeFocused()
    // The commit is still showing: Escape closed the card and nothing under it.
    await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()

    await name.click()
    await expect(card(page)).toBeVisible()
    await page.getByRole('heading', { name: 'Commit 0' }).click()
    await expect(card(page)).toHaveCount(0)
  })

  test('searches the timeline for them', async ({ page }) => {
    await open(page, 'repoView', scenario())
    await selectCommit(page)
    await panel(page).getByRole('button', { name: 'Ana Cruz' }).click()
    await card(page).getByRole('button', { name: 'Search the timeline for them' }).click()
    await expect(card(page)).toHaveCount(0)
    await expect(page.getByRole('combobox', { name: 'Search commits' })).toHaveValue('Ana Cruz')
    await expect.poll(() => calls(page, 'SearchGraph')).toContainEqual(['/r', 'Ana Cruz'])
  })

  test('Open People goes to them, and Same person as… starts with them picked', async ({ page }) => {
    await open(page, 'repoView', scenario())
    await selectCommit(page)
    const name = panel(page).getByRole('button', { name: 'Ana Cruz' })
    await name.click()
    await expect(card(page).getByRole('status')).toHaveText('3 commits, co-author on 6')
    await card(page).getByRole('button', { name: 'Open People' }).click()
    await expect(card(page)).toHaveCount(0)
    let tab = await peopleTab(page)
    expect(tab.open).toBe(true)
    expect(tab.repo).toBe('/r')
    expect(tab.ask).toMatchObject({ pick: false, person: { name: 'Ana Cruz', email: 'ana@example.com' } })

    await name.click()
    await card(page).getByRole('button', { name: 'Same person as…' }).click()
    tab = await peopleTab(page)
    // Everything read as them goes along: the dialog joins all of it.
    expect(tab.ask).toMatchObject({ pick: true, person: { name: 'Ana Cruz', email: 'ana@example.com', commits: 3 } })
    // Looking is all any of it did.
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
  })

  test('opens from a graph row: Enter on the commit, a click on its author, and its menu', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    const a = { name: 'A', email: 'a@example.com' }
    await open(page, 'repoView', scenario({ AuthorCard: { shown: a, written: a, lines: [], bot: false }, PersonCard: { ...a, commits: 3, coAuthored: 0, bot: false, identities: [] } }))
    await selectCommit(page)
    const grid = page.locator('.scroller')
    await grid.focus()
    await page.keyboard.press('Enter')
    const c = card(page, 'A')
    await expect(c).toBeVisible()
    await expect(c).toBeFocused()
    await expect(c.getByRole('status')).toHaveText('3 commits')
    expect(await calls(page, 'AuthorCard')).toEqual([['/r', sha]])
    // Under the row, not over it.
    const box = (await grid.boundingBox())!
    expect((await c.boundingBox())!.y).toBeGreaterThanOrEqual(box.y + ROW + 40)
    await page.keyboard.press('Escape')
    await expect(c).toHaveCount(0)
    await expect(grid).toBeFocused()

    // The first commit starts a run, so its second line names the author, under the day's
    // line and the subject. The pointer says where a click will land on them.
    const y = box.y + ROW + 64
    let x = -1
    for (let at = box.x + 20; at < box.x + 200; at += 3) {
      await page.mouse.move(at, y)
      if ((await grid.evaluate((el) => (el as HTMLElement).style.cursor)) === 'pointer') {
        x = at
        break
      }
    }
    expect(x, 'somewhere on the row is the author').toBeGreaterThan(0)
    await page.mouse.click(x, y)
    await expect(c).toBeVisible()
    await page.keyboard.press('Escape')
    // Off the name the row is only selected.
    await page.mouse.click(box.x + 900, y)
    await expect(c).toHaveCount(0)

    await page.mouse.click(box.x + 500, box.y + ROW + 44, { button: 'right' })
    await page.getByRole('menuitem', { name: 'About A' }).click()
    await expect(c).toBeVisible()
  })
})

test.describe('the People tab, asked for someone by an author\'s card', () => {
  const person = (name: string, email: string, commits: number) => ({ name, email, commits, coAuthored: 0, bot: false, identities: [{ name, email, commits, coAuthored: 0, lines: [] }] })
  const everyone = [person('Maya Okafor', 'maya@tidewater.example', 71), person('Priya Raman', 'priya@tidewater.example', 48), person('Priya R', 'priya@oldmill.example', 9)]
  const list = { people: everyone, matching: 3, everyone: 3, bots: 0, identities: 3, commits: 128 }
  const responses = { MailmapSources: [{ source: '.mailmap', path: '/r/.mailmap', exists: false, editable: true, lines: 0 }], ReadMailmapFile: '', People: list, MailmapNotes: [], WhoIs: [] }

  test('goes to them: the list is narrowed to their address', async ({ page }) => {
    await open(page, 'people', { responses, peopleAsk: { person: everyone[2], pick: false } })
    await expect(page.getByPlaceholder('Find a person by any name or address')).toHaveValue('priya@oldmill.example')
    await expect.poll(async () => (await calls(page, 'People')).some((c) => (c[1] as { query: string }).query === 'priya@oldmill.example')).toBe(true)
    await expect(page.locator('.picking')).toHaveCount(0)
  })

  test('starts Same person as… with them picked, and the whole list to choose from', async ({ page }) => {
    await open(page, 'people', { responses, peopleAsk: { person: everyone[2], pick: true } })
    await expect(page.locator('.picking')).toContainText('Who is Priya R the same person as?')
    await expect(page.getByPlaceholder('Find a person by any name or address')).toHaveValue('')
    const rows = page.getByRole('list', { name: 'People' }).getByRole('listitem')
    // Everyone else can be chosen; they can't be joined to themselves.
    await expect(rows.filter({ hasText: 'Priya Raman' }).getByRole('button', { name: 'This one' })).toBeVisible()
    await expect(rows.filter({ hasText: 'priya@oldmill.example' }).getByRole('button', { name: 'This one' })).toHaveCount(0)
    expect(await calls(page, 'SaveMailmap')).toHaveLength(0)
  })
})

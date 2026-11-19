import { expect, test, type Page } from '@playwright/test'
import { branch, calls, open } from './support'

// #810: the Map's second mode, with a node for each person who wrote commits.
test.describe("the Map's Authors mode", () => {
  const DAY = 24 * 60 * 60
  const id = (i: number) => String(i).padStart(40, '0')
  const now = Math.floor(Date.now() / 1000)
  const trunk = { name: 'main', kind: 'trunk', commit: id(0), commits: 392, updated: now, parent: '', head: true, branchKind: 'trunk' }
  const map = { trunk, nodes: [{ name: 'topic', kind: 'branch', commit: id(5), commits: 3, updated: now - DAY, parent: '', head: false }], branches: 2, tags: 0, moreBranches: 0 }
  const author = (name: string, commits: number, days: number, over: object = {}) => {
    const email = `${name.toLowerCase().replace(/[^a-z0-9]+/g, '.')}@example.com`
    return { key: email, name, email, identities: [{ name, email, commits }], commits, authored: commits, coAuthored: 0, updated: now - days * DAY, bot: false, you: false, trunk: commits, branches: [], moreBranches: 0, ...over }
  }
  const maya = author('Maya Okafor', 38, 0, {
    you: true,
    authored: 12,
    coAuthored: 26,
    identities: [
      { name: 'Maya Okafor', email: 'maya.okafor@example.com', commits: 30 },
      { name: 'mokafor', email: 'maya.okafor@example.com', commits: 8 },
    ],
  })
  const authors = { trunk, authors: [maya, author('renovate[bot]', 21, 2, { bot: true }), author('Leo Brandt', 9, 12), author('Priya Nair', 1, 40)], people: 3, bots: 1, commits: 69, since: now - 61 * DAY, uncountedBranches: 0 }
  const scenario = (over: object = {}) => ({
    graphRows: 30,
    graphRefs: { [id(0)]: [branch('main', { head: true, branchKind: 'trunk' })] },
    responses: {
      CommitSignatures: [],
      Refs: [branch('main', { head: true, branchKind: 'trunk' })],
      RepoPreferences: { hideKinds: [] },
      GraphViews: { view: 'map', experimental: ['map'] },
      BranchMap: map,
      MapAuthors: authors,
      MapAuthorHistory: { authored: 420, coAuthored: 41, first: now - 400 * DAY, last: now },
      ...over,
    },
  })
  const authorOf = (page: Page, name: string) => page.locator('.node[data-node^="author:"]').filter({ hasText: name })
  const toAuthors = async (page: Page, over: object = {}) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario(over))
    await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' }).click()
  }
  const shapeWidth = async (page: Page, name: string) => (await authorOf(page, name).locator('.shape').boundingBox())!.width

  test('opens on branches, and the switch turns it to the people of the last two months', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    const sw = page.getByRole('group', { name: 'What the map shows' })
    await expect(sw.getByRole('button', { name: 'Branches' })).toHaveAttribute('aria-pressed', 'true')
    await expect(page.locator('.node[data-node="branch:topic"]')).toBeVisible()
    expect(await calls(page, 'MapAuthors')).toEqual([])

    await sw.getByRole('button', { name: 'Authors' }).click()
    await expect(authorOf(page, 'Maya Okafor')).toBeVisible()
    await expect(page.locator('.node[data-node="branch:topic"]')).toHaveCount(0)
    // The trunk moved today, so it opens at two months: 61 days.
    expect(await calls(page, 'MapAuthors')).toEqual([['/repo', 61]])
    await expect(page.getByRole('group', { name: 'Zoom' })).toContainText('Two months')
    await expect(page.locator('.status .meta')).toHaveText('3 people · 1 bot in the last two months')
    expect(await page.evaluate(() => localStorage.getItem('graph.map.mode:/repo'))).toBe('authors')
    // The trunk stays in the middle; the working tree's mark belongs to the branches.
    await expect(page.locator('.node[data-node="trunk:main"]')).toContainText('392 commits')
    await expect(page.locator('.node[data-node="trunk:main"]')).not.toContainText('you’re here')
  })

  test('remembers the mode for the repository', async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem('graph.map.mode:/repo', 'authors'))
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await expect(authorOf(page, 'Maya Okafor')).toBeVisible()
    await expect(page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' })).toHaveAttribute('aria-pressed', 'true')
  })

  test('sizes a person by their commits, marks you, and draws a bot apart', async ({ page }) => {
    await toAuthors(page)
    await page.getByRole('group', { name: 'Authors to show' }).getByRole('button', { name: 'Everyone' }).click()
    await expect(authorOf(page, 'Maya Okafor')).toContainText('38 commits')
    await expect(authorOf(page, 'Maya Okafor')).toContainText('that’s you')
    await expect(authorOf(page, 'Maya Okafor').locator('.shape')).toHaveText('MO')
    await expect(authorOf(page, 'Leo Brandt')).not.toContainText('that’s you')
    await expect(authorOf(page, 'Priya Nair')).toContainText('1 commit')
    expect(await shapeWidth(page, 'Maya Okafor')).toBeGreaterThan(await shapeWidth(page, 'Leo Brandt'))
    expect(await shapeWidth(page, 'Leo Brandt')).toBeGreaterThan(await shapeWidth(page, 'Priya Nair'))
    await expect(authorOf(page, 'Maya Okafor')).toHaveClass(/is-author/)
    await expect(authorOf(page, 'renovate[bot]')).toHaveClass(/is-bot/)
    await expect(authorOf(page, 'renovate[bot]').locator('.shape svg')).toBeVisible()
    await expect(page.getByRole('button', { name: /^Maya Okafor, you\. 38 commits in the last two months: 12 written, 26 co-authored\./ })).toBeVisible()
    await expect(page.getByRole('button', { name: /^renovate\[bot\]\. A bot\. 21 commits in the last two months\./ })).toBeVisible()
  })

  test('sits whoever committed lately nearer the middle', async ({ page }) => {
    await toAuthors(page)
    await expect(authorOf(page, 'Priya Nair')).toBeVisible()
    const centre = async (loc: ReturnType<typeof authorOf>) => {
      const b = (await loc.boundingBox())!
      return { x: b.x + b.width / 2, y: b.y + b.height / 2 }
    }
    const hub = await centre(page.locator('.node[data-node="trunk:main"]'))
    const far = async (name: string) => {
      const c = await centre(authorOf(page, name))
      return Math.hypot(c.x - hub.x, c.y - hub.y)
    }
    expect(await far('Leo Brandt')).toBeLessThan(await far('Priya Nair'))
  })

  test('opens on people, and the switch shows everyone or only bots, and remembers', async ({ page }) => {
    await toAuthors(page)
    const sw = page.getByRole('group', { name: 'Authors to show' })
    await expect(sw.getByRole('button', { name: 'People' })).toHaveAttribute('aria-pressed', 'true')
    await expect(authorOf(page, 'Leo Brandt')).toBeVisible()
    await expect(authorOf(page, 'renovate[bot]')).toHaveCount(0)
    await sw.getByRole('button', { name: 'Everyone' }).click()
    await expect(authorOf(page, 'renovate[bot]')).toBeVisible()
    await expect(authorOf(page, 'Leo Brandt')).toBeVisible()
    await sw.getByRole('button', { name: 'Bots' }).click()
    await expect(authorOf(page, 'renovate[bot]')).toBeVisible()
    await expect(authorOf(page, 'Leo Brandt')).toHaveCount(0)
    expect(await page.evaluate(() => localStorage.getItem('graph.map.who:/repo'))).toBe('bots')
    // The bar goes on counting everyone.
    await expect(page.locator('.status .meta')).toHaveText('3 people · 1 bot in the last two months')
  })

  test('zooming reads another span of time, and keeps its own zoom', async ({ page }) => {
    await toAuthors(page)
    await expect(authorOf(page, 'Maya Okafor')).toBeVisible()
    await page.getByRole('button', { name: 'Zoom out' }).click()
    await expect(page.getByRole('group', { name: 'Zoom' })).toContainText('Six months')
    await expect.poll(() => calls(page, 'MapAuthors')).toEqual([
      ['/repo', 61],
      ['/repo', 183],
    ])
    await expect(page.locator('.status .meta')).toHaveText('3 people · 1 bot in the last six months')
    expect(await page.evaluate(() => [localStorage.getItem('graph.map.zoom.authors:/repo'), localStorage.getItem('graph.map.zoom:/repo')])).toEqual(['3', null])
    // It can always look further back: nobody older than the span is known to it.
    await page.getByRole('button', { name: 'Zoom out' }).click()
    await page.getByRole('button', { name: 'Zoom out' }).click()
    await expect(page.getByRole('group', { name: 'Zoom' })).toContainText('Ten years')
    await expect(page.getByRole('button', { name: 'Zoom out' })).toBeDisabled()
    // Back among the branches, the zoom is the branches' own.
    await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Branches' }).click()
    await expect(page.getByRole('group', { name: 'Zoom' })).toContainText('Two weeks')
  })

  test('a card says what they wrote, in the span and in all, who was folded in, and searches for them', async ({ page }) => {
    await toAuthors(page)
    await authorOf(page, 'Maya Okafor').click()
    const card = page.getByRole('group', { name: 'About Maya Okafor' })
    await expect(card).toContainText('38 commits in the last two months: 12 written, 26 co-authored.')
    await expect(card).toContainText('Last commit just now.')
    await expect(card).toContainText('461 commits in all (41 co-authored). First commit')
    await expect(card).toContainText('Counted as one person:')
    // Each identity is a name, an email and how many commits carry it.
    await expect(card.locator('.who b')).toHaveText(['Maya Okafor', 'mokafor'])
    await expect(card.locator('.who span')).toHaveText(['maya.okafor@example.com', 'maya.okafor@example.com'])
    await expect(card.locator('.who small')).toHaveText(['30', '8'])
    // The count is never what gets cut short.
    for (const count of await card.locator('.who small').all()) await expect(count).toBeInViewport({ ratio: 1 })
    expect(await calls(page, 'MapAuthorHistory')).toEqual([['/repo', maya.identities]])

    // Somebody under one name has nothing folded in to list.
    await authorOf(page, 'Leo Brandt').click()
    const leo = page.getByRole('group', { name: 'About Leo Brandt' })
    await expect(leo).toContainText('9 commits in the last two months.')
    await expect(leo).not.toContainText('Counted as one person')

    await leo.getByRole('button', { name: 'Search the timeline for them' }).click()
    await expect(page.locator('.map')).toHaveCount(0)
    await expect.poll(async () => (await calls(page, 'SetGraphView')).at(-1)).toEqual([''])
    await expect.poll(async () => (await calls(page, 'SearchGraph')).at(-1)).toEqual(['/repo', 'Leo Brandt'])
  })

  test('a card closes on Escape, and a dragged author stays where they were put', async ({ page }) => {
    await toAuthors(page)
    await authorOf(page, 'Leo Brandt').click()
    await expect(page.getByRole('group', { name: 'About Leo Brandt' })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('group', { name: 'About Leo Brandt' })).toHaveCount(0)
    await expect(authorOf(page, 'Leo Brandt')).toBeFocused()

    const box = (await authorOf(page, 'Leo Brandt').boundingBox())!
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(980, 140, { steps: 6 })
    await page.mouse.up()
    await expect(authorOf(page, 'Leo Brandt')).toHaveClass(/pinned/)
    expect(Object.keys(await page.evaluate(() => JSON.parse(localStorage.getItem('graph.map.pins:/repo') ?? '{}') as object))).toEqual(['author:leo.brandt@example.com'])
  })

  test('says so when nobody committed in the span', async ({ page }) => {
    await toAuthors(page, { MapAuthors: { ...authors, authors: [], people: 0, bots: 0, commits: 0 } })
    await expect(page.getByRole('status').filter({ hasText: 'Nobody committed' })).toHaveText('Nobody committed in the last two months. Zoom out to look further back.')
    await expect(page.locator('.status .meta')).toHaveText('0 people in the last two months')
  })

  test('opens further out when the trunk has been quiet, so it isn’t empty', async ({ page }) => {
    const quiet = { ...trunk, updated: now - 100 * DAY }
    await toAuthors(page, { BranchMap: { ...map, trunk: quiet }, MapAuthors: { ...authors, trunk: quiet } })
    await expect(page.getByRole('group', { name: 'Zoom' })).toContainText('Six months')
    await expect.poll(() => calls(page, 'MapAuthors')).toEqual([['/repo', 183]])
  })

  test('says how many people were left out when there are more than it draws', async ({ page }) => {
    const many = Array.from({ length: 60 }, (_, i) => author(`Person ${i}`, Math.ceil(300 / (i + 1)), i % 50))
    await toAuthors(page, { MapAuthors: { ...authors, authors: many, people: 10339, bots: 0 } })
    await expect(page.locator('.status .meta')).toHaveText(`60 of ${(10339).toLocaleString()} people in the last two months`)
    await expect(page.locator('.legend')).toContainText(`${(10279).toLocaleString()} people with fewer not shown`)
    await expect(page.locator('.node[data-node^="author:"]')).toHaveCount(60)
  })

  test('says what went wrong when the authors can’t be read', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', { ...scenario(), failures: { MapAuthors: 'fatal: bad object' } })
    await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' }).click()
    await expect(page.getByRole('alert')).toContainText("Couldn't read who wrote the commits: fatal: bad object")
  })
})

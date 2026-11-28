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

  // #811: an author opens into the trunk and the unmerged branches that hold their commits.
  test.describe('opened by branch', () => {
    const held = (name: string, kind: string, commits: number, days: number) => ({ name, kind, commit: id(5), commits, updated: now - days * DAY, parent: '', head: false })
    const mayaOpen = { ...maya, trunk: 31, branches: [held('topic', 'branch', 5, 1), held('origin/spike', 'remote', 2, 3)], moreBranches: 2 }
    const leoOpen = author('Leo Brandt', 9, 12, { trunk: 8, branches: [held('topic', 'branch', 1, 1)] })
    const priya = author('Priya Nair', 1, 40, { trunk: 0, branches: [] })
    const opened = { MapAuthors: { ...authors, authors: [mayaOpen, leoOpen, priya], people: 3, bots: 0 } }
    const heldBy = (page: Page, who: string, what: string) => page.locator(`.node.owned[data-node^="branch:${who}|"][data-node$=":${what}"], .node.owned[data-node^="remote:${who}|"][data-node$=":${what}"]`)

    test('a double-click opens an author into what has landed and what’s on branches, and another folds it away', async ({ page }) => {
      await toAuthors(page, opened)
      const node = authorOf(page, 'Maya Okafor')
      await expect(node).toHaveAttribute('aria-expanded', 'false')
      await expect(page.locator('.node.owned')).toHaveCount(0)
      await node.dblclick()
      await expect(node).toHaveAttribute('aria-expanded', 'true')
      await expect(page.locator('.node.owned')).toHaveCount(3)
      await expect(heldBy(page, maya.key, 'main')).toContainText('31 landed')
      await expect(heldBy(page, maya.key, 'topic')).toContainText('5 commits')
      await expect(heldBy(page, maya.key, 'origin/spike')).toHaveClass(/is-remote/)
      await expect(page.getByRole('button', { name: /^main, of Maya Okafor's\. 31 commits of Maya Okafor's in the last two months are in main: landed\./ })).toBeVisible()
      await expect(page.getByRole('button', { name: /^topic, of Maya Okafor's\. 5 commits of Maya Okafor's in the last two months are here and not in main\./ })).toBeVisible()
      await node.dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(0)
      // Which authors are open isn't kept.
      expect(await page.evaluate(() => Object.keys(localStorage).filter((k) => /open/i.test(k)))).toEqual([])
    })

    test('what an author holds stands by them, in their colour', async ({ page }) => {
      await toAuthors(page, opened)
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      const centre = async (loc: ReturnType<typeof authorOf>) => {
        const b = (await loc.boundingBox())!
        return { x: b.x + b.width / 2, y: b.y + b.height / 2 }
      }
      const [at, hub] = [await centre(authorOf(page, 'Maya Okafor')), await centre(page.locator('.node[data-node="trunk:main"]'))]
      for (const what of ['main', 'topic', 'origin/spike']) {
        const c = await centre(heldBy(page, maya.key, what))
        expect(Math.hypot(c.x - at.x, c.y - at.y), what).toBeLessThan(Math.hypot(c.x - hub.x, c.y - hub.y))
      }
      const colour = (loc: ReturnType<typeof authorOf>) => loc.evaluate((el) => (el as HTMLElement).style.getPropertyValue('--c'))
      expect(await colour(heldBy(page, maya.key, 'topic'))).toBe(await colour(authorOf(page, 'Maya Okafor')))
    })

    test('the card opens and folds them, and says when some branches aren’t shown', async ({ page }) => {
      await toAuthors(page, opened)
      await authorOf(page, 'Maya Okafor').click()
      const card = page.getByRole('group', { name: 'About Maya Okafor' })
      await card.getByRole('button', { name: 'Show their branches' }).click()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      await expect(card).toContainText("2 more branches of theirs, less recently moved, aren't shown.")
      await card.getByRole('button', { name: 'Fold their branches away' }).click()
      await expect(page.locator('.node.owned')).toHaveCount(0)
      // Somebody with nothing landed and nothing on a branch that was read has nothing to open.
      await authorOf(page, 'Priya Nair').click()
      await expect(page.getByRole('group', { name: 'About Priya Nair' }).getByRole('button', { name: 'Show their branches' })).toHaveCount(0)
      await expect(authorOf(page, 'Priya Nair')).not.toHaveAttribute('aria-expanded')
    })

    test('two open authors each have their own, and pointing at a branch lights it up under the other', async ({ page }) => {
      await toAuthors(page, opened)
      await authorOf(page, 'Maya Okafor').dblclick()
      await authorOf(page, 'Leo Brandt').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(5)
      await expect(heldBy(page, leoOpen.key, 'main')).toContainText('8 landed')
      await expect(heldBy(page, leoOpen.key, 'topic')).toContainText('1 commit')
      await heldBy(page, maya.key, 'topic').hover()
      await expect(heldBy(page, leoOpen.key, 'topic')).toHaveClass(/echo/)
      await expect(heldBy(page, maya.key, 'topic')).toHaveClass(/echo/)
      await expect(heldBy(page, leoOpen.key, 'main')).not.toHaveClass(/echo/)
      // From the keyboard too.
      await page.mouse.move(2, 2)
      await expect(page.locator('.node.echo')).toHaveCount(0)
      await heldBy(page, leoOpen.key, 'main').focus()
      await expect(heldBy(page, maya.key, 'main')).toHaveClass(/echo/)
    })

    test('a branch an author holds has a card that leads to the timeline, and the branch’s own menu', async ({ page }) => {
      await toAuthors(page, opened)
      await authorOf(page, 'Maya Okafor').dblclick()
      await heldBy(page, maya.key, 'topic').click()
      const card = page.getByRole('group', { name: "About topic, of Maya Okafor's" })
      await expect(card).toContainText("5 commits of Maya Okafor's in the last two months are here and not in main. The branch last moved 1d ago.")
      await expect(card.getByRole('button', { name: 'Show it in the timeline' })).toBeVisible()
      // Folding the author away takes the card with it.
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(card).toHaveCount(0)

      await authorOf(page, 'Maya Okafor').dblclick()
      await heldBy(page, maya.key, 'topic').click()
      await page.getByRole('group', { name: "About topic, of Maya Okafor's" }).getByRole('button', { name: 'Focus the timeline on it' }).click()
      await expect.poll(async () => (await calls(page, 'SetGraphFocus')).at(-1)).toEqual(['/repo', 'topic', false])
    })
  })

  // #812: an author opens into the folders their commits touched, read when they're opened.
  test.describe('opened by area', () => {
    const commit = (n: number, subject: string, days: number) => ({ sha: id(n), subject, when: now - days * DAY })
    const folder = (path: string, commits: number, days: number, over: object = {}) => ({ path, name: `${path.split('/').pop()}/`, files: false, commits, updated: now - days * DAY, folders: 0, recent: [commit(3, `Work in ${path}`, days)], ...over })
    const areas = {
      '': {
        under: '',
        areas: [
          folder('frontend', 24, 0, { folders: 2, recent: [commit(3, 'Draw the tide table', 0), commit(4, 'Fix the units switch', 2), commit(6, 'Tidy the sidebar', 9)] }),
          folder('internal', 11, 1, { folders: 1 }),
          { path: '', name: 'top-level files', files: true, commits: 3, updated: now - 4 * DAY, folders: 0, recent: [commit(7, 'Bump the version', 4)] },
        ],
        more: 4,
        commits: 38,
      },
      frontend: { under: 'frontend', areas: [folder('frontend/src', 20, 0, { folders: 3 }), folder('frontend/e2e', 6, 2), { path: 'frontend', name: 'files in frontend/', files: true, commits: 2, updated: now - 5 * DAY, folders: 0, recent: [] }], more: 0, commits: 38 },
    }
    const leoAreas = author('Leo Brandt', 9, 12)
    const withAreas = { MapAuthors: { ...authors, authors: [maya, leoAreas], people: 2, bots: 0 }, MapAuthorAreas: areas }
    const by = (page: Page) => page.getByRole('group', { name: 'What an author opens into' })
    const areaOf = (page: Page, who: string, path: string, files = false) => page.locator(`.node.owned[data-node="folder:${who}|${files ? 'files' : 'folder'}:${path}"]`)
    const toAreas = async (page: Page, over: object = withAreas) => {
      await toAuthors(page, over)
      await by(page).getByRole('button', { name: 'By area' }).click()
    }

    test('the switch opens on branches, turns authors to areas, and remembers', async ({ page }) => {
      await toAuthors(page, withAreas)
      await expect(by(page).getByRole('button', { name: 'By branch' })).toHaveAttribute('aria-pressed', 'true')
      await by(page).getByRole('button', { name: 'By area' }).click()
      await expect(by(page).getByRole('button', { name: 'By area' })).toHaveAttribute('aria-pressed', 'true')
      expect(await page.evaluate(() => localStorage.getItem('graph.map.by:/repo'))).toBe('area')
      // Nothing is read until somebody is opened.
      expect(await calls(page, 'MapAuthorAreas')).toEqual([])
    })

    test('opening an author reads the folders they worked in and stands them round them', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      expect(await calls(page, 'MapAuthorAreas')).toEqual([['/repo', maya.identities, 61, '']])
      await expect(areaOf(page, maya.key, 'frontend')).toContainText('frontend/')
      await expect(areaOf(page, maya.key, 'frontend')).toContainText('24 commits')
      await expect(areaOf(page, maya.key, 'frontend')).toHaveClass(/is-folder/)
      await expect(areaOf(page, maya.key, '', true)).toContainText('top-level files')
      await expect(areaOf(page, maya.key, '', true)).toHaveClass(/is-files/)
      await expect(page.getByRole('button', { name: /^frontend\/, of Maya Okafor's\. 24 commits of Maya Okafor's in the last two months touch frontend\/\. The last was just now\./ })).toBeVisible()
      // A folder with folders of theirs beneath it opens; one without doesn't, nor do the files.
      await expect(areaOf(page, maya.key, 'frontend')).toHaveAttribute('aria-expanded', 'false')
      await expect(areaOf(page, maya.key, '', true)).not.toHaveAttribute('aria-expanded')
      // More commits, a bigger folder.
      const wide = async (path: string) => (await areaOf(page, maya.key, path).locator('.shape').boundingBox())!.width
      expect(await wide('frontend')).toBeGreaterThan(await wide('internal'))
    })

    test('a folder opens into its own folders, a level at a time, and folds away with what was under it', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').dblclick()
      await areaOf(page, maya.key, 'frontend').dblclick()
      await expect(areaOf(page, maya.key, 'frontend')).toHaveAttribute('aria-expanded', 'true')
      await expect(page.locator('.node.owned')).toHaveCount(6)
      await expect(areaOf(page, maya.key, 'frontend/src')).toContainText('src/')
      await expect(areaOf(page, maya.key, 'frontend/src')).toContainText('20 commits')
      await expect(areaOf(page, maya.key, 'frontend', true)).toContainText('files in frontend/')
      expect((await calls(page, 'MapAuthorAreas')).map((c) => c[3])).toEqual(['', 'frontend'])
      // What's in a folder stands by the folder, not by the author.
      const centre = async (loc: ReturnType<typeof areaOf>) => {
        const b = (await loc.boundingBox())!
        return { x: b.x + b.width / 2, y: b.y + b.height / 2 }
      }
      const [src, fe, hub] = [await centre(areaOf(page, maya.key, 'frontend/src')), await centre(areaOf(page, maya.key, 'frontend')), await centre(page.locator('.node[data-node="trunk:main"]'))]
      expect(Math.hypot(src.x - fe.x, src.y - fe.y)).toBeLessThan(Math.hypot(src.x - hub.x, src.y - hub.y))

      await areaOf(page, maya.key, 'frontend').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      // Folding the author away takes everything; opening them again starts from the top,
      // without asking again.
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(0)
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      expect((await calls(page, 'MapAuthorAreas')).map((c) => c[3])).toEqual(['', 'frontend'])
    })

    test('a folder’s card lists their latest commits there, each leading to the timeline', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').dblclick()
      await areaOf(page, maya.key, 'frontend').click()
      const card = page.getByRole('group', { name: "About frontend/, of Maya Okafor's" })
      await expect(card).toContainText('Maya Okafor in frontend/')
      await expect(card).toContainText("24 commits of Maya Okafor's in the last two months touch frontend/. The last was just now.")
      await expect(card).toContainText('The latest 3:')
      await expect(card.locator('.latest button')).toHaveText(['Draw the tide table', 'Fix the units switch', 'Tidy the sidebar'])
      await card.getByRole('button', { name: 'Show its folders' }).click()
      await expect(page.locator('.node.owned')).toHaveCount(6)
      await expect(card.getByRole('button', { name: 'Fold its folders away' })).toBeVisible()

      await card.getByRole('button', { name: 'Fix the units switch' }).click()
      await expect(page.locator('.map')).toHaveCount(0)
      await expect.poll(async () => (await calls(page, 'CommitRow')).at(-1)).toEqual(['/repo', id(4)])
    })

    // #1060: the folder's whole history, everyone's, in the timeline.
    test('a folder’s card focuses the timeline on it; the files at the top have nothing to focus on', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').dblclick()
      await areaOf(page, maya.key, '', true).click()
      const topCard = page.locator('.map .card')
      await expect(topCard).toContainText('the files at the top')
      await expect(topCard.getByRole('button', { name: 'Focus the timeline on it' })).toHaveCount(0)
      await areaOf(page, maya.key, 'frontend').click()
      await page.getByRole('group', { name: "About frontend/, of Maya Okafor's" }).getByRole('button', { name: 'Focus the timeline on it' }).click()
      await expect.poll(() => calls(page, 'SetGraphPath')).toEqual([['/repo', 'frontend/']])
      await expect(page.locator('.map')).toHaveCount(0)
    })

    test('the author’s card opens them, and says when folders were left out', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').click()
      const card = page.getByRole('group', { name: 'About Maya Okafor' })
      await card.getByRole('button', { name: 'Show their areas' }).click()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      await expect(card).toContainText("4 more folders of theirs, with fewer commits, aren't shown.")
      await card.getByRole('button', { name: 'Fold their areas away' }).click()
      await expect(page.locator('.node.owned')).toHaveCount(0)
    })

    test('says it’s reading while the folders are read, and when they can’t be', async ({ page }) => {
      await page.setViewportSize({ width: 1200, height: 700 })
      await page.addInitScript(() => localStorage.setItem('graph.map.by:/repo', 'area'))
      await open(page, 'graph', { ...scenario(withAreas), delays: { MapAuthorAreas: 800 } })
      await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' }).click()
      await authorOf(page, 'Maya Okafor').click()
      const card = page.getByRole('group', { name: 'About Maya Okafor' })
      await card.getByRole('button', { name: 'Show their areas' }).click()
      await expect(card.getByRole('button', { name: 'Reading their folders…' })).toBeDisabled()
      await expect(card.getByRole('button', { name: 'Fold their areas away' })).toBeEnabled()
      await expect(page.locator('.node.owned')).toHaveCount(3)
    })

    test('says so when the folders can’t be read', async ({ page }) => {
      await page.setViewportSize({ width: 1200, height: 700 })
      await page.addInitScript(() => localStorage.setItem('graph.map.by:/repo', 'area'))
      await open(page, 'graph', { ...scenario(withAreas), failures: { MapAuthorAreas: 'fatal: bad object' } })
      await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' }).click()
      await authorOf(page, 'Maya Okafor').click()
      const card = page.getByRole('group', { name: 'About Maya Okafor' })
      await card.getByRole('button', { name: 'Show their areas' }).click()
      await expect(card.getByRole('alert')).toHaveText("Couldn't read the folders they worked in.")
      await expect(page.locator('.node.owned')).toHaveCount(0)
    })

    test('flipping the switch opens every open author the other way, and pointing at a folder lights it up under the others', async ({ page }) => {
      const both = { ...withAreas, MapAuthors: { ...authors, authors: [{ ...maya, trunk: 31, branches: [] }, { ...leoAreas, trunk: 8, branches: [] }], people: 2, bots: 0 } }
      await toAuthors(page, both)
      await authorOf(page, 'Maya Okafor').dblclick()
      await authorOf(page, 'Leo Brandt').dblclick()
      // By branch: each has what landed.
      await expect(page.locator('.node.owned.landed')).toHaveCount(2)
      await by(page).getByRole('button', { name: 'By area' }).click()
      await expect(page.locator('.node.owned.landed')).toHaveCount(0)
      await expect(page.locator('.node.owned.is-folder')).toHaveCount(4)
      await expect.poll(async () => (await calls(page, 'MapAuthorAreas')).length).toBe(2)
      await areaOf(page, maya.key, 'frontend').hover()
      await expect(areaOf(page, leoAreas.key, 'frontend')).toHaveClass(/echo/)
      await expect(areaOf(page, leoAreas.key, 'internal')).not.toHaveClass(/echo/)
      // And back.
      await by(page).getByRole('button', { name: 'By branch' }).click()
      await expect(page.locator('.node.owned.landed')).toHaveCount(2)
      await expect(page.locator('.node.owned.is-folder')).toHaveCount(0)
    })

    test('a change of span reads the open author’s folders again', async ({ page }) => {
      await toAreas(page)
      await authorOf(page, 'Maya Okafor').dblclick()
      await expect(page.locator('.node.owned')).toHaveCount(3)
      await page.getByRole('button', { name: 'Zoom out' }).click()
      await expect.poll(async () => (await calls(page, 'MapAuthorAreas')).map((c) => [c[2], c[3]])).toEqual([
        [61, ''],
        [183, ''],
      ])
      await expect(page.locator('.node.owned')).toHaveCount(3)
    })
  })

  test('says what went wrong when the authors can’t be read', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', { ...scenario(), failures: { MapAuthors: 'fatal: bad object' } })
    await page.getByRole('group', { name: 'What the map shows' }).getByRole('button', { name: 'Authors' }).click()
    await expect(page.getByRole('alert')).toContainText("Couldn't read who wrote the commits: fatal: bad object")
  })
})

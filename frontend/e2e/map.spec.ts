import { expect, test, type Page } from '@playwright/test'
import { branch, calls, events, open, pickView, tag } from './support'

// #652: an experimental view with a node for each branch and tag.
test.describe('the Map view', () => {
  const DAY = 24 * 60 * 60
  const id = (i: number) => String(i).padStart(40, '0')
  const now = Math.floor(Date.now() / 1000)
  const node = (name: string, kind: string, commits: number, days: number, over: object = {}) => ({ name, kind, commit: id(0), commits, updated: now - days * DAY, parent: '', head: false, ...over })
  const map = {
    trunk: node('main', 'trunk', 307, 0, { head: true, branchKind: 'trunk' }),
    nodes: [
      node('origin/gh-pages', 'remote', 65, 0),
      node('topic', 'branch', 1, 1, { commit: id(5) }),
      node('release/v0.8', 'branch', 0, 12),
      node('release/v0.7', 'branch', 56, 13),
      node('release/v0.6', 'branch', 9, 30),
      node('v0.11.0', 'tag', 58, 1, { commit: id(7) }),
      node('v0.7.0', 'tag', 47, 13, { parent: 'release/v0.7' }),
      node('6 older tags', 'tags', 72, 20, { tags: 6 }),
    ],
    branches: 6,
    tags: 8,
    moreBranches: 0,
  }
  const graphRefs = { [id(0)]: [branch('main', { head: true, branchKind: 'trunk' })], [id(5)]: [branch('topic')], [id(7)]: [tag('v0.11.0')] }
  const scenario = (over: object = {}, views: object = { view: 'map', experimental: ['map'] }) => ({
    graphRows: 30,
    graphRefs,
    responses: { CommitSignatures: [], Refs: [branch('main', { head: true, branchKind: 'trunk' })], RepoPreferences: { hideKinds: [] }, GraphViews: views, BranchMap: map, ...over },
  })
  const nodeOf = (page: Page, name: string) => page.locator(`.node[data-node$=":${name}"]`)
  const shapeWidth = async (page: Page, name: string) => (await nodeOf(page, name).locator('.shape').boundingBox())!.width

  test('is in the menu once switched on, and choosing it shows and saves it', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario({}, { view: '', experimental: ['map'] }))
    await expect(page.locator('.map')).toHaveCount(0)
    await pickView(page, 'Map, experimental')
    await expect(nodeOf(page, 'main')).toBeVisible()
    expect(await calls(page, 'SetGraphView')).toEqual([['map']])
    await expect(page.locator('.viewport')).toHaveAttribute('inert', '')
  })

  test('says what it holds in the bar, where the commit count was, and puts the search away', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await expect(page.locator('.status .meta')).toHaveText('6 branches · 8 tags')
    await expect(page.getByRole('searchbox', { name: 'Search commits' })).toBeHidden()
  })

  test('names every node and says how many commits it has', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await expect(nodeOf(page, 'main')).toContainText('307 commits')
    await expect(nodeOf(page, 'main')).toContainText('you’re here')
    await expect(nodeOf(page, 'topic')).toContainText('1 commit')
    await expect(nodeOf(page, 'release/v0.8')).toContainText('all in main')
    await expect(nodeOf(page, 'v0.11.0')).toContainText('58 commits')
    await expect(page.getByRole('button', { name: /^origin\/gh-pages\. Only on the remote\. 65 commits that aren't in main\./ })).toBeVisible()
  })

  test('draws more commits bigger, and each kind of node its own shape', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await expect(nodeOf(page, 'main')).toBeVisible()
    expect(await shapeWidth(page, 'release/v0.7')).toBeGreaterThan(await shapeWidth(page, 'release/v0.6'))
    expect(await shapeWidth(page, 'release/v0.6')).toBeGreaterThan(await shapeWidth(page, 'topic'))
    expect(await shapeWidth(page, 'main')).toBeGreaterThan(await shapeWidth(page, 'origin/gh-pages'))
    await expect(nodeOf(page, 'release/v0.8')).toHaveClass(/is-merged/)
    await expect(nodeOf(page, 'origin/gh-pages')).toHaveClass(/is-remote/)
    await expect(nodeOf(page, 'topic')).toHaveClass(/is-branch/)
    await expect(nodeOf(page, 'v0.11.0')).toHaveClass(/is-tag/)
  })

  test('sits what moved lately nearer the trunk than what has been still', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    const centre = async (name: string) => {
      const b = (await nodeOf(page, name).boundingBox())!
      return { x: b.x + b.width / 2, y: b.y + b.height / 2 }
    }
    const main = await centre('main')
    const far = async (name: string) => {
      const c = await centre(name)
      return Math.hypot(c.x - main.x, c.y - main.y)
    }
    expect(await far('topic')).toBeLessThan(await far('release/v0.7'))
    expect(await far('release/v0.7')).toBeLessThan(await far('release/v0.6'))
  })

  test('a click opens a node’s card, and Escape closes it and gives the focus back', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await nodeOf(page, 'release/v0.8').click()
    const card = page.getByRole('group', { name: 'About release/v0.8' })
    await expect(card).toContainText("Nothing main doesn't have")
    await expect(nodeOf(page, 'release/v0.8')).toHaveAttribute('aria-pressed', 'true')
    await page.keyboard.press('Escape')
    await expect(card).toHaveCount(0)
    await expect(nodeOf(page, 'release/v0.8')).toBeFocused()
  })

  test('a tag’s card says what it counts and where it hangs', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await nodeOf(page, 'v0.7.0').click()
    await expect(page.getByRole('group', { name: 'About v0.7.0' })).toContainText('47 commits since the tag before it, on release/v0.7')
    await nodeOf(page, '6 older tags').click()
    await expect(page.getByRole('group', { name: 'About 6 older tags' })).toContainText('6 tags older than the ones shown')
  })

  test('Show it in the timeline goes back to the rows and selects the commit', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await nodeOf(page, 'topic').click()
    await page.getByRole('button', { name: 'Show it in the timeline' }).click()
    await expect(page.locator('.map')).toHaveCount(0)
    expect(await calls(page, 'SetGraphView')).toEqual([['']])
    await expect.poll(async () => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index).toBe(5)
  })

  test('Focus the timeline on it is offered for a local branch, and focuses the graph', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await nodeOf(page, 'origin/gh-pages').click()
    await expect(page.getByRole('button', { name: 'Focus the timeline on it' })).toHaveCount(0)
    await nodeOf(page, 'topic').click()
    await page.getByRole('button', { name: 'Focus the timeline on it' }).click()
    await expect(page.locator('.map')).toHaveCount(0)
    expect(await calls(page, 'SetGraphFocus')).toEqual([['/repo', 'topic', false]])
  })

  test('a right-click on a branch asks for its commit’s menu', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await nodeOf(page, 'topic').click({ button: 'right' })
    await expect.poll(() => events(page, 'context-row')).toEqual([[5]])
  })

  // #657: in a narrow panel the key and the rings' names covered the nodes.
  test('a narrow panel leaves the key and the rings’ names out', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 700 })
    await open(page, 'graph', scenario())
    await expect(page.locator('.legend')).toBeVisible()
    await expect(page.locator('.ringname')).toHaveCount(3)
    await page.setViewportSize({ width: 400, height: 700 })
    await expect(page.locator('.legend')).toHaveCount(0)
    await expect(page.locator('.ringname')).toHaveCount(0)
    await expect(nodeOf(page, 'main')).toBeVisible()
  })

  test('says so when the repository has no branch to draw', async ({ page }) => {
    await open(page, 'graph', scenario({ BranchMap: { trunk: node('', 'trunk', 0, 0), nodes: [], branches: 0, tags: 0, moreBranches: 0 } }))
    await expect(page.locator('.map')).toContainText('No branches yet')
  })

  test('in a crowd, older names wait for the focus', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 800 })
    const many = Array.from({ length: 44 }, (_, i) => node(`b${i}`, 'branch', 1 + (i % 7), i < 4 ? 1 : 20))
    await open(page, 'graph', scenario({ BranchMap: { ...map, nodes: many, branches: 45, tags: 0 } }))
    await expect(nodeOf(page, 'b0').locator('.name')).toBeVisible()
    await expect(nodeOf(page, 'b30').locator('.name')).toBeHidden()
    await nodeOf(page, 'b30').focus()
    await expect(nodeOf(page, 'b30').locator('.name')).toBeVisible()
  })
})

test.describe('the Map view in settings', () => {
  const prefs = { theme: '', graphDensity: '', experimentalViews: [], updateChecks: true, gitPath: '', gitInUse: '/usr/bin/git', cloneDir: '', zoom: 100 }

  test('is one of the experimental views to switch on', async ({ page }) => {
    await open(page, 'settings', { responses: { Preferences: prefs } })
    await page.getByRole('checkbox', { name: 'Map' }).check()
    await page.getByRole('button', { name: 'Save' }).click()
    await expect.poll(async () => ((await calls(page, 'SetPreferences'))[0]?.[0] as { experimentalViews: string[] } | undefined)?.experimentalViews).toEqual(['map'])
  })
})

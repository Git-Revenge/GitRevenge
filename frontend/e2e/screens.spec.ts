import { expect, test, type Locator, type Page } from '@playwright/test'
import { branch, calls, events, open, pickView, remote, tag, textUnderFloor } from './support'

test.describe('commit menu', () => {
  // #505
  test('opens the worktree of a branch checked out in one', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: [branch('topic', { worktree: '/wt/topic' })] } })
    await page.getByRole('menuitem', { name: 'topic', exact: true }).click()
    await expect(page.getByRole('menuitem', { name: 'Checkout topic' })).toHaveCount(0)
    await page.getByRole('menuitem', { name: 'Open the worktree of topic' }).click()
    await expect.poll(() => calls(page, 'OpenRepo')).toEqual([['/wt/topic']])
  })

  // #323: repeated labels threw each_key_duplicate, and no menu appeared.
  test('opens on a commit with two or more branches', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: [branch('main', { head: true }), remote('origin/main')] } })
    await expect(page.getByRole('menu')).toBeVisible()
    await expect(page.getByRole('menuitem').first()).toBeVisible()
    const labels = await page.getByRole('menuitem').allTextContents()
    expect(new Set(labels).size).toBe(labels.length)
  })

  test('opens on a commit with a branch and a tag', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: [branch('release/v1'), tag('v1.0')] } })
    await expect(page.getByRole('menu')).toBeVisible()
  })

  const twoBranches = { responses: { Refs: [branch('main', { head: true }), remote('origin/main'), tag('v1.0')] } }

  // #385: a label too long for the menu is cut off, and then its tooltip says it in full.
  test.describe('a long label', () => {
    const merge = (name: string) => `Merge ${name} into main…`
    const opened = async (page: Page, name: string) => {
      await open(page, 'commitMenu', { menuX: 20, responses: { Refs: [branch('main', { head: true }), branch(name)] } })
      await page.getByRole('menuitem', { name, exact: true }).hover()
      const item = page.getByRole('menuitem', { name: merge(name) })
      await item.hover()
      return item
    }
    const cutOff = (item: Awaited<ReturnType<typeof opened>>) => item.locator('.label').evaluate((e) => e.scrollWidth > e.clientWidth)

    test('fits for a typical branch name, with no tooltip', async ({ page }) => {
      const item = await opened(page, 'feature/metric-units')
      expect(await cutOff(item)).toBe(false)
      await expect(item).toHaveAttribute('title', '')
    })

    test('has its full text as a tooltip when it is cut off', async ({ page }) => {
      const name = `feature/${'a-very-long-branch-name-'.repeat(3)}end`
      const item = await opened(page, name)
      expect(await cutOff(item)).toBe(true)
      await expect(item).toHaveAttribute('title', merge(name))
    })
  })

  test('keeps a commit menu short, with a row per ref', async ({ page }) => {
    await open(page, 'commitMenu', twoBranches)
    const labels = await page.getByRole('menuitem').allTextContents()
    expect(labels.length).toBeLessThan(12)
    await expect(page.getByRole('menuitem', { name: 'origin/main' })).toHaveAttribute('aria-haspopup', 'menu')
  })

  test('hovering a ref opens its submenu, and moving away closes it', async ({ page }) => {
    await open(page, 'commitMenu', twoBranches)
    await page.getByRole('menuitem', { name: 'origin/main' }).hover()
    await expect(page.getByRole('menu')).toHaveCount(2)
    await expect(page.getByRole('menuitem', { name: 'Delete origin/main from origin' })).toBeVisible()
    await page.getByRole('menuitem', { name: /Create branch here/ }).hover()
    await expect(page.getByRole('menu')).toHaveCount(1)
  })

  test('the arrow keys go in and out of a submenu, and Esc only closes that level', async ({ page }) => {
    await open(page, 'commitMenu', twoBranches)
    await expect(page.getByRole('menu')).toBeVisible()
    // The refs are last: v1.0, then origin/main above it.
    await page.keyboard.press('End')
    await page.keyboard.press('ArrowUp')
    await expect(page.getByRole('menuitem', { name: 'origin/main' })).toBeFocused()
    await page.keyboard.press('ArrowRight')
    await expect(page.getByRole('menu')).toHaveCount(2)
    await expect(page.getByRole('menuitem', { name: 'Checkout origin/main' })).toBeFocused()
    await page.keyboard.press('ArrowLeft')
    await expect(page.getByRole('menu')).toHaveCount(1)
    await expect(page.getByRole('menuitem', { name: 'origin/main' })).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('menu')).toHaveCount(2)
    await page.keyboard.press('Escape')
    await expect(page.getByRole('menu')).toHaveCount(1)
    expect(await events(page, 'close')).toHaveLength(0)
    await page.keyboard.press('Escape')
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('choosing from a submenu runs it and closes everything', async ({ page }) => {
    await open(page, 'commitMenu', twoBranches)
    await page.getByRole('menuitem', { name: 'v1.0' }).click()
    await page.getByRole('menuitem', { name: 'Push tag v1.0 to origin' }).click()
    expect(await events(page, 'close')).toHaveLength(1)
    await expect.poll(() => calls(page, 'PushRef')).toHaveLength(1)
  })

  test('clicking a submenu row keeps the menu open', async ({ page }) => {
    await open(page, 'commitMenu', twoBranches)
    await page.getByRole('menuitem', { name: 'origin/main' }).click()
    await expect(page.getByRole('menu')).toHaveCount(2)
    expect(await events(page, 'close')).toHaveLength(0)
  })

  test('a submenu opens to the left when there is no room on the right', async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 700 })
    await open(page, 'commitMenu', { ...twoBranches, menuX: 500 })
    await page.getByRole('menuitem', { name: 'origin/main' }).hover()
    await expect(page.getByRole('menu')).toHaveCount(2)
    const [parent, child] = await page.getByRole('menu').evaluateAll((m) => m.map((x) => x.getBoundingClientRect().toJSON()))
    expect(child.right).toBeLessThanOrEqual(800)
    expect(child.left).toBeLessThan(parent.left)
  })

  test('stays inside a short window', async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 400 })
    await open(page, 'commitMenu', { responses: { Refs: [branch('one'), branch('two'), remote('origin/one'), tag('v1')] } })
    const box = (await page.getByRole('menu').boundingBox())!
    expect(box.y).toBeGreaterThanOrEqual(0)
    expect(box.y + box.height).toBeLessThanOrEqual(400)
  })
})

/** A timeline row's height. In the repository view the working tree is the graph's first row,
 * so the first commit starts this far down. */
const ROW = 52

test.describe('commit graph', () => {
  /** The row just inside the top edge, read by right-clicking it. */
  async function topRow(page: Page) {
    const box = (await page.locator('.scroller').boundingBox())!
    // Just under the day line, which is the first 26px of the view: in its place at the top of
    // the history, and pinned there anywhere else.
    await page.mouse.click(box.x + 400, box.y + 26 + 8, { button: 'right' })
    return (await events(page, 'context-row')).at(-1)?.[0]
  }

  // #506
  test.describe('focused on a branch', () => {
    const scenario = (focus: unknown) => ({ graphRows: 3, responses: { CommitSignatures: [], Refs: [], GraphFocus: focus } })

    test('says so, and stops when asked', async ({ page }) => {
      await open(page, 'graph', scenario({ branch: 'topic', stack: false }))
      await expect(page.getByText('Focused on topic')).toBeVisible()
      await page.getByRole('button', { name: 'Stop focusing' }).click()
      await expect.poll(() => calls(page, 'SetGraphFocus')).toEqual([['/repo', '', false]])
    })

    test('names a stack', async ({ page }) => {
      await open(page, 'graph', scenario({ branch: 'topic', stack: true }))
      await expect(page.getByText('Focused on the stack of topic')).toBeVisible()
    })

    test('says nothing without a focus', async ({ page }) => {
      for (const focus of [null, { branch: '', stack: false }]) {
        await open(page, 'graph', scenario(focus))
        await expect(page.getByText('3 commits')).toBeVisible()
        await expect(page.getByRole('button', { name: 'Stop focusing' })).toHaveCount(0)
      }
    })

    test('keeps the header on one line with a long name in a narrow column', async ({ page }) => {
      await page.setViewportSize({ width: 640, height: 600 })
      await open(page, 'graph', scenario({ branch: 'feature/a-very-long-branch-name-that-goes-on-and-on-and-on-and-on-and-on', stack: false }))
      await expect(page.getByRole('button', { name: 'Stop focusing' })).toBeVisible()
      expect(await page.locator('.status').evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)
    })
  })

  test.describe('the Kinds filter', () => {
    const refs = [branch('main', { branchKind: 'trunk' }), branch('docs/site', { branchKind: 'docs' }), branch('dependabot/x', { branchKind: 'bot' }), branch('plain')]
    const shown = { graphRows: 3, responses: { CommitSignatures: [], Refs: refs, RepoPreferences: { hideKinds: [] } } }

    test('has a checkbox for each kind in the repository, and hides one when it is cleared', async ({ page }) => {
      await open(page, 'graph', shown)
      await page.getByRole('button', { name: 'Kinds' }).click()
      await expect(page.getByRole('checkbox')).toHaveCount(3)
      const docs = page.getByRole('checkbox', { name: 'Docs' })
      await expect(docs).toBeChecked()
      await docs.click()
      await expect.poll(() => calls(page, 'SetKindView')).toEqual([['/repo', 'docs', 'hide']])
    })

    test('says when a kind is filtered out, and can bring it back', async ({ page }) => {
      await open(page, 'graph', { graphRows: 3, responses: { CommitSignatures: [], Refs: [branch('main', { branchKind: 'trunk' })], RepoPreferences: { hideKinds: ['docs'] } } })
      const button = page.getByRole('button', { name: /Kinds/ })
      await expect(button).toHaveText(/1 hidden/)
      await button.click()
      const docs = page.getByRole('checkbox', { name: 'Docs' })
      await expect(docs).not.toBeChecked()
      await docs.click()
      // Brought back as it was: it doesn't fold here, since the stub names no kinds that do.
      await expect.poll(() => calls(page, 'SetKindView')).toEqual([['/repo', 'docs', 'show']])
    })

    test('closes on Escape, handing focus back, and on a click elsewhere', async ({ page }) => {
      await open(page, 'graph', shown)
      const button = page.getByRole('button', { name: 'Kinds' })
      await button.click()
      await expect(page.getByRole('checkbox', { name: 'Trunk' })).toBeFocused()
      await page.keyboard.press('Escape')
      await expect(page.getByRole('checkbox')).toHaveCount(0)
      await expect(button).toBeFocused()
      await button.click()
      await page.locator('.scroller').click({ position: { x: 400, y: 200 } })
      await expect(page.getByRole('checkbox')).toHaveCount(0)
    })

    test('stays away when the repository has no kinds', async ({ page }) => {
      await open(page, 'graph', { graphRows: 3, responses: { CommitSignatures: [], Refs: [branch('plain')], RepoPreferences: { hideKinds: [] } } })
      await expect(page.getByText('3 commits')).toBeVisible()
      await expect(page.getByRole('button', { name: /Kinds/ })).toHaveCount(0)
    })
  })

  // #383: the new toggles took the room the count had, and the search count ran off the edge.
  test.describe('in a narrow column', () => {
    const narrow = { graphRows: 32, responses: { CommitSignatures: [], Refs: [branch('docs/site', { branchKind: 'docs' }), branch('dependabot/x', { branchKind: 'bot' })] } }

    test('the header keeps the commit count on one line', async ({ page }) => {
      await page.setViewportSize({ width: 640, height: 600 })
      await open(page, 'graph', narrow)
      await expect(page.getByRole('button', { name: 'Kinds' })).toBeVisible()
      const count = page.getByText('32 commits')
      await expect(count).toBeVisible()
      const box = (await count.boundingBox())!
      expect(box.height).toBeLessThan(24)
      expect(await page.locator('.status').evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)
    })

    test('the search count stays inside the column', async ({ page }) => {
      await page.setViewportSize({ width: 640, height: 600 })
      await open(page, 'graph', { ...narrow, responses: { ...narrow.responses, SearchGraph: { rows: [], total: 0 } } })
      await page.getByRole('searchbox', { name: 'Search commits' }).fill('nothing like this')
      const count = page.getByText('No matches')
      await expect(count).toBeVisible()
      const box = (await count.boundingBox())!
      const next = (await page.getByRole('button', { name: 'Next match' }).boundingBox())!
      expect(box.x + box.width).toBeLessThanOrEqual(next.x)
      expect(next.x + next.width).toBeLessThanOrEqual(640)
    })

    // #626: the bar never wraps. The count goes into the search's placeholder, then the search
    // becomes an icon.
    test('the count moves into the search placeholder when there is no room for both', async ({ page }) => {
      await page.setViewportSize({ width: 420, height: 600 })
      await open(page, 'graph', narrow)
      const search = page.getByRole('searchbox', { name: 'Search commits' })
      await expect(search).toHaveAttribute('placeholder', 'Search 32 commits')
      await expect(page.locator('.status .meta')).toHaveCount(0)
      const status = page.locator('.status')
      expect((await status.boundingBox())!.height).toBeLessThan(50)
      expect(await status.evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)
    })

    test('the search is an icon when even that is too much, and opens over the bar', async ({ page }) => {
      await page.setViewportSize({ width: 360, height: 600 })
      await open(page, 'graph', { ...narrow, responses: { ...narrow.responses, SearchGraph: { rows: [1], total: 1 } } })
      const status = page.locator('.status')
      const opener = page.getByRole('button', { name: 'Search commits' })
      const search = page.getByRole('searchbox', { name: 'Search commits' })
      await expect(opener).toBeVisible()
      await expect(search).toBeHidden()
      expect((await status.boundingBox())!.height).toBeLessThan(50)
      expect(await status.evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)

      await opener.click()
      await expect(search).toBeFocused()
      await expect(search).toHaveAttribute('placeholder', 'Search 32 commits')
      // It takes the bar's whole width, on the same line.
      const bar = (await status.boundingBox())!
      const field = (await page.locator('.search').boundingBox())!
      expect(field.width).toBeGreaterThanOrEqual(bar.width - 1)
      expect(field.height).toBeLessThanOrEqual(bar.height)

      // Leaving it empty closes it; with something typed it stays, so the matches can be stepped.
      await page.locator('.scroller').focus()
      await expect(opener).toBeVisible()
      await opener.click()
      await search.fill('Commit')
      await page.locator('.scroller').focus()
      await expect(search).toBeVisible()
      await expect(page.getByRole('button', { name: 'Next match' })).toBeVisible()

      // Escape, or its close button, clears it and puts the icon back.
      await page.getByRole('button', { name: 'Close search' }).click()
      await expect(opener).toBeVisible()
      await expect(search).toBeHidden()
      await expect(page.locator('.scroller')).toBeFocused()
    })
  })

  // #555: a row's actions appear on hover, so right-click isn't the only way in.
  test('hovering a row shows its actions, and More opens the same menu as right-click', async ({ page }) => {
    await open(page, 'graph', { graphRows: 5, responses: { Refs: [], CommitSignatures: [] } })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.move(box.x + 400, box.y + 30)
    const actions = page.getByRole('group', { name: /^Actions for/ })
    await expect(actions).toBeVisible()
    // Leaving the graph takes the card away, and coming back brings it.
    await page.mouse.move(box.x + 400, box.y - 40)
    await expect(actions).toHaveCount(0)
    await page.mouse.move(box.x + 400, box.y + 30)
    await expect(actions).toBeVisible()
    await actions.getByRole('button', { name: /^Copy/ }).click()
    await expect(actions.getByRole('button', { name: /^Copied/ })).toBeVisible()
    await actions.getByRole('button', { name: /^Menu for/ }).click()
    await expect.poll(async () => (await events(page, 'context-row')).at(-1)?.[0]).toBe(0)
    // Once its buttons have focus the card stays for the keyboard, pointer or not. (Focused
    // explicitly: WebKit doesn't focus a button on click.)
    await actions.getByRole('button', { name: /^Cop/ }).focus()
    await page.mouse.move(box.x + 400, box.y - 40)
    await expect(actions).toBeVisible()
  })

  // #340: reaching the top was ignored, leaving part of the first row cut off.
  test('scrolling back up reaches the first row', async ({ page }) => {
    await open(page, 'graph', { graphRows: 500, responses: { Refs: [], CommitSignatures: [] } })
    const scroller = page.locator('.scroller')
    await expect(scroller).toHaveAttribute('aria-rowcount', '500')
    const box = (await scroller.boundingBox())!
    await page.mouse.move(box.x + 400, box.y + 200)
    for (const delta of [120, 70, -60, -500]) {
      await page.mouse.wheel(0, delta)
      await page.waitForTimeout(120)
    }
    await expect.poll(() => scroller.evaluate((e) => e.scrollTop)).toBe(0)
    expect(await topRow(page)).toBe(0)
  })

  // #354: a failed fetch used to leave blank rows and no message.
  test('says so when rows can\'t be read', async ({ page }) => {
    await open(page, 'graph', { graphRows: 500, responses: { Refs: [], CommitSignatures: [] }, failures: { GraphRows: 'the repository has gone away' } })
    await expect(page.getByText("Couldn't read history: the repository has gone away")).toBeVisible()
  })

  // #411: reading a busy repository, with lanes, labels and columns.
  test.describe('with many branches', () => {
    const lanes = (columns: number[]) => columns.map((c) => ({ from: c, to: c, color: c }))
    /** Three lanes: 0 is idle, 1 is idle, and 2 holds every commit. */
    const graphData = Array.from({ length: 20 }, (_, i) => ({
      index: i,
      col: 2,
      color: 2,
      up: i ? lanes([0, 1, 2]) : lanes([0, 1]),
      down: lanes([0, 1, 2]),
      width: 3,
      id: String(i).padStart(40, '0'),
      parents: 1,
      author: 'Ana Cruz',
      authorTime: 1793500000 - i * 60,
      subject: `Commit ${i}`,
    }))
    const scenario = { graphData, responses: { Refs: [], CommitSignatures: [], SearchGraph: { rows: [1], total: 1 } } }
    // The timeline's lanes: 150 of labels, 10 of padding, then 16 a lane.
    const lane = (col: number) => 150 + 10 + 16 * col + 8

    /** How much ink is on a pixel: the canvas is clear where nothing is drawn. */
    const inkAt = (page: Page, x: number, y: number) =>
      page.locator('.viewport > canvas').evaluate(
        (el, [px, py]) => {
          const dpr = window.devicePixelRatio || 1
          return (el as HTMLCanvasElement).getContext('2d')!.getImageData(px * dpr, py * dpr, 1, 1).data[3]
        },
        [x, y],
      )

    // #626: compact is a table, so its columns are named; the timeline has one column of words.
    test('headers name the columns in compact, and the timeline has none', async ({ page }) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      const header = page.locator('.columns')
      await expect(header).toHaveCount(0)
      await pickView(page, 'Compact')
      for (const name of ['Branch / tag', 'Message', 'Author', 'When', 'SHA']) await expect(header.getByText(name, { exact: true })).toBeVisible()
    })

    test('a narrow window has no column headers', async ({ page }) => {
      await page.setViewportSize({ width: 600, height: 600 })
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      await page.waitForTimeout(200)
      await expect(page.locator('.columns')).toHaveCount(0)
    })

    test('idle lanes draw quieter than the one with commits', async ({ page }) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      await expect.poll(() => inkAt(page, lane(2), 78 + 22)).toBeGreaterThan(0)
      const busy = await inkAt(page, lane(2), 78 + 22)
      const idle = await inkAt(page, lane(1), 78 + 22)
      expect(idle).toBeGreaterThan(0)
      expect(busy).toBeGreaterThan(idle * 2)
    })

    // #625: the band of lane colour behind a node, and the line tying a label to it, are gone.
    test('nothing is drawn between a node and its message, or between the labels and the lanes', async ({ page }) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      // The middle of the third commit's row, an unselected one that carries on a run: under
      // the day line (26), the first commit (52) and the second (30).
      const y = 26 + 52 + 30 + 15
      // The lane itself draws, so the row has painted by now.
      await expect.poll(() => inkAt(page, lane(2), y)).toBeGreaterThan(0)
      // Rows fade up into place as they arrive, a few px low at first: let that finish, or a
      // row's wash can still be where it will not stay.
      await page.waitForTimeout(600)
      // Between the node and the message.
      expect(await inkAt(page, lane(2) + 15, y)).toBe(0)
      // Between two idle lanes, and in the label column of a row with no labels.
      expect(await inkAt(page, lane(1) + 8, y)).toBe(0)
      expect(await inkAt(page, 100, y)).toBe(0)
    })

    test("the author button searches for the selected commit's author", async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 800 })
      await open(page, 'graph', scenario)
      await pickView(page, 'Compact')
      await page.getByRole('button', { name: "Search for the selected commit's author" }).click()
      await expect(page.getByRole('searchbox', { name: 'Search commits' })).toHaveValue('Ana Cruz')
      await expect.poll(() => calls(page, 'SearchGraph')).toContainEqual(['/repo', 'Ana Cruz'])
    })

    test('the message column stays put when a wider row scrolls by', async ({ page }) => {
      const wideLater = graphData.map((r, i) => (i === 19 ? { ...r, width: 9 } : r))
      await page.setViewportSize({ width: 1280, height: 300 })
      await open(page, 'graph', { ...scenario, graphData: wideLater })
      // Compact names its columns, which is how this test finds the message column.
      await pickView(page, 'Compact')
      const message = page.locator('.columns').getByText('Message')
      await expect(message).toBeVisible()
      const before = (await message.boundingBox())!.x
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.move(box.x + 400, box.y + 100)
      await page.mouse.wheel(0, 2000)
      // The wider row widens the graph once, and the column holds from then on.
      await expect.poll(async () => (await message.boundingBox())!.x).toBeGreaterThan(before)
      const widened = (await message.boundingBox())!.x
      await page.mouse.wheel(0, -2000)
      await page.waitForTimeout(150)
      expect((await message.boundingBox())!.x).toBe(widened)
    })
  })

  // #627: the ruler is the scrollbar, with the history marked on it.
  test.describe('the ruler', () => {
    const id = (i: number) => String(i).padStart(40, '0')
    const marks = { loaded: 1000, months: [{ row: 0, time: 1793500000 }, { row: 600, time: 1790000000 }], tips: [0, 400], tags: [401, 402, 800], notes: [400] }
    const scenario = (rows = 1000, over: object = {}) => ({
      graphRows: rows,
      responses: { CommitSignatures: [], Refs: [{ ...branch('main', { head: true, branchKind: 'trunk' }), commit: id(0) }], HeadCommit: { branch: 'main', commit: id(0) }, RulerMarks: marks, ...over },
    })
    const ruler = (page: Page) => page.getByRole('scrollbar', { name: 'Position in the history' })
    const ready = async (page: Page, rows = 1000, over: object = {}) => {
      await open(page, 'graph', scenario(rows, over))
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', String(rows))
      await expect.poll(() => calls(page, 'RulerMarks')).toContainEqual(['/repo'])
      await page.waitForTimeout(300)
    }
    const selected = async (page: Page) => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index
    /** The commit just under the pinned day line, found by clicking it. */
    const topCommit = async (page: Page) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + 26 + 10)
      return selected(page)
    }
    const boxOf = async (page: Page) => (await page.locator('.ruler-box').boundingBox())!

    test('stands in for the scrollbar: it says where the view is, and the browser draws none', async ({ page }) => {
      await ready(page)
      await expect(ruler(page)).toHaveAttribute('aria-valuenow', '0')
      await expect(ruler(page)).toHaveAttribute('aria-valuetext', 'Commit 1 of 1,000')
      await expect(ruler(page)).toHaveAttribute('aria-orientation', 'vertical')
      // It names the rows it scrolls, and isn't a Tab stop.
      const controls = await ruler(page).getAttribute('aria-controls')
      expect(await page.locator('.scroller').getAttribute('id')).toBe(controls)
      await expect(ruler(page)).toHaveAttribute('tabindex', '-1')
      // The rows end where it begins, and the scroll area shows no bar of its own.
      const r = (await ruler(page).boundingBox())!
      const view = (await page.locator('.scroller').boundingBox())!
      expect(Math.round(r.width)).toBe(14)
      expect(Math.round(r.x + r.width)).toBe(Math.round(view.x + view.width))
      expect(await page.locator('.scroller').evaluate((e) => getComputedStyle(e).scrollbarWidth)).toBe('none')
      // The box is at the top, and not too small to take hold of.
      const b = await boxOf(page)
      expect(Math.round(b.y)).toBe(Math.round(r.y))
      expect(b.height).toBeGreaterThanOrEqual(24)
    })

    test('the box follows the view, and reaches the bottom at the end', async ({ page }) => {
      await ready(page)
      await page.locator('.scroller').focus()
      await page.keyboard.press('End')
      await expect(ruler(page)).toHaveAttribute('aria-valuenow', '100')
      const r = (await ruler(page).boundingBox())!
      await expect.poll(async () => Math.round((await boxOf(page)).y + (await boxOf(page)).height)).toBe(Math.round(r.y + r.height))
      await page.keyboard.press('Home')
      await expect(ruler(page)).toHaveAttribute('aria-valuenow', '0')
    })

    test('dragging the box moves through the history, and says the month beside it', async ({ page }) => {
      await ready(page)
      const r = (await ruler(page).boundingBox())!
      const b = await boxOf(page)
      await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2)
      await page.mouse.down()
      await page.mouse.move(b.x + b.width / 2, r.y + r.height * 0.75, { steps: 6 })
      // While it's held, the month under it shows beside it.
      await expect(page.locator('.ruler-when')).toBeVisible()
      await expect(page.locator('.ruler-when')).toHaveText(/\d{4}/)
      await page.mouse.up()
      await expect(page.locator('.ruler-when')).toHaveCount(0)
      const now = () => ruler(page).getAttribute('aria-valuenow').then(Number)
      await expect.poll(now).toBeGreaterThan(55)
      expect(await now()).toBeLessThan(95)
      // The view really is there: the commit at the top is well into the history.
      const top = (await topCommit(page))!
      expect(top).toBeGreaterThan(500)
      expect(top).toBeLessThan(950)
    })

    test('a press on the track brings the box there', async ({ page }) => {
      await ready(page)
      const r = (await ruler(page).boundingBox())!
      await page.mouse.click(r.x + r.width / 2, r.y + r.height / 2)
      // The box and what the ruler says follow on the next paint.
      const now = () => ruler(page).getAttribute('aria-valuenow').then(Number)
      await expect.poll(now).toBeGreaterThan(40)
      expect(await now()).toBeLessThan(60)
      await expect.poll(async () => {
        const b = await boxOf(page)
        return Math.abs(b.y + b.height / 2 - (r.y + r.height / 2))
      }).toBeLessThan(3)
    })

    test('a tick says what is there when the pointer is on it', async ({ page }) => {
      await ready(page)
      const r = (await ruler(page).boundingBox())!
      // Rows 400, 401 and 402 of 1000 are within 2px of each other: one tick for a branch, two
      // tags and a note.
      await page.mouse.move(r.x + r.width / 2, r.y + (400 / 1000) * r.height + 1)
      await expect(ruler(page)).toHaveAttribute('title', '1 branch, 2 tags, 1 note')
      // The checked-out commit and its branch are at the very top.
      await page.mouse.move(r.x + r.width / 2, r.y + 1)
      await expect(ruler(page)).toHaveAttribute('title', 'the checked-out commit, 1 branch')
      await page.mouse.move(r.x + r.width / 2, r.y + r.height * 0.6)
      await expect(ruler(page)).toHaveAttribute('title', '')
    })

    test('search matches are marked on it', async ({ page }) => {
      await ready(page, 1000, { SearchGraph: { rows: [650, 651], total: 2 } })
      await page.getByRole('searchbox', { name: 'Search commits' }).fill('something')
      await expect.poll(() => calls(page, 'SearchGraph')).toHaveLength(1)
      const r = (await ruler(page).boundingBox())!
      await page.mouse.move(r.x + r.width / 2, r.y + (650 / 1000) * r.height + 1)
      await expect(ruler(page)).toHaveAttribute('title', '2 matches')
    })

    test('works to the far end of a very long history', async ({ page }) => {
      await ready(page, 1_500_000)
      await expect(ruler(page)).toHaveAttribute('aria-valuetext', 'Commit 1 of 1,500,000')
      const r = (await ruler(page).boundingBox())!
      const b = await boxOf(page)
      expect(Math.round(b.height)).toBe(24)
      await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2)
      await page.mouse.down()
      await page.mouse.move(b.x + b.width / 2, r.y + r.height + 50, { steps: 8 })
      await page.mouse.up()
      await expect(ruler(page)).toHaveAttribute('aria-valuenow', '100')
      await page.waitForTimeout(400)
      const view = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(view.x + 300, view.y + view.height - 10)
      expect(await selected(page)).toBe(1_499_999)
    })

    test('the wheel over it scrolls the history', async ({ page }) => {
      await ready(page)
      const r = (await ruler(page).boundingBox())!
      await page.mouse.move(r.x + r.width / 2, r.y + r.height / 2)
      await page.mouse.wheel(0, 900)
      await page.waitForTimeout(500)
      expect((await topCommit(page))!).toBeGreaterThan(20)
    })
  })

  // #624: a Bot or Docs lane's commits take one line a day.
  test.describe('folded lanes', () => {
    const id = (i: number) => String(i).padStart(40, '0')
    const noon = (daysAgo: number) => {
      const d = new Date()
      d.setHours(12, 0, 0, 0)
      d.setDate(d.getDate() - daysAgo)
      return Math.floor(d.getTime() / 1000)
    }
    // Newest first. Today: main, the docs lane's tip, then main and docs commits between each
    // other. Yesterday: two more docs commits among main's.
    const lanes = ['m', 'd', 'm', 'd', 'm', 'd', 'd', 'm', 'd', 'm', 'd', 'm', 'm', 'm']
    const graphData = lanes.map((lane, i) => ({
      index: i,
      col: lane === 'd' ? 1 : 0,
      color: lane === 'd' ? 1 : 0,
      up: i ? [{ from: 0, to: 0, color: 0, kind: 'trunk' }, { from: 1, to: 1, color: 1, kind: 'docs' }] : [],
      down: [{ from: 0, to: 0, color: 0, kind: 'trunk' }, { from: 1, to: 1, color: 1, kind: 'docs' }],
      width: 2,
      kind: lane === 'd' ? 'docs' : 'trunk',
      id: id(i),
      parents: 1,
      author: lane === 'd' ? 'pages-bot' : 'Ana Cruz',
      authorTime: (i < 8 ? noon(0) : noon(1)) - i * 60,
      subject: lane === 'd' ? `Deploying to gh-pages ${i}` : `Commit ${i}`,
    }))
    const refs = [branch('main', { head: true, branchKind: 'trunk' }), branch('gh-pages', { branchKind: 'docs' })]
    const graphRefs = { [id(0)]: [refs[0]], [id(1)]: [refs[1]] }
    const prefs = { hide: [], hideKinds: [], showKinds: [], foldableKinds: ['bot', 'docs'], kinds: [] }
    const scenario = (over: object = {}) => ({ graphData, graphRefs, responses: { Refs: refs, CommitSignatures: [], RepoPreferences: prefs, ...over } })
    const ready = async (page: Page, over: object = {}) => {
      await open(page, 'graph', scenario(over))
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '14')
      await expect(page.getByRole('button', { name: 'Kinds' })).toBeVisible()
      await page.waitForTimeout(300)
      await page.locator('.scroller').focus()
    }
    const selected = async (page: Page) => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index
    const clickAt = async (page: Page, y: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 400, box.y + y)
      return selected(page)
    }
    // Today, top to bottom: the day line (26), main's tip (52), the docs lane's tip (52: it
    // carries the label, so it stays a row), commit 2 (30), the fold's line (28), commit 4 (30),
    // commit 7 (30).
    const FOLD = 26 + 52 + 52 + 30
    const down = async (page: Page, times = 1) => {
      for (let i = 0; i < times; i++) await page.keyboard.press('ArrowDown')
      return selected(page)
    }

    test("a Docs lane's commits of the day are one line, which a click opens and closes", async ({ page }) => {
      await ready(page)
      // Under the fold's line is commit 4: commits 3, 5 and 6 are folded away.
      expect(await clickAt(page, FOLD + 28 + 15)).toBe(4)
      expect(await clickAt(page, FOLD + 28 + 30 + 15)).toBe(7)
      // A click on the line selects nothing, and opens it: its commits take their rows.
      const before = (await events(page, 'select')).length
      await clickAt(page, FOLD + 14)
      expect((await events(page, 'select')).length).toBe(before)
      expect(await clickAt(page, FOLD + 28 + 15)).toBe(3)
      expect(await clickAt(page, FOLD + 28 + 30 + 15)).toBe(4)
      // And closes it again.
      await clickAt(page, FOLD + 14)
      expect(await clickAt(page, FOLD + 28 + 30 + 15)).toBe(7)
    })

    test('the keyboard lands on a fold, opens it with Right and closes it with Left', async ({ page }) => {
      await ready(page)
      expect(await clickAt(page, 26 + 52 + 52 + 15)).toBe(2)
      // Down from commit 2 is the fold's newest commit, then past the rest to commit 4 and 7.
      expect(await down(page)).toBe(3)
      expect(await down(page)).toBe(4)
      expect(await down(page)).toBe(7)
      await page.keyboard.press('ArrowUp')
      await page.keyboard.press('ArrowUp')
      await expect.poll(() => selected(page)).toBe(3)
      // Open: every commit of the fold is a row to step through.
      await page.keyboard.press('ArrowRight')
      expect(await down(page)).toBe(4)
      expect(await down(page)).toBe(5)
      expect(await down(page)).toBe(6)
      // Closed again from inside it, the selected commit still shows, and the rest don't.
      await page.keyboard.press('ArrowLeft')
      expect(await down(page)).toBe(7)
    })

    test('a search match in a fold shows, so the search never lands on something hidden', async ({ page }) => {
      await ready(page, { SearchGraph: { rows: [5], total: 1 } })
      await page.getByRole('searchbox', { name: 'Search commits' }).fill('gh-pages 5')
      await expect.poll(() => calls(page, 'SearchGraph')).toHaveLength(1)
      await page.waitForTimeout(300)
      await page.locator('.scroller').focus()
      expect(await clickAt(page, 26 + 52 + 52 + 15)).toBe(2)
      expect(await down(page)).toBe(3)
      expect(await down(page)).toBe(4)
      // Commit 5 is a match, so it has a row; commit 6 isn't, and stays folded.
      expect(await down(page)).toBe(5)
      expect(await down(page)).toBe(7)
    })

    test('the day counts the commits that are somebody\'s work', async ({ page }) => {
      await ready(page)
      expect(await clickAt(page, FOLD + 28 + 30 + 15)).toBe(7)
      // Into yesterday: its fold's newest, commit 8, comes first.
      expect(await down(page)).toBe(8)
      // Four of yesterday's six commits are on main.
      await expect(page.getByRole('status')).toHaveText(/^Yesterday, .+, 4 commits$/)
    })

    test('compact folds too', async ({ page }) => {
      await ready(page)
      await pickView(page, 'Compact')
      await page.waitForTimeout(200)
      await page.locator('.scroller').focus()
      // Rows of 30: main's tip, the docs tip, commit 2, the fold's line (28), commit 4, commit 7.
      expect(await clickAt(page, 2 * 30 + 15)).toBe(2)
      expect(await clickAt(page, 3 * 30 + 28 + 15)).toBe(4)
      expect(await clickAt(page, 3 * 30 + 28 + 30 + 15)).toBe(7)
    })

    test('the Kinds menu folds a lane or shows it in full, for kinds that fold', async ({ page }) => {
      await ready(page)
      await page.getByRole('button', { name: 'Kinds' }).click()
      const fold = page.getByRole('button', { name: 'Fold Docs lanes to one line a day' })
      await expect(fold).toHaveAttribute('aria-pressed', 'true')
      await expect(page.getByRole('button', { name: /^Fold Trunk/ })).toHaveCount(0)
      await fold.click()
      await expect.poll(() => calls(page, 'SetKindView')).toEqual([['/repo', 'docs', 'show']])
    })

    test('a lane shown in full has no folds', async ({ page }) => {
      await ready(page, { RepoPreferences: { ...prefs, showKinds: ['docs'] } })
      // Where the fold's line was is commit 3, with a row like any other.
      expect(await clickAt(page, 26 + 52 + 52 + 15)).toBe(2)
      expect(await down(page)).toBe(3)
      expect(await down(page)).toBe(4)
      expect(await down(page)).toBe(5)
      await page.getByRole('button', { name: 'Kinds' }).click()
      await expect(page.getByRole('button', { name: 'Fold Docs lanes to one line a day' })).toHaveAttribute('aria-pressed', 'false')
    })
  })

  // #623: the day is a line of its own above its first commit, and stays at the top of the view
  // while its commits scroll past.
  test.describe('day lines', () => {
    const noon = (daysAgo: number) => {
      const d = new Date()
      d.setHours(12, 0, 0, 0)
      d.setDate(d.getDate() - daysAgo)
      return Math.floor(d.getTime() / 1000)
    }
    /** Ten commits today by one person, then sixty yesterday by two. */
    const graphData = Array.from({ length: 70 }, (_, i) => ({
      index: i,
      col: 0,
      color: 0,
      up: i ? [{ from: 0, to: 0, color: 0 }] : [],
      down: [{ from: 0, to: 0, color: 0 }],
      width: 1,
      id: String(i).padStart(40, '0'),
      parents: 1,
      author: i >= 10 && i % 2 ? 'Ben Ode' : 'Ana Cruz',
      authorTime: (i < 10 ? noon(0) : noon(1)) - i * 60,
      subject: `Commit ${i}`,
    }))
    const scenario = { graphData, responses: { Refs: [], CommitSignatures: [] } }
    const ready = async (page: Page) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '70')
      await page.waitForTimeout(300)
    }
    /** How much ink is in a strip of the canvas: its left third at a height, 4px tall. */
    const inkAcross = (page: Page, y: number) =>
      page.locator('.viewport > canvas').evaluate((el, py) => {
        const c = el as HTMLCanvasElement
        const dpr = window.devicePixelRatio || 1
        const data = c.getContext('2d')!.getImageData(0, py * dpr, c.width / 3, 4 * dpr).data
        let ink = 0
        for (let i = 3; i < data.length; i += 4) ink += data[i] ? 1 : 0
        return ink / (data.length / 4)
      }, y)
    const selected = async (page: Page) => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index
    const clickAt = async (page: Page, y: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + y)
      return selected(page)
    }

    test('the first commit sits under its day line, which is not a commit', async ({ page }) => {
      await ready(page)
      // A click on the day line selects nothing new; under it is the first commit.
      const before = (await events(page, 'select')).length
      await clickAt(page, 10)
      expect((await events(page, 'select')).length).toBe(before)
      expect(await clickAt(page, 26 + 10)).toBe(0)
      // The line in its place has no ground of its own: above its text is clear.
      expect(await inkAcross(page, 1)).toBe(0)
    })

    test("stays at the top while its day scrolls past, on a ground of its own, and gives way to the next day's", async ({ page }) => {
      await ready(page)
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.move(box.x + 300, box.y + 200)
      // Today is its line, a starter and nine followers: 26 + 52 + 9 * 30 = 348 tall.
      await page.mouse.wheel(0, 150)
      await page.waitForTimeout(600)
      // Pinned: the strip at the very top is filled edge to edge.
      expect(await inkAcross(page, 1)).toBe(1)
      // The row under the pinned line is still one of today's.
      expect(await clickAt(page, 26 + 4)).toBeLessThan(10)
      // Well into yesterday, its line is the one pinned, and the rows under it are yesterday's.
      await page.mouse.move(box.x + 300, box.y + 200)
      await page.mouse.wheel(0, 600)
      await page.waitForTimeout(600)
      expect(await inkAcross(page, 1)).toBe(1)
      expect(await clickAt(page, 26 + 4)).toBeGreaterThanOrEqual(10)
    })

    test('moving the selection into another day says which day, and what it holds', async ({ page }) => {
      await ready(page)
      await page.locator('.scroller').focus()
      expect(await clickAt(page, 26 + 52 + 8 * 30 + 15)).toBe(9)
      await expect(page.getByRole('status')).toHaveText('')
      await page.keyboard.press('ArrowDown')
      await expect.poll(() => selected(page)).toBe(10)
      await expect(page.getByRole('status')).toHaveText(/^Yesterday, .+, 60 commits · 2 people$/)
      // Within the day, nothing more is said; back into today, today is.
      await page.keyboard.press('ArrowDown')
      await expect(page.getByRole('status')).toHaveText(/^Yesterday, /)
      await page.keyboard.press('ArrowUp')
      await page.keyboard.press('ArrowUp')
      await expect(page.getByRole('status')).toHaveText(/^Today, .+, 10 commits$/)
    })

    test('a selection brought into view from above lands under the pinned line, not behind it', async ({ page }) => {
      await ready(page)
      await page.locator('.scroller').focus()
      await page.keyboard.press('End')
      await expect.poll(() => selected(page)).toBe(69)
      await page.keyboard.press('PageUp')
      await page.keyboard.press('PageUp')
      const at = (await selected(page))!
      // The selected row is the one just under the pinned day line.
      expect(await clickAt(page, 26 + 6)).toBe(at)
    })

    test('compact has no day lines', async ({ page }) => {
      await ready(page)
      await pickView(page, 'Compact')
      await page.waitForTimeout(200)
      // Under compact's column headings the first row is the first commit, and the eleventh
      // follows the tenth with nothing between.
      expect(await clickAt(page, 10)).toBe(0)
      expect(await clickAt(page, 10 * 30 + 10)).toBe(10)
    })
  })

  // #621: the view is placed by the row at its top edge, not by a height added up from the first
  // commit, so it has to behave the same at the far end of a history too long to measure.
  test.describe('a very long history', () => {
    const rows = 1_500_000
    const scenario = { graphRows: rows, responses: { CommitSignatures: [], Refs: [] } }
    const ready = async (page: Page) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', String(rows))
      await page.locator('.scroller').focus()
    }
    const selected = async (page: Page) => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index
    /** The commit whose row is at a height in the view, found by clicking it. */
    const commitAt = async (page: Page, y: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + y)
      return selected(page)
    }
    const scrollState = (page: Page) => page.locator('.scroller').evaluate((e) => ({ top: e.scrollTop, max: e.scrollHeight - e.clientHeight, view: e.clientHeight }))

    test('End goes to the last commit, at the bottom of the view, and Home comes back', async ({ page }) => {
      await ready(page)
      await page.keyboard.press('End')
      await expect.poll(() => selected(page)).toBe(rows - 1)
      const { top, max, view } = await scrollState(page)
      expect(Math.abs(top - max)).toBeLessThanOrEqual(1)
      // The last row's bottom edge is the view's: the row a little above the bottom is the last commit.
      expect(await commitAt(page, view - 10)).toBe(rows - 1)
      await page.keyboard.press('Home')
      await expect.poll(() => selected(page)).toBe(0)
      expect((await scrollState(page)).top).toBe(0)
      // The day line is first, then the first commit.
      expect(await commitAt(page, 26 + 10)).toBe(0)
    })

    test('the wheel moves through rows at its usual speed, wherever it is', async ({ page }) => {
      await ready(page)
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.move(box.x + 300, box.y + 100)
      // A row is taken to start a run until it has loaded, so let the first rows arrive.
      await page.waitForTimeout(400)
      // The first commit starts a run (52), and the stub's commits after it carry it on (30 each).
      // Its day line (26) scrolls off, and the same day's line is then pinned over the top 26px,
      // so the row found just under that is the twenty-second.
      const under = 26 + 10
      await page.mouse.wheel(0, 52 + 20 * 30)
      // A wheel notch is eased over a few frames. Finding the row by clicking it would stop it
      // part way, so wait for it to finish.
      const settle = () => page.waitForTimeout(600)
      await settle()
      // Twenty-one rows have gone by, not a ten-thousandth of the history.
      expect(await commitAt(page, under)).toBe(21)
      // The same from the middle: drag the scrollbar there, then wheel.
      await page.locator('.scroller').evaluate((e) => e.scrollTo(0, (e.scrollHeight - e.clientHeight) / 2))
      await page.waitForTimeout(500)
      const middle = (await commitAt(page, under))!
      expect(Math.abs(middle - rows / 2)).toBeLessThan(rows / 100)
      // Bring a row's top edge to just under the pinned day line, so whole rows can be counted.
      await page.keyboard.press('ArrowUp')
      await page.keyboard.press('ArrowUp')
      await page.keyboard.press('ArrowDown')
      const at = (await commitAt(page, under))!
      await page.mouse.move(box.x + 300, box.y + 100)
      await page.mouse.wheel(0, 600)
      await settle()
      // Twenty rows of 30, or nineteen and a day line if midnight falls among them.
      const down = (await commitAt(page, under))!
      expect([at + 19, at + 20]).toContain(down)
      await page.mouse.move(box.x + 300, box.y + 100)
      await page.mouse.wheel(0, -1200)
      await settle()
      const up = (await commitAt(page, under))!
      expect(up).toBeGreaterThanOrEqual(at - 21)
      expect(up).toBeLessThanOrEqual(at - 19)
    })

    test('the arrow keys keep the selection in view a row at a time', async ({ page }) => {
      await ready(page)
      await page.locator('.scroller').evaluate((e) => e.scrollTo(0, (e.scrollHeight - e.clientHeight) / 2))
      await page.waitForTimeout(150)
      const { view } = await scrollState(page)
      const bottom = (await commitAt(page, view - 60))!
      for (let i = 0; i < 4; i++) await page.keyboard.press('ArrowDown')
      await expect.poll(() => selected(page)).toBe(bottom + 4)
      // The selection went past the bottom edge, and the view followed it by just enough.
      await page.keyboard.press('ArrowUp')
      await expect.poll(() => selected(page)).toBe(bottom + 3)
      expect(await commitAt(page, view - 60)).toBe(bottom + 3)
    })
  })

  // #619: a way back to the checked-out commit, and the view follows a branch you move.
  test.describe('the checked-out commit', () => {
    const id = (i: number) => String(i).padStart(40, '0')
    const at = (row: number, behind = 0) => ({ HeadCommit: { branch: 'main', commit: id(row) }, Refs: [{ ...branch('main', { head: true, branchKind: 'trunk' }), commit: id(row), behind }] })
    const scenario = (row: number) => ({ graphRows: 400, responses: { CommitSignatures: [], ...at(row) } })
    const button = (page: Page) => page.getByRole('button', { name: /^Go to the checked-out commit/ })
    const scrollTop = (page: Page) => page.locator('.scroller').evaluate((e) => e.scrollTop)
    /** The commit whose row is at a height in the view, found by clicking it (which selects it). */
    const commitAt = async (page: Page, y: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + y)
      return ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index
    }
    const ready = async (page: Page, row: number) => {
      await open(page, 'graph', scenario(row))
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '400')
      await expect.poll(() => calls(page, 'HeadCommit')).toContainEqual(['/repo'])
    }
    /** The app has just run something that moves a branch, as a pull would have. */
    const acted = (page: Page) => page.evaluate(() => (window as unknown as { __respond: (r: object) => void }).__respond({ OwnMoveAge: 120 }))
    /** The repository as it is after a branch moved, and the reload that follows. */
    const reloadAt = (page: Page, row: number, arrived: number[], behind = 0) =>
      page.evaluate(
        ([answers, gained]) => {
          const w = window as unknown as { __respond: (r: object) => void; __emit: (n: string, d: object) => void }
          w.__respond({ ...(answers as object), ArrivedCommits: gained })
          w.__emit('graph:progress', { path: '/repo', rows: 400, done: true, paused: false, estimate: 400 })
        },
        [at(row, behind), { commits: arrived.map(id), total: arrived.length }] as const,
      )
    /** Ink at the right edge of the row drawn at a height in the view, where an arrived commit's bar is. */
    const barAt = (page: Page, y: number) =>
      page.locator('.viewport > canvas').evaluate((el, py) => {
        const c = el as HTMLCanvasElement
        const dpr = window.devicePixelRatio || 1
        return c.getContext('2d')!.getImageData(c.width - 2 * dpr, py * dpr, 1, 1).data[3]
      }, y)

    test('has no button while it is in view', async ({ page }) => {
      await ready(page, 0)
      await page.waitForTimeout(400)
      await expect(button(page)).toHaveCount(0)
    })

    test('a button goes back to it once it has scrolled away, and selects nothing', async ({ page }) => {
      await ready(page, 0)
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.move(box.x + 300, box.y + 200)
      await page.mouse.wheel(0, 3000)
      await expect(button(page)).toBeVisible()
      await expect(button(page)).toHaveAccessibleName('Go to the checked-out commit, main')
      await expect(button(page).locator('.arrow')).not.toHaveClass(/down/)
      // Clear of the SHA column and the row actions at the right.
      const b = (await button(page).boundingBox())!
      expect(b.x + b.width).toBeLessThanOrEqual(box.x + box.width - 130)
      expect(b.height).toBeGreaterThanOrEqual(32)
      const selections = (await events(page, 'select')).length
      await button(page).focus()
      await page.keyboard.press('Enter')
      await expect.poll(() => scrollTop(page)).toBe(0)
      await expect(button(page)).toHaveCount(0)
      await expect(page.locator('.scroller')).toBeFocused()
      expect((await events(page, 'select')).length).toBe(selections)
    })

    test('points down when the checked-out commit is below the view, and lands it a third of the way down', async ({ page }) => {
      await ready(page, 200)
      await expect(button(page)).toBeVisible()
      await expect(button(page).locator('.arrow')).toHaveClass(/down/)
      await button(page).click()
      await expect(button(page)).toHaveCount(0)
      const view = (await page.locator('.scroller').boundingBox())!.height
      // Its row starts a third of the way down: just under that line is the checked-out commit,
      // and just over it the commit before.
      await expect.poll(() => commitAt(page, view / 3 + 8)).toBe(200)
      expect(await commitAt(page, view / 3 - 8)).toBe(199)
    })

    test('a detached HEAD is named by its short SHA', async ({ page }) => {
      await open(page, 'graph', { graphRows: 400, responses: { CommitSignatures: [], Refs: [], HeadCommit: { branch: '', commit: id(200) } } })
      await expect(button(page)).toHaveAccessibleName('Go to the checked-out commit, 0000000')
    })

    test('the view follows a branch the person moved, marks what arrived, and selects the tip', async ({ page }) => {
      await ready(page, 200)
      await acted(page)
      await reloadAt(page, 150, [150, 151, 152])
      const view = (await page.locator('.scroller').boundingBox())!.height
      await expect(page.getByRole('status')).toHaveText('main moved to 0000000, 3 new commits')
      // Nothing had been chosen, so the tip is selected.
      await expect.poll(async () => (await events(page, 'select')).at(-1)?.[0]).toMatchObject({ index: 150 })
      // The tip is a third of the way down. The stub's rows carry no labels, so none is marked as
      // checked out and every commit here carries on a run: 30 tall. The three that arrived wear
      // a bar at the right edge; the row above them and the one below don't.
      const rowMid = (row: number) => Math.round(view / 3 + (row - 150) * 30 + 15)
      await expect.poll(() => barAt(page, rowMid(151))).toBeGreaterThan(0)
      expect(await barAt(page, rowMid(150))).toBeGreaterThan(0)
      expect(await barAt(page, rowMid(152))).toBeGreaterThan(0)
      expect(await barAt(page, rowMid(149))).toBe(0)
      expect(await barAt(page, rowMid(153))).toBe(0)
      // Selecting something else clears the marks.
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + rowMid(153))
      await expect.poll(() => barAt(page, rowMid(151))).toBe(0)
    })

    test('a commit the person selected stays selected, and only the view moves', async ({ page }) => {
      await ready(page, 200)
      const box = (await page.locator('.scroller').boundingBox())!
      // The fourth row: the day line, one that starts a run, then two that carry it on, are above it.
      await page.mouse.click(box.x + 300, box.y + 26 + 52 + 2 * 30 + 15)
      await expect.poll(async () => (await events(page, 'select')).at(-1)?.[0]).toMatchObject({ index: 3 })
      await acted(page)
      await reloadAt(page, 150, [150])
      await expect(page.getByRole('status')).toHaveText('main moved to 0000000, 1 new commit')
      await expect.poll(() => scrollTop(page)).toBeGreaterThan(0)
      await page.waitForTimeout(400)
      const moved = await scrollTop(page)
      expect((await events(page, 'select')).at(-1)?.[0]).toMatchObject({ index: 3 })
      // The selection is out of view now, and the view did not go back for it.
      await page.waitForTimeout(200)
      expect(await scrollTop(page)).toBe(moved)
      // The view is at the branch's new tip, a third of the way down. (Looked for last, since
      // finding it by clicking moves the selection.)
      expect(await commitAt(page, box.height / 3 + 8)).toBe(150)
    })

    test('a branch that moved without the person doing anything is not followed', async ({ page }) => {
      await ready(page, 200)
      await page.waitForTimeout(200)
      await reloadAt(page, 150, [150])
      await page.waitForTimeout(500)
      expect(await scrollTop(page)).toBe(0)
      await expect(page.getByRole('status')).toHaveText('')
    })

    test('after a fetch the view stays, and the button counts what is new upstream', async ({ page }) => {
      await ready(page, 200)
      await expect(button(page)).toHaveAccessibleName('Go to the checked-out commit, main')
      await acted(page)
      await reloadAt(page, 200, [], 3)
      await expect(button(page)).toHaveAccessibleName('Go to the checked-out commit, main, 3 new')
      await expect(button(page)).toContainText('3 new')
      expect(await scrollTop(page)).toBe(0)
    })

    test('with reduced motion the view jumps', async ({ page }) => {
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await ready(page, 60)
      await button(page).click()
      // No glide: it is there on the next frame.
      await page.waitForTimeout(50)
      const view = (await page.locator('.scroller').boundingBox())!.height
      expect(await commitAt(page, view / 3 + 8)).toBe(60)
    })
  })

  // #620: a commit with a git note says so in the graph.
  test.describe('note marks', () => {
    const id = (i: number) => String(i).padStart(40, '0')
    const notes = [
      { ref: 'refs/notes/commits', text: 'Deployed to staging.\nTwice.' },
      { ref: 'refs/notes/review', text: 'looks good' },
    ]
    const scenario = { graphRows: 5, responses: { Refs: [], CommitSignatures: [], NotedCommits: [id(1)], CommitNotes: notes, SearchGraph: { rows: [1], total: 1 } } }
    /** The note mark's slot on a row: 28px left of the SHA column, on the subject's line. */
    const slot = async (page: Page, row: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      // Under the day line (26), the first row starts a run: two lines, the mark on the first.
      // The stub's other commits carry it on: one line each, 30 tall.
      const y = 26 + (row === 0 ? 19 : 52 + (row - 1) * 30 + 15)
      // The rows end where the ruler (14px) begins; the SHA column (76) is at their right, and
      // the note mark's slot 28px before it.
      const x = box.width - 14 - 76 - 28
      return { x: box.x + x, y: box.y + y, left: x - 8, top: y - 8 }
    }
    /** How much ink is in a row's slot. */
    const inkIn = async (page: Page, row: number) => {
      const s = await slot(page, row)
      return page.locator('.viewport > canvas').evaluate(
        (el, [x, y]) => {
          const dpr = window.devicePixelRatio || 1
          const data = (el as HTMLCanvasElement).getContext('2d')!.getImageData(x * dpr, y * dpr, 16 * dpr, 16 * dpr).data
          let ink = 0
          for (let i = 3; i < data.length; i += 4) ink += data[i]
          return ink
        },
        [s.left, s.top],
      )
    }
    const ready = async (page: Page) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '5')
      await expect.poll(() => calls(page, 'NotedCommits')).toContainEqual(['/repo'])
    }

    test('a commit with a note wears a mark beside the SHA, and one without has none', async ({ page }) => {
      await ready(page)
      // Row 0 is selected, so compare two plain rows.
      await expect.poll(() => inkIn(page, 1)).toBeGreaterThan(0)
      expect(await inkIn(page, 2)).toBe(0)
    })

    test('hovering the mark shows the notes, naming a ref that is not the default', async ({ page }) => {
      await ready(page)
      await expect.poll(() => inkIn(page, 1)).toBeGreaterThan(0)
      const s = await slot(page, 1)
      await page.mouse.move(s.x, s.y)
      const card = page.locator('.note-card')
      await expect(card).toContainText('Deployed to staging.')
      await expect(card).toContainText('looks good')
      await expect(card.locator('.ref')).toHaveText(['review'])
      expect(await calls(page, 'CommitNotes')).toContainEqual(['/repo', id(1)])
      // It never takes the pointer, and goes when the pointer leaves the mark.
      expect(await card.evaluate((e) => getComputedStyle(e).pointerEvents)).toBe('none')
      await page.mouse.move(s.x - 200, s.y)
      await expect(card).toHaveCount(0)
      // A row with no note has nothing to show.
      const other = await slot(page, 2)
      await page.mouse.move(other.x, other.y)
      await page.waitForTimeout(100)
      await expect(card).toHaveCount(0)
    })

    test('clicking the mark selects the commit and asks for its note', async ({ page }) => {
      await ready(page)
      await expect.poll(() => inkIn(page, 1)).toBeGreaterThan(0)
      const s = await slot(page, 1)
      await page.mouse.click(s.x, s.y)
      await expect.poll(async () => (await events(page, 'note')).at(-1)?.[0]).toBe(1)
      // A click elsewhere on the row only selects it.
      await page.mouse.click(s.x - 300, s.y + 52)
      await page.waitForTimeout(100)
      expect((await events(page, 'note')).length).toBe(1)
    })

    test('the marks follow the notes when the graph reloads', async ({ page }) => {
      await ready(page)
      await expect.poll(() => inkIn(page, 1)).toBeGreaterThan(0)
      await page.evaluate((noted) => {
        const w = window as unknown as { __respond: (r: object) => void; __emit: (n: string, d: object) => void }
        w.__respond({ NotedCommits: noted })
        w.__emit('graph:progress', { path: '/repo', rows: 5, done: true, paused: false, estimate: 5 })
      }, [id(2)])
      await expect.poll(() => inkIn(page, 2)).toBeGreaterThan(0)
      expect(await inkIn(page, 1)).toBe(0)
    })

    test('has:note is sent to the search as typed', async ({ page }) => {
      await ready(page)
      await page.getByRole('searchbox', { name: 'Search commits' }).fill('has:note')
      await expect.poll(() => calls(page, 'SearchGraph')).toContainEqual(['/repo', 'has:note'])
      await expect(page.getByText('1 of 1').or(page.getByText(/^1$/))).toBeVisible()
    })
  })

  // #418: +N didn't say what it was hiding.
  test.describe('hovering over labels', () => {
    const id = '0'.repeat(40)
    const graphRefs = { [id]: [branch('main', { head: true, branchKind: 'trunk' }), remote('origin/main', { branchKind: 'trunk' }), remote('fork/main', { branchKind: 'trunk' }), tag('v1.0'), branch('feature/a-rather-long-branch-name')] }
    const scenario = { graphRows: 5, graphRefs, responses: { Refs: graphRefs[id], CommitSignatures: [] } }
    /** The first row's labels, in the label column. */
    const overLabels = async (page: Page) => {
      const box = (await page.locator('.scroller').boundingBox())!
      // The middle of the first commit's row, which is under the day line.
      return { x: box.x + 30, y: box.y + 26 + 26 }
    }
    const ready = async (page: Page) => {
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '5')
      await page.waitForTimeout(150)
    }

    test('shows every ref in full, and closes when the pointer leaves', async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      const { x, y } = await overLabels(page)
      await page.mouse.move(x, y)
      const list = page.locator('.ref-expand')
      for (const name of ['main', '◆ v1.0', 'feature/a-rather-long-branch-name']) await expect(list.getByText(name, { exact: true })).toBeVisible()
      // The same branch, wherever it lives, takes one row that says where.
      await expect(list.getByText('local · origin · fork', { exact: true })).toBeVisible()
      await expect(list.locator('.chip')).toHaveCount(3)
      await page.mouse.move(x + 600, y)
      await expect(list).toHaveCount(0)
    })

    // #625: one label then a count, so the rest need a way in that isn't the pointer.
    test("the row's label button lists every label for the keyboard, each with its own menu", async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      await page.locator('.scroller').focus()
      const actions = page.getByRole('group', { name: /^Actions for/ })
      const all = actions.getByRole('button', { name: 'All 3 branches and tags on 0000000' })
      await expect(all).toHaveAttribute('aria-expanded', 'false')
      // Tab goes from the graph to its row's actions, and the label button is the first of them.
      await page.keyboard.press('Tab')
      await expect(all).toBeFocused()
      await page.keyboard.press('Enter')
      const list = page.getByRole('group', { name: 'Branches and tags on this commit' })
      await expect(list.getByRole('button')).toHaveCount(3)
      // The focus moves into the list, on its first label.
      await expect(list.getByRole('button', { name: 'Menu for main' })).toBeFocused()
      await expect(all).toHaveAttribute('aria-expanded', 'true')
      await list.getByRole('button', { name: 'Menu for ◆ v1.0' }).focus()
      await page.keyboard.press('Enter')
      await expect.poll(async () => (await events(page, 'context-row')).at(-1)?.[0]).toBe(0)
      // Escape closes it and gives the focus back to the graph.
      await list.getByRole('button', { name: 'Menu for main' }).focus()
      await page.keyboard.press('Escape')
      await expect(list).toHaveCount(0)
      await expect(page.locator('.scroller')).toBeFocused()
    })

    test('a row with one label has no label button', async ({ page }) => {
      const one = { [id]: [branch('main', { head: true, branchKind: 'trunk' })] }
      await open(page, 'graph', { graphRows: 5, graphRefs: one, responses: { Refs: one[id], CommitSignatures: [] } })
      await ready(page)
      await page.locator('.scroller').focus()
      const actions = page.getByRole('group', { name: /^Actions for/ })
      await expect(actions).toBeVisible()
      await expect(actions.getByRole('button', { name: /branches and tags/ })).toHaveCount(0)
    })

    // #625: the column fits its labels, and its edge can be dragged.
    test('the label column fits its labels, and can be resized and set back to fitting', async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      const edge = page.getByRole('separator', { name: 'Resize the branch and tag column' })
      const width = async () => Number(await edge.getAttribute('aria-valuenow'))
      // "main", its kind icon and remote mark, and "+2": narrower than the old fixed 150.
      await expect.poll(width).toBeLessThan(150)
      const fitted = await width()
      expect(fitted).toBeGreaterThanOrEqual(120)
      await edge.focus()
      await page.keyboard.press('ArrowRight')
      await expect.poll(width).toBe(fitted + 16)
      expect(await page.evaluate(() => localStorage.getItem('gitrevenge.labelColumn:/repo'))).toBe(String(fitted + 16))
      const box = (await edge.boundingBox())!
      await page.mouse.move(box.x + 3, box.y + 200)
      await page.mouse.down()
      await page.mouse.move(box.x + 3 + 40, box.y + 200, { steps: 4 })
      await page.mouse.up()
      await expect.poll(width).toBe(fitted + 56)
      await edge.focus()
      await page.keyboard.press('Home')
      await expect.poll(width).toBe(fitted)
      expect(await page.evaluate(() => localStorage.getItem('gitrevenge.labelColumn:/repo'))).toBeNull()
    })

    test('the row without labels has nothing to expand', async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      const { x, y } = await overLabels(page)
      await page.mouse.move(x, y + 52)
      await page.waitForTimeout(100)
      await expect(page.locator('.ref-expand')).toHaveCount(0)
    })

    test('animates open and closed', async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      const { x, y } = await overLabels(page)
      // Counted as it starts: looking afterwards can land before it begins or after it ends.
      await page.evaluate(() => {
        const animate = Element.prototype.animate
        const win = window as unknown as { __opening: number }
        win.__opening = 0
        Element.prototype.animate = function (...args) {
          if (this.classList.contains('ref-expand')) win.__opening++
          return animate.apply(this, args)
        }
      })
      await page.mouse.move(x, y)
      const list = page.locator('.ref-expand')
      await expect(list).toBeVisible()
      await expect
        .poll(() => page.evaluate(() => (window as unknown as { __opening: number }).__opening))
        .toBeGreaterThan(0)
      await expect.poll(() => list.evaluate((el) => el.getAnimations().length)).toBe(0)
      await page.mouse.move(x + 600, y)
      // Still there for a moment on the way out.
      await expect(list).toHaveCount(1)
      await expect(list).toHaveCount(0)
    })

    test('reduced motion opens it without animating', async ({ page }) => {
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await open(page, 'graph', scenario)
      await ready(page)
      const { x, y } = await overLabels(page)
      await page.mouse.move(x, y)
      const list = page.locator('.ref-expand')
      await expect(list).toBeVisible()
      await page.waitForTimeout(50)
      expect(await list.evaluate((el) => getComputedStyle(el).opacity)).toBe('1')
    })

    test('stays out of the way of clicks on the row', async ({ page }) => {
      await open(page, 'graph', scenario)
      await ready(page)
      const { x, y } = await overLabels(page)
      await page.mouse.move(x, y)
      await expect(page.locator('.ref-expand')).toBeVisible()
      // Over the second row, under the open list.
      await page.mouse.click(x, y + 52)
      await expect.poll(async () => ((await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined)?.index).toBe(1)
    })

    test('stops a very long list, and says how many more', async ({ page }) => {
      const many = Array.from({ length: 20 }, (_, i) => branch(`b${i}`))
      await open(page, 'graph', { ...scenario, graphRefs: { [id]: many } })
      await ready(page)
      const { x, y } = await overLabels(page)
      await page.mouse.move(x, y)
      const list = page.locator('.ref-expand')
      await expect(list.getByText('and 8 more')).toBeVisible()
      await expect(list.getByText('b11', { exact: true })).toBeVisible()
      await expect(list.getByText('b12', { exact: true })).toHaveCount(0)
    })

    test('stays inside the view at the bottom edge', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 260 })
      const lastId = '4'.padStart(40, '0')
      await open(page, 'graph', { ...scenario, graphRefs: { [lastId]: graphRefs[id] } })
      await ready(page)
      const view = (await page.locator('.scroller').boundingBox())!
      await page.locator('.scroller').evaluate((el) => el.scrollTo(0, el.scrollHeight))
      await page.waitForTimeout(100)
      await page.mouse.move(view.x + 30, view.y + view.height - 26)
      const list = page.locator('.ref-expand')
      await expect(list).toBeVisible()
      await expect.poll(async () => {
        const b = (await list.boundingBox())!
        return b.y + b.height <= view.y + view.height + 1
      }).toBe(true)
    })
  })

  // #434: the right panel was a fixed 420px.
  test.describe('resizing the right panel', () => {
    const files = { path: 'a.txt', index: '.', workTree: 'M' }
    const scenario = { graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: { status: { files: [files] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } } } }
    const edge = (page: Page) => page.getByRole('separator', { name: 'Resize the right panel' })
    const width = (page: Page) => page.locator('.right').evaluate((e) => e.getBoundingClientRect().width)
    const ready = async (page: Page) => {
      await page.setViewportSize({ width: 1600, height: 800 })
      await open(page, 'repoView', scenario)
      await expect(edge(page)).toBeVisible()
    }

    test('dragging the edge resizes it, and double-clicking resets it', async ({ page }) => {
      await ready(page)
      expect(await width(page)).toBe(420)
      const box = (await edge(page).boundingBox())!
      const x = box.x + box.width / 2
      const y = box.y + 200
      await page.mouse.move(x, y)
      await page.mouse.down()
      await page.mouse.move(x - 100, y)
      await page.mouse.up()
      expect(await width(page)).toBe(520)
      await edge(page).dblclick()
      await expect.poll(() => width(page)).toBe(420)
    })

    test('the arrow keys resize it, and Home resets it', async ({ page }) => {
      await ready(page)
      await edge(page).focus()
      await page.keyboard.press('ArrowLeft')
      expect(await width(page)).toBe(436)
      await page.keyboard.press('ArrowRight')
      await page.keyboard.press('ArrowRight')
      expect(await width(page)).toBe(404)
      await page.keyboard.press('Home')
      expect(await width(page)).toBe(420)
    })

    test('stops at its limits', async ({ page }) => {
      await ready(page)
      const box = (await edge(page).boundingBox())!
      const y = box.y + 200
      await page.mouse.move(box.x + 3, y)
      await page.mouse.down()
      await page.mouse.move(1500, y)
      await page.mouse.up()
      expect(await width(page)).toBe(320)
      const narrow = (await edge(page).boundingBox())!
      await page.mouse.move(narrow.x + 3, y)
      await page.mouse.down()
      await page.mouse.move(0, y)
      await page.mouse.up()
      expect(await width(page)).toBe(960)
    })

    test('remembers the width after a reload', async ({ page }) => {
      await ready(page)
      await edge(page).focus()
      await page.keyboard.press('ArrowLeft')
      await page.reload()
      await expect(edge(page)).toBeVisible()
      await expect.poll(() => width(page)).toBe(436)
    })

    test('the working tree panel puts the commit form beside the files when wide', async ({ page }) => {
      await ready(page)
      await expect(page.locator('.composer')).toBeVisible()
      const list = page.locator('.lists')
      const beside = async () => (await page.locator('.composer').boundingBox())!.x >= (await list.boundingBox())!.x + (await list.boundingBox())!.width - 1
      expect(await beside()).toBe(false)
      const box = (await edge(page).boundingBox())!
      await page.mouse.move(box.x + 3, box.y + 200)
      await page.mouse.down()
      await page.mouse.move(box.x - 300, box.y + 200)
      await page.mouse.up()
      await expect.poll(beside).toBe(true)
    })
  })

  // #435: a long commit pushed its files far below the fold.
  test.describe('the commit panel', () => {
    const id = '0'.repeat(40)
    const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
    const commit = { id, subject: 'Commit 0', body: 'The long story.', trailers: [{ key: 'Closes', value: '#12' }], parents: ['a'.repeat(40)], author: person, committer: person }
    const files = [
      { status: 'M', path: 'src/lib/a.ts', added: 12, deleted: 3 },
      { status: 'A', path: 'src/lib/b.ts', added: 5, deleted: 0 },
      { status: 'M', path: 'docs/c.md', added: 1, deleted: 1 },
      { status: 'M', path: 'logo.png', added: 0, deleted: 0, binary: true },
    ]
    const scenario = (over: object = {}) => ({ graphRows: 3, responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@example.com' } }, CommitDetail: { commit: { ...commit, ...over }, files }, CommitRow: 1 } })
    const openCommit = async (page: Page, over: object = {}) => {
      await open(page, 'repoView', scenario(over))
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 26)
      await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    }
    const tab = (page: Page, name: string) => page.getByRole('tab', { name, exact: true })

    // #681: a note is rendered as markdown, as the message is.
    test('a note is rendered as markdown, and as written when the message is', async ({ page }) => {
      const notes = [{ ref: 'refs/notes/commits', text: '**Deployed** to `staging`\n\n- smoke tests pass\n- waiting on QA' }]
      const s = scenario()
      await open(page, 'repoView', { ...s, responses: { ...s.responses, CommitNotes: notes } })
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 26)
      await tab(page, 'Details').click()
      const note = page.locator('.notes .note')
      await expect(note.locator('strong')).toHaveText('Deployed')
      await expect(note.locator('code')).toHaveText('staging')
      await expect(note.getByRole('listitem')).toHaveText(['smoke tests pass', 'waiting on QA'])
      await expect(note).not.toContainText('**')
      // The message's Raw switch shows the note as it was written, too.
      await tab(page, 'Message').click()
      await page.getByRole('button', { name: 'Markdown' }).click()
      await tab(page, 'Details').click()
      await expect(note).toContainText('**Deployed** to `staging`')
      await expect(note.locator('strong')).toHaveCount(0)
    })

    test('a note links web addresses and smart links, and keeps its line breaks', async ({ page }) => {
      const rule = { name: 'Jira', pattern: 'ABC-\\d+', url: 'https://jira.example.com/browse/{0}', in: ['message', 'trailers', 'branches'], enabled: true }
      const notes = [{ ref: 'refs/notes/commits', text: 'Deployed for ABC-1234.\nRunbook: https://example.com/runbook' }]
      const s = scenario()
      await open(page, 'repoView', { ...s, responses: { ...s.responses, CommitNotes: notes, LinkRules: { web: true, rules: [rule] } } })
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 26)
      await tab(page, 'Details').click()
      const note = page.locator('.notes .note')
      await expect(note).toContainText('Deployed for ABC-1234.')
      const key = note.getByRole('link', { name: 'ABC-1234' })
      await expect(key).toHaveAttribute('href', 'https://jira.example.com/browse/ABC-1234')
      await expect(key).toHaveClass(/smart/)
      await expect(note.getByRole('link', { name: 'https://example.com/runbook' })).toHaveAttribute('href', 'https://example.com/runbook')
      // The words around a link are still there, and so is the note's second line.
      await expect(note.locator('p')).toHaveText('Deployed for ABC-1234.\nRunbook: https://example.com/runbook', { useInnerText: true })
      // A click opens it in the browser, not in the app's own window.
      await key.click()
      expect(await events(page, 'open-url')).toContainEqual(['https://jira.example.com/browse/ABC-1234'])
      // And it has the message's link menu.
      await key.click({ button: 'right' })
      await expect(page.getByRole('menuitem', { name: 'Open ABC-1234 in Jira' })).toBeVisible()
      await expect(page.getByRole('menuitem', { name: 'Find commits mentioning ABC-1234' })).toBeVisible()
    })

    test('a note with nothing to link reads as it did', async ({ page }) => {
      const s = scenario()
      await open(page, 'repoView', { ...s, responses: { ...s.responses, CommitNotes: [{ ref: 'refs/notes/commits', text: 'just a plain note' }] } })
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 26)
      await tab(page, 'Details').click()
      await expect(page.locator('.notes .note')).toContainText('just a plain note')
      await expect(page.locator('.notes .note a')).toHaveCount(0)
    })

    test('opens on the message, under plain headings', async ({ page }) => {
      await openCommit(page)
      await expect(tab(page, 'Message')).toHaveAttribute('aria-selected', 'true')
      await expect(page.getByRole('tabpanel')).toContainText('The long story.')
      await expect(page.getByRole('tab')).toHaveText(['Message', 'Files', 'Trailers', 'Details'])
    })

    test('lists files by name under their folders, with line counts', async ({ page }) => {
      await openCommit(page)
      await tab(page, 'Files').click()
      const panel = page.getByRole('tabpanel')
      await expect(panel.getByText('src/lib/', { exact: true })).toBeVisible()
      await expect(panel.getByRole('button', { name: /a\.ts/ })).toContainText('+12')
      await expect(panel.getByRole('button', { name: /a\.ts/ })).toContainText('−3')
      await expect(panel.getByRole('button', { name: /logo\.png/ })).toContainText('binary')
    })

    test('remembers the tab across commits and sessions', async ({ page }) => {
      await openCommit(page)
      await tab(page, 'Details').click()
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 78)
      await expect(tab(page, 'Details')).toHaveAttribute('aria-selected', 'true')
      await page.reload()
      const again = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(again.x + 500, again.y + ROW + 26)
      await expect(tab(page, 'Details')).toHaveAttribute('aria-selected', 'true')
    })

    test('the arrow keys move along the tabs', async ({ page }) => {
      await openCommit(page)
      await tab(page, 'Message').focus()
      await page.keyboard.press('ArrowRight')
      await expect(tab(page, 'Files')).toBeFocused()
      await expect(tab(page, 'Files')).toHaveAttribute('aria-selected', 'true')
      await page.keyboard.press('ArrowLeft')
      await page.keyboard.press('ArrowLeft')
      await expect(tab(page, 'Details')).toBeFocused()
    })

    test('a commit without trailers shows the message, and keeps the choice for one that has them', async ({ page }) => {
      await page.addInitScript(() => localStorage.setItem('gitrevenge.commitTab', 'trailers'))
      await openCommit(page, { trailers: [] })
      await expect(tab(page, 'Trailers')).toHaveCount(0)
      await expect(tab(page, 'Message')).toHaveAttribute('aria-selected', 'true')
      expect(await page.evaluate(() => localStorage.getItem('gitrevenge.commitTab'))).toBe('trailers')
    })

    // #436: trailers read in the order they were added, with what mattered lost among them.
    test('groups trailers by purpose, with Cc collapsed', async ({ page }) => {
      const line = (key: string, value: string) => ({ key, value })
      await openCommit(page, {
        trailers: [
          line('Signed-off-by', 'Ana Cruz <ana@example.com>'),
          line('Signed-off-by', 'Bo Lee <bo@example.com>'),
          line('Closes', '#12'),
          line('Cc', 'One <one@example.com>'),
          line('Cc', 'Two <two@example.com>'),
          line('Cc', 'Three <three@example.com>'),
          line('Cc', 'Four <four@example.com>'),
        ],
      })
      await tab(page, 'Trailers').click()
      const panel = page.getByRole('tabpanel')
      await expect(panel.getByRole('heading')).toHaveText(['Related', 'People'])
      await expect(panel.getByText('Three')).toHaveCount(0)
      await panel.getByRole('button', { name: '+ 2 more' }).click()
      await expect(panel.getByText('Three')).toBeVisible()
      await expect(panel.getByRole('button', { name: 'Show fewer' })).toBeVisible()
      // The sign-offs stay in the order they were made.
      const text = (await panel.textContent()) ?? ''
      expect(text.indexOf('Ana Cruz')).toBeLessThan(text.indexOf('Bo Lee'))
    })

    test('Raw shows trailers as written', async ({ page }) => {
      const line = (key: string, value: string) => ({ key, value })
      await openCommit(page, { trailers: [line('Signed-off-by', 'Ana Cruz <ana@example.com>'), line('Closes', '#12')] })
      await tab(page, 'Trailers').click()
      const panel = page.getByRole('tabpanel')
      await panel.getByRole('button', { name: 'Grouped' }).click()
      await expect(panel.getByRole('heading')).toHaveCount(0)
      const text = (await panel.textContent()) ?? ''
      expect(text.indexOf('Signed-off-by')).toBeLessThan(text.indexOf('Closes'))
    })

    // #440: long lists of files were hard to scan.
    test.describe('with many files', () => {
      const many = [...files, { status: 'M', path: 'src/lib/d.ts', added: 2, deleted: 2 }, { status: 'M', path: 'src/e.ts', added: 1, deleted: 0 }]
      const openMany = async (page: Page) => {
        await open(page, 'repoView', { ...scenario(), responses: { ...scenario().responses, CommitDetail: { commit, files: many } } })
        const box = (await page.locator('.scroller').boundingBox())!
        await page.mouse.click(box.x + 500, box.y + ROW + 26)
        await tab(page, 'Files').click()
      }

      test('filters them, and says when nothing matches', async ({ page }) => {
        await openMany(page)
        const panel = page.getByRole('tabpanel')
        await panel.getByRole('searchbox', { name: 'Filter files' }).fill('docs')
        await expect(panel.locator('.file-row')).toHaveCount(1)
        await panel.getByRole('searchbox', { name: 'Filter files' }).fill('no such file')
        await expect(panel.getByText('No files match that.')).toBeVisible()
      })

      test('shows them as a tree, with folders that close, and remembers', async ({ page }) => {
        await openMany(page)
        const panel = page.getByRole('tabpanel')
        await panel.getByRole('button', { name: 'Tree' }).click()
        await expect(panel.getByRole('button', { name: 'lib/', exact: true })).toBeVisible()
        await expect(panel.locator('.file-row')).toHaveCount(6)
        await panel.getByRole('button', { name: 'lib/', exact: true }).click()
        await expect(panel.locator('.file-row')).toHaveCount(3)
        expect(await page.evaluate(() => localStorage.getItem('gitrevenge.fileTree'))).toBe('on')
      })
    })

    // #437
    test('the arrow keys move between its files', async ({ page }) => {
      await openCommit(page)
      await tab(page, 'Files').click()
      const rows = page.getByRole('tabpanel').locator('.file-row')
      await rows.first().focus()
      await page.keyboard.press('ArrowDown')
      await expect(rows.nth(1)).toBeFocused()
    })

    test('has no filter for a short list', async ({ page }) => {
      await openCommit(page)
      await tab(page, 'Files').click()
      await expect(page.getByRole('searchbox', { name: 'Filter files' })).toHaveCount(0)
    })

    test('says so when there is no description', async ({ page }) => {
      await openCommit(page, { body: '', trailers: [] })
      await expect(tab(page, 'Trailers')).toHaveCount(0)
      await expect(page.getByRole('tabpanel')).toContainText('No description.')
    })

    // #582: a link in the subject was the browser's blue, beside rose ones in the body.
    test('a link in the subject is the same rose as one in the body', async ({ page }) => {
      await open(page, 'repoView', scenario({ subject: 'See https://example.com/a', body: 'And https://example.com/b too.' }))
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 500, box.y + ROW + 26)
      const inSubject = page.getByRole('heading', { name: /^See/ }).getByRole('link')
      await expect(inSubject).toBeVisible()
      const color = (l: typeof inSubject) => l.evaluate((e) => getComputedStyle(e).color)
      const rose = await page.evaluate(() => {
        const probe = document.createElement('span')
        probe.style.color = 'var(--link)'
        document.body.append(probe)
        const c = getComputedStyle(probe).color
        probe.remove()
        return c
      })
      expect(await color(inSubject)).toBe(rose)
      expect(await color(page.getByRole('tabpanel').getByRole('link').first())).toBe(rose)
    })

    test('copies the SHA, and shows the parent', async ({ page }) => {
      await openCommit(page)
      await page.getByRole('button', { name: 'Copy SHA 0000000' }).click()
      await expect(page.getByRole('button', { name: 'Copy SHA 0000000' })).toContainText('Copied')
      await page.getByRole('button', { name: 'parent aaaaaaa' }).click()
      await expect.poll(() => calls(page, 'CommitRow').then((c) => c.at(-1))).toEqual(['/r', 'a'.repeat(40)])
    })

    test('Actions opens the commit menu', async ({ page }) => {
      await openCommit(page)
      await page.getByRole('button', { name: 'Actions' }).click()
      await expect(page.getByRole('menu')).toBeVisible()
    })

    test('when wide, the files stay beside the tabs', async ({ page }) => {
      await page.setViewportSize({ width: 1600, height: 800 })
      await openCommit(page)
      await page.getByRole('separator', { name: 'Resize the right panel' }).focus()
      for (let i = 0; i < 16; i++) await page.keyboard.press('ArrowLeft')
      await expect(tab(page, 'Files')).toHaveCount(0)
      await expect(page.getByRole('tabpanel')).toContainText('The long story.')
      await expect(page.locator('.side-files').getByRole('button', { name: /a\.ts/ })).toBeVisible()
    })
  })

  // #441: the graph couldn't take the panel's width.
  test.describe('hiding the right panel', () => {
    const scenario = { graphRows: 3, responses: { Refs: [], CommitSignatures: [], CommitNotes: [], WorkingTree: { status: { files: [] }, signing: { enabled: false }, identity: { name: 'A', email: 'a@b' } }, CommitDetail: { commit: { id: '0'.repeat(40), subject: 'Commit 0', body: '', trailers: [], parents: [], author: { name: 'A', email: 'a@b', time: 1 }, committer: { name: 'A', email: 'a@b', time: 1 } }, files: [] } } }
    const hide = (page: Page) => page.getByRole('button', { name: 'Hide the right panel' })
    const show = (page: Page) => page.getByRole('button', { name: 'Show the right panel' })
    const selectRow = async (page: Page, y: number) => {
      const box = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(box.x + 300, box.y + ROW + y)
    }

    test('the button hides and shows it, and the graph takes the room', async ({ page }) => {
      await open(page, 'repoView', scenario)
      await selectRow(page, 26)
      await expect(page.locator('.right')).toBeVisible()
      const before = (await page.locator('.scroller').boundingBox())!.width
      await hide(page).click()
      await expect(page.locator('.right')).toHaveCount(0)
      await expect.poll(async () => (await page.locator('.scroller').boundingBox())!.width).toBeGreaterThan(before)
      await show(page).click()
      await expect(page.locator('.right')).toBeVisible()
    })

    test('selecting a commit does not reopen it', async ({ page }) => {
      await open(page, 'repoView', scenario)
      await selectRow(page, 26)
      await hide(page).click()
      await selectRow(page, 78)
      await page.waitForTimeout(100)
      await expect(page.locator('.right')).toHaveCount(0)
    })

    test('the row of changes reopens it', async ({ page }) => {
      await open(page, 'repoView', scenario)
      await selectRow(page, 26)
      await hide(page).click()
      await page.locator('button.wip').click()
      await expect(page.locator('.right')).toBeVisible()
    })

    test('remembers across a reload', async ({ page }) => {
      await open(page, 'repoView', scenario)
      await selectRow(page, 26)
      await hide(page).click()
      await page.reload()
      await expect(show(page)).toBeVisible()
      await expect(page.locator('.right')).toHaveCount(0)
    })
  })

  // #418: the commit panel has the same list, for the keyboard and screen readers.
  test("the commit panel lists the selected commit's refs", async ({ page }) => {
    const id = '0'.repeat(40)
    const person = { name: 'Ana Cruz', email: 'ana@example.com', time: 1793500000 }
    const commit = { id, subject: 'Commit 0', body: '', trailers: [], parents: [], author: person, committer: person }
    await open(page, 'repoView', {
      graphRows: 3,
      graphRefs: { [id]: [branch('main', { head: true, branchKind: 'trunk' }), remote('origin/main', { branchKind: 'trunk' }), tag('v1.0')] },
      responses: { Refs: [remote('origin/main')], CommitSignatures: [], WorkingTree: { status: { files: [] } }, CommitDetail: { commit, files: [] } },
    })
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 500, box.y + ROW + 26)
    const list = page.getByLabel('Branches and tags on this commit')
    await expect(list).toBeVisible()
    await expect(list.getByText('main', { exact: true })).toBeVisible()
    await expect(list.getByText('◆ v1.0')).toBeVisible()
  })

  // #340: a selection before the first measure scrolled the graph one row down.
  test('opens at the top', async ({ page }) => {
    await open(page, 'graph', { graphRows: 500, responses: { Refs: [], CommitSignatures: [] } })
    const scroller = page.locator('.scroller')
    await expect(scroller).toHaveAttribute('aria-rowcount', '500')
    await page.waitForTimeout(200)
    expect(await scroller.evaluate((e) => e.scrollTop)).toBe(0)
  })

  // #386: finishing the load put the selection back on the row it had before refs arrived.
  test('selects HEAD when refs arrive just before the load finishes', async ({ page }) => {
    await open(page, 'graph', { graphRows: 30, delays: { CommitRow: 200 }, responses: { Refs: [], CommitSignatures: [], CommitRow: 0 } })
    const selected = async () => {
      await page.mouse.move(0, 0)
      return (await events(page, 'select')).at(-1)?.[0] as { index: number } | undefined
    }
    await expect.poll(async () => (await selected())?.index).toBe(0)

    const head = String(5).padStart(40, '0')
    await page.evaluate((id) => {
      const w = window as unknown as { __scenario: { graphRefs: unknown }; __emit: (name: string, data: unknown) => void }
      w.__scenario.graphRefs = { [id]: [{ name: 'refs/heads/main', short: 'main', kind: 'local', commit: id, head: true }] }
      w.__emit('graph:progress', { path: '/repo', rows: 30, refsReady: true })
      w.__emit('graph:progress', { path: '/repo', rows: 30, done: true, estimate: 30 })
    }, head)
    await expect.poll(async () => (await selected())?.index).toBe(5)
    await page.waitForTimeout(500)
    expect((await selected())?.index).toBe(5)
  })
})

test.describe('start page', () => {
  const GROUPS = [
    { name: 'Work', paths: ['/r/api', '/r/web', '/r/docs', '/r/ops'] },
    { name: 'Side', paths: ['/r/game', '/r/notes'] },
    { name: 'Tools', paths: ['/r/cli'] },
  ]
  const scenario = () => ({ responses: { Groups: structuredClone(GROUPS) } })
  const card = (page: Page, name: string) => page.locator('[data-card]', { has: page.getByRole('heading', { name, exact: true }) })
  const names = (page: Page) => page.locator('[data-card] h3').allTextContents()
  const repos = (page: Page, name: string) => card(page, name).locator('.repo').allTextContents()

  async function drag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }) {
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    await page.mouse.move(to.x, to.y)
    await page.mouse.up()
  }
  const middle = (b: { x: number; y: number; width: number; height: number }) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 })

  test('dragging a card moves the group', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').hover()
    const from = middle((await card(page, 'Work').locator('.card-grip').boundingBox())!)
    const last = (await card(page, 'Tools').boundingBox())!
    await drag(page, from, { x: last.x + last.width - 4, y: last.y + 10 })
    await expect.poll(() => names(page)).toEqual(['Side', 'Tools', 'Work'])
    expect(await calls(page, 'GroupAction')).toContainEqual(['move', ['Work', '2']])
  })

  // #349: unpositioned rows stacked every grip and marker in the middle of the card.
  test('each repository has its grip beside its own row', async ({ page }) => {
    await open(page, 'start', scenario())
    const rows = card(page, 'Work').locator('[data-repo]')
    await expect(rows).toHaveCount(4)
    const gaps = await rows.evaluateAll((items) =>
      items.map((li) => {
        const grip = li.querySelector('.repo-grip')!.getBoundingClientRect()
        const row = li.getBoundingClientRect()
        return Math.abs(grip.top + grip.height / 2 - (row.top + row.height / 2))
      }),
    )
    for (const gap of gaps) expect(gap).toBeLessThanOrEqual(1)
  })

  test('dragging a repository moves it, and does not open it', async ({ page }) => {
    await open(page, 'start', scenario())
    const first = card(page, 'Work').locator('[data-repo]').first()
    await first.hover()
    const from = middle((await first.locator('.repo-grip').boundingBox())!)
    const target = (await card(page, 'Work').locator('[data-repo]').nth(2).boundingBox())!
    await drag(page, from, { x: from.x, y: target.y + target.height - 2 })
    await expect.poll(() => repos(page, 'Work')).toEqual(['web', 'docs', 'api', 'ops'])
    expect(await calls(page, 'OpenRepo')).toEqual([])
  })

  // #358: positions read at the start of a drag went stale when the page scrolled.
  test('scrolling during a drag keeps the drop in the right place', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 520 })
    await open(page, 'start', scenario())
    const first = card(page, 'Work').locator('[data-repo]').first()
    await first.scrollIntoViewIfNeeded()
    await first.hover()
    const from = middle((await first.locator('.repo-grip').boundingBox())!)
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    const scrollTop = () =>
      page.evaluate(() => {
        const scroller = [...document.querySelectorAll('*')].find((e) => e.scrollHeight > e.clientHeight + 40 && getComputedStyle(e).overflowY !== 'visible')
        return (scroller ?? document.scrollingElement)!.scrollTop
      })
    const before = await scrollTop()
    await page.mouse.wheel(0, 60)
    // A wheel event scrolls asynchronously, so wait for it to land and settle.
    await expect.poll(scrollTop).toBeGreaterThan(before)
    let last = -1
    await expect
      .poll(async () => {
        const now = await scrollTop()
        const settled = now === last
        last = now
        return settled
      })
      .toBe(true)
    // Where the third row is now, not where it was when the drag began.
    const target = (await card(page, 'Work').locator('[data-repo]').nth(2).boundingBox())!
    await page.mouse.move(from.x, target.y + target.height - 2)
    await page.mouse.up()
    await expect.poll(() => repos(page, 'Work')).toEqual(['web', 'docs', 'api', 'ops'])
  })

  test('dropping a repository on another group changes nothing', async ({ page }) => {
    await open(page, 'start', scenario())
    const first = card(page, 'Work').locator('[data-repo]').first()
    await first.hover()
    const from = middle((await first.locator('.repo-grip').boundingBox())!)
    await drag(page, from, middle((await card(page, 'Side').boundingBox())!))
    await page.waitForTimeout(150)
    expect(await repos(page, 'Work')).toEqual(['api', 'web', 'docs', 'ops'])
    expect((await calls(page, 'GroupAction')).length).toBe(0)
  })

  test('Alt+arrows move a repository and keep focus', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').getByRole('button', { name: 'docs', exact: true }).focus()
    await page.keyboard.press('Alt+ArrowUp')
    await expect.poll(() => repos(page, 'Work')).toEqual(['api', 'docs', 'web', 'ops'])
    await expect(card(page, 'Work').getByRole('button', { name: 'docs', exact: true })).toBeFocused()
    await expect(page.locator('[aria-live]')).toContainText('Moved docs to position 2 of 4')
  })

  test('the menu moves a group, and is disabled at the ends', async ({ page }) => {
    await open(page, 'start', scenario())
    await card(page, 'Work').getByRole('button', { name: 'More actions for Work' }).click()
    await expect(page.getByRole('menuitem', { name: 'Move earlier' })).toHaveAttribute('aria-disabled', 'true')
    await page.getByRole('menuitem', { name: 'Move later' }).click()
    await expect.poll(() => names(page)).toEqual(['Side', 'Work', 'Tools'])
  })
})

test.describe('upstream dialog', () => {
  const refs = [branch('main', { upstream: 'origin/main' }), remote('origin/main'), remote('origin/x'), tag('v1')]
  const field = (page: Page) => page.getByLabel('Track')
  const save = (page: Page) => page.getByRole('button', { name: 'Save' })

  test('opens on the current upstream, suggesting remote branches', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await expect(field(page)).toHaveValue('origin/main')
    const options = await page.locator('datalist option').evaluateAll((o) => o.map((x) => (x as HTMLOptionElement).value))
    expect(options).toEqual(['origin/main', 'origin/x'])
  })

  test('sends a typed name, trimmed', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await field(page).fill('  origin/typed  ')
    await save(page).click()
    await expect.poll(() => calls(page, 'BranchAction')).toEqual([['/r', 'upstream', ['main', 'origin/typed']]])
    expect(await events(page, 'close')).toHaveLength(1)
  })

  test('emptying it stops tracking', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs } })
    await field(page).fill('')
    await save(page).click()
    await expect.poll(() => calls(page, 'BranchAction')).toEqual([['/r', 'upstream', ['main', '']]])
  })

  test('keeps the dialog open with git’s error', async ({ page }) => {
    await open(page, 'upstreamDialog', { responses: { Refs: refs }, failures: { BranchAction: 'the requested upstream branch does not exist' } })
    await save(page).click()
    await expect(page.getByRole('alert')).toContainText('does not exist')
    expect(await events(page, 'close')).toHaveLength(0)
  })

  // #373: the button that was clicked is disabled while it runs, which drops focus to the body.
  test.describe('after a button has dropped focus', () => {
    async function failedSave(page: Page) {
      await open(page, 'upstreamDialog', { responses: { Refs: refs }, failures: { BranchAction: 'nope' }, delays: { BranchAction: 150 } })
      await save(page).click()
      await expect(page.getByRole('alert')).toBeVisible()
      // Chromium does this by itself. WebKit doesn't focus a clicked button, so focus never left
      // the dialog, and it needs a push to get here.
      await page.evaluate(() => (document.activeElement as HTMLElement).blur())
      await expect.poll(() => page.evaluate(() => document.activeElement === document.body)).toBe(true)
    }

    test('Escape still closes the dialog', async ({ page }) => {
      await failedSave(page)
      await page.keyboard.press('Escape')
      expect(await events(page, 'close')).toHaveLength(1)
    })

    test('Tab and Shift+Tab stay inside the dialog', async ({ page }) => {
      await failedSave(page)
      const inside = () => page.evaluate(() => !!document.activeElement?.closest('[role=dialog]'))
      await page.keyboard.press('Tab')
      expect(await inside()).toBe(true)
      await page.evaluate(() => (document.activeElement as HTMLElement).blur())
      await page.keyboard.press('Shift+Tab')
      expect(await inside()).toBe(true)
    })

    test('other keys are left alone', async ({ page }) => {
      await failedSave(page)
      await page.keyboard.press('a')
      expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true)
    })
  })

  test('with focus lost, Escape closes only the top dialog', async ({ page }) => {
    await open(page, 'stackedDialogs')
    await page.evaluate(() => (document.activeElement as HTMLElement).blur())
    await page.keyboard.press('Escape')
    expect(await events(page, 'close-above')).toHaveLength(1)
    expect(await events(page, 'close-below')).toHaveLength(0)
  })
})

// #374: an empty starting value read as a fact until the first answer came back.
// #390: the date column was cut off at the default size.
test('blame shows the whole date, whatever the length of the author name', async ({ page }) => {
  const line = (author: string) => ({ commit: 'a'.repeat(40), author, time: 1784214240, summary: 'S', line: 'text', origLine: 1 })
  await open(page, 'blame', { responses: { Blame: [line('Someone With A Really Very Long Name Indeed')] } })
  const when = page.locator('.when')
  await expect(when).toBeVisible()
  const cell = (await page.locator('.gutter').boundingBox())!
  const date = (await when.boundingBox())!
  expect(date.x + date.width).toBeLessThanOrEqual(cell.x + cell.width)
  expect(await when.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true)
  await expect(page.locator('.who')).toHaveAttribute('title', 'Someone With A Really Very Long Name Indeed')
})

test.describe('before the first load has answered', () => {
  const repoView = (files: unknown[]) => ({ graphRows: 0, delays: { WorkingTree: 500 }, responses: { Refs: [], CommitSignatures: [], WorkingTree: { status: { files } } } })
  const bar = (page: Page) => page.locator('button.wip')

  test('the working tree bar claims nothing, then says it is clean', async ({ page }) => {
    await open(page, 'repoView', repoView([]))
    const titleLeft = async () => (await bar(page).locator('.wip-title').boundingBox())!.x
    await expect(bar(page)).toContainText('Working tree')
    await expect(bar(page)).not.toContainText(/clean/i)
    const before = await titleLeft()
    await expect(bar(page)).toContainText(/clean/i)
    expect(await titleLeft()).toBe(before)
  })

  test('a conflicted tree never shows as clean', async ({ page }) => {
    await open(page, 'repoView', repoView([{ path: 'a.txt', index: 'U', workTree: 'U', conflicted: true }]))
    // Every state it passes through, since a retrying assertion would wait out a wrong one.
    const seen = await bar(page).evaluate(
      (el) =>
        new Promise<string[]>((resolve) => {
          const texts = [el.textContent ?? '']
          const observer = new MutationObserver(() => {
            texts.push(el.textContent ?? '')
            if (/1 file/i.test(el.textContent ?? '')) {
              observer.disconnect()
              resolve(texts)
            }
          })
          observer.observe(el, { subtree: true, childList: true, characterData: true })
        }),
    )
    expect(seen.join(' / ')).not.toMatch(/clean/i)
  })

  // #389: the commit panel said 0 files, and ignored Ctrl+Enter, until status had loaded.
  test.describe('commit panel', () => {
    const file = (path: string, index: string, workTree: string) => ({ path, index, workTree, untracked: false, conflicted: false })
    const panel = (files: unknown[]) => ({
      delays: { WorkingTree: 1500 },
      responses: { CommitChanges: 'abc', WorkingTree: { status: { files }, identity: { name: 'A', email: 'a@b' }, signing: { enabled: false } } },
    })
    const summary = (page: Page) => page.getByRole('textbox', { name: 'Commit summary' })

    test('claims no counts until status has loaded', async ({ page }) => {
      await open(page, 'workingTree', panel([file('a.txt', 'M', '.'), file('b.txt', '.', 'M')]))
      await expect(page.getByText('Files', { exact: true })).toBeVisible()
      await expect(page.getByRole('button', { name: 'Commit', exact: true })).toBeVisible()
      await expect(page.getByText('1 of 2 ticked')).toBeVisible()
      await expect(page.getByRole('button', { name: 'Tick a file to commit it' }).or(page.getByRole('button', { name: 'Describe it to commit 1 file' }))).toBeVisible()
    })

    test('Ctrl+Enter before status has loaded commits once it has', async ({ page }) => {
      await open(page, 'workingTree', panel([file('a.txt', 'M', '.')]))
      await summary(page).fill('Early')
      await page.keyboard.press('Control+Enter')
      await expect(page.getByRole('button', { name: 'Reading status…' })).toBeVisible()
      expect(await calls(page, 'CommitChanges')).toEqual([])
      await expect.poll(() => calls(page, 'CommitChanges')).toEqual([['/r', 'Early', false]])
      await expect(summary(page)).toHaveValue('')
    })

    test('Ctrl+Enter before status has loaded commits nothing if nothing is staged', async ({ page }) => {
      await open(page, 'workingTree', panel([file('b.txt', '.', 'M')]))
      await summary(page).fill('Early')
      await page.keyboard.press('Control+Enter')
      await expect(page.getByRole('button', { name: 'Tick a file to commit it' })).toBeVisible()
      expect(await calls(page, 'CommitChanges')).toEqual([])
      await expect(summary(page)).toHaveValue('Early')
    })
  })

  // #427: the rose banner competed with the selected row, and didn't say what to do.
  test.describe('the row of changes above the graph', () => {
    const dirty = (files: unknown[]) => ({ graphRows: 3, responses: { Refs: [], CommitSignatures: [], WorkingTree: { status: { files }, identity: { name: 'A', email: 'a@b' }, signing: { enabled: false } } } })
    const one = { path: 'a.txt', index: '.', workTree: 'M', untracked: false, conflicted: false }

    test('says how many files changed, and offers a review', async ({ page }) => {
      await open(page, 'repoView', dirty([one, { ...one, path: 'b.txt' }]))
      await expect(bar(page)).toContainText('2 files changed')
      await expect(bar(page)).toContainText('Review')
      await expect(bar(page).locator('.ring')).toBeVisible()
    })

    test('reads as clean when it is', async ({ page }) => {
      await open(page, 'repoView', dirty([]))
      await expect(bar(page)).toContainText(/clean/i)
      await expect(bar(page)).not.toContainText('Review')
    })

    test('stays quiet while open, rather than taking the selection colour', async ({ page }) => {
      await open(page, 'repoView', dirty([one]))
      await expect(bar(page)).toHaveAttribute('aria-pressed', 'true')
      const color = await bar(page).evaluate((e) => getComputedStyle(e).backgroundColor)
      const selected = await bar(page).evaluate(() => {
        const probe = document.createElement('div')
        probe.style.background = 'var(--selected)'
        document.body.append(probe)
        const c = getComputedStyle(probe).backgroundColor
        probe.remove()
        return c
      })
      expect(color).not.toBe(selected)
    })

    // #559: the graph's tools share the toolbar's bar, and step aside when a file takes over.
    test("the graph's tools sit in the toolbar's bar, and leave it with the graph", async ({ page }) => {
      await page.setViewportSize({ width: 1800, height: 800 })
      await open(page, 'repoView', dirty([one]))
      const fetch = page.getByRole('button', { name: 'Fetch' })
      const search = page.getByRole('searchbox', { name: 'Search commits' })
      await expect(search).toBeVisible()
      const a = (await fetch.boundingBox())!
      const b = (await search.boundingBox())!
      // One row: the two overlap vertically.
      expect(b.y).toBeLessThan(a.y + a.height)
      expect(a.y).toBeLessThan(b.y + b.height)
      // Nothing of the header is left above the graph.
      expect(await page.locator('.graph .status').count()).toBe(0)

      await bar(page).click()
      await page.getByRole('button', { name: /a\.txt/ }).first().click()
      await expect(page.locator('.scroller')).toBeHidden()
      await expect(search).toBeHidden()
      await expect(fetch).toBeVisible()
    })

    // #621: the row above the first commit is a row like any other to the layout.
    test('the working tree row scrolls away with the history and comes back with the first commit', async ({ page }) => {
      await page.setViewportSize({ width: 1600, height: 500 })
      await open(page, 'repoView', { ...dirty([one]), graphRows: 300 })
      await expect(bar(page)).toContainText('1 file changed')
      const scroller = page.locator('.scroller')
      const top = (await scroller.boundingBox())!.y
      expect((await bar(page).boundingBox())!.y).toBe(top)
      // Half a row down: the row is half out of the top edge, still there.
      const box = (await scroller.boundingBox())!
      await page.mouse.move(box.x + 300, box.y + 200)
      await page.mouse.wheel(0, 26)
      await expect.poll(async () => Math.round((await bar(page).boundingBox())!.y - top)).toBe(-26)
      // Further, it's gone; End and Home bring it back with the first commit.
      await page.mouse.wheel(0, 2000)
      await expect(bar(page)).toHaveCount(0)
      await scroller.focus()
      await page.keyboard.press('End')
      await expect.poll(() => scroller.evaluate((e) => Math.abs(e.scrollTop - (e.scrollHeight - e.clientHeight)) <= 1)).toBe(true)
      // 300 commits and the row above them, at 52 each.
      expect(await scroller.evaluate((e) => e.scrollHeight)).toBe(301 * 52)
      await page.keyboard.press('Home')
      await expect.poll(async () => (await bar(page).boundingBox())?.y).toBe(top)
    })

    // #626: the bar stays on one line, giving way in a fixed order as it narrows.
    test.describe('the bar on one line', () => {
      const kinds = [branch('main', { head: true, branchKind: 'trunk' }), branch('gh-pages', { branchKind: 'docs' })]
      const scenario = { ...dirty([one]), graphRows: 411, responses: { ...dirty([one]).responses, Refs: kinds } }
      const theBar = (page: Page) => page.locator('.main > .bar')
      /** Whether a toolbar button still shows its word; its name is the same either way. */
      const worded = async (page: Page, name: string) => ((await page.getByRole('toolbar', { name: 'Repository actions' }).getByRole('button', { name, exact: true }).textContent()) ?? '').trim() === name
      const oneLine = async (page: Page) => {
        const box = (await theBar(page).boundingBox())!
        expect(box.height).toBeLessThanOrEqual(45)
        // Nothing is pushed out of the bar or out of the graph's share of it.
        expect(await theBar(page).evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)
        expect(await page.locator('.graph-tools .status').evaluate((e) => e.scrollWidth - e.clientWidth)).toBe(0)
      }
      const at = async (page: Page, barWidth: number) => {
        await page.setViewportSize({ width: 1900, height: 700 })
        await open(page, 'repoView', scenario)
        await expect(page.getByRole('button', { name: 'Kinds' })).toBeVisible()
        // The sidebar and the right panel take a fixed share, so size the window for the bar.
        const spare = 1900 - (await theBar(page).evaluate((e) => e.clientWidth))
        await page.setViewportSize({ width: barWidth + spare, height: 700 })
        await expect.poll(() => theBar(page).evaluate((e) => e.clientWidth)).toBe(barWidth)
      }

      test('with room, everything is spelled out', async ({ page }) => {
        await at(page, 1160)
        await expect(page.getByText('411 commits')).toBeVisible()
        await expect(page.getByRole('searchbox', { name: 'Search commits' })).toHaveAttribute('placeholder', 'Search message, author or SHA')
        for (const name of ['Undo', 'Redo', 'Fetch', 'Pull', 'Push']) expect(await worded(page, name)).toBe(true)
        await oneLine(page)
      })

      test('first the count moves into the search placeholder', async ({ page }) => {
        await at(page, 1000)
        await expect(page.locator('.status .meta')).toHaveCount(0)
        await expect(page.getByRole('searchbox', { name: 'Search commits' })).toHaveAttribute('placeholder', 'Search 411 commits')
        expect(await worded(page, 'Undo')).toBe(true)
        await oneLine(page)
      })

      test('then the search becomes an icon', async ({ page }) => {
        await at(page, 880)
        await expect(page.getByRole('button', { name: 'Search commits' })).toBeVisible()
        await expect(page.getByRole('searchbox', { name: 'Search commits' })).toBeHidden()
        expect(await worded(page, 'Undo')).toBe(true)
        await oneLine(page)
        // Opened, it covers the bar, toolbar included, and Escape gives the bar back.
        await page.getByRole('button', { name: 'Search commits' }).click()
        const field = (await page.locator('.search').boundingBox())!
        const box = (await theBar(page).boundingBox())!
        expect(field.width).toBeGreaterThanOrEqual(box.width - 1)
        await expect(page.getByRole('searchbox', { name: 'Search commits' })).toBeFocused()
        await page.keyboard.press('Escape')
        await expect(page.getByRole('button', { name: 'Search commits' })).toBeVisible()
        await oneLine(page)
      })

      test('then Undo and Redo lose their words, and keep their names', async ({ page }) => {
        await at(page, 780)
        expect(await worded(page, 'Undo')).toBe(false)
        expect(await worded(page, 'Redo')).toBe(false)
        for (const name of ['Fetch', 'Pull', 'Push']) expect(await worded(page, name)).toBe(true)
        await oneLine(page)
      })

      test('last, Fetch, Pull and Push lose theirs', async ({ page }) => {
        await at(page, 600)
        for (const name of ['Undo', 'Redo', 'Fetch', 'Pull', 'Push']) expect(await worded(page, name)).toBe(false)
        await expect(page.getByRole('button', { name: /^Kinds/ })).toBeVisible()
        await expect(page.getByRole('button', { name: /^Graph view/ })).toBeVisible()
        await oneLine(page)
      })
    })

    // #583: a plain click on an ignored file opened its menu, as if it were a right-click.
    test('an ignored file opens its menu on right-click only', async ({ page }) => {
      const s = dirty([one])
      const ignored = { files: [{ path: 'out.log', rule: { source: '.gitignore', line: 1, pattern: '*.log' }, canExcept: true }], truncated: false }
      await open(page, 'repoView', { ...s, responses: { ...s.responses, IgnoredFiles: ignored } })
      await page.getByRole('button', { name: 'Ignored' }).click()
      const row = page.locator('button.name.ig')
      await expect(row).toBeVisible()
      await row.click()
      await expect(page.getByRole('menu')).toHaveCount(0)
      await row.click({ button: 'right' })
      await expect(page.getByRole('menu')).toBeVisible()
      // The keyboard's way in still works: the Menu key raises the same event on the focused row.
      await page.keyboard.press('Escape')
      await expect(page.getByRole('menu')).toHaveCount(0)
      await row.focus()
      await row.dispatchEvent('contextmenu')
      await expect(page.getByRole('menu')).toBeVisible()
    })

    // #556: it's the graph's first row, so its words start where the messages do and its mark
    // sits on the first lane. (#514 pinned it at 44px while it sat above the graph, where the
    // columns could leave it stranded; inside the graph it leaves with the graph instead.)
    test('is the first row of the graph, lined up with it, and leaves with it', async ({ page }) => {
      await page.setViewportSize({ width: 1600, height: 800 })
      await open(page, 'repoView', dirty([one]))
      await expect(bar(page)).toContainText('1 file changed')
      const scroller = (await page.locator('.scroller').boundingBox())!
      expect((await bar(page).boundingBox())!.y).toBe(scroller.y)
      // Compact names its columns, which says where the messages start.
      await pickView(page, 'Compact')
      const message = (await page.locator('.columns').getByText('Message').boundingBox())!
      const title = (await bar(page).locator('.wip-title').boundingBox())!
      expect(Math.abs(title.x - message.x)).toBeLessThanOrEqual(2)
      const dot = (await bar(page).locator('.dot').boundingBox())!
      expect(dot.x + dot.width).toBeLessThanOrEqual(title.x)

      // With a file open the graph steps aside, and its first row with it.
      await bar(page).click()
      await page.getByRole('button', { name: /a\.txt/ }).first().click()
      await expect(page.locator('.scroller')).toBeHidden()
      await expect(bar(page)).toBeHidden()
    })
  })

  // #439: adding a trailer meant remembering the exact format.
  test.describe('the trailer helper', () => {
    const one = { path: 'a.txt', index: 'M', workTree: '.', untracked: false, conflicted: false }
    const scenario = {
      graphRows: 1,
      responses: {
        Refs: [],
        CommitSignatures: [],
        WorkingTree: { status: { files: [one] }, identity: { name: 'Ris Adams', email: 'ris@example.com' }, signing: { enabled: false } },
        TrailerHints: { keys: ['Change-Id'], people: ['Ana Cruz <ana@example.com>'] },
      },
    }
    const description = (page: Page) => page.getByRole('textbox', { name: 'Commit description' })
    const openHelper = async (page: Page) => {
      await open(page, 'repoView', scenario)
      await page.getByRole('button', { name: 'Trailer' }).click()
    }

    test("offers the history's keys and people, then the common keys", async ({ page }) => {
      await openHelper(page)
      const menu = page.getByRole('menu')
      await expect(menu.getByRole('menuitem', { name: 'Change-Id:' })).toBeVisible()
      await expect(menu.getByRole('menuitem', { name: 'Closes:' })).toBeVisible()
      await expect(menu.getByRole('menuitem', { name: 'Co-authored-by: Ana Cruz' })).toBeVisible()
    })

    test('adds a person in full, and opens the description', async ({ page }) => {
      await openHelper(page)
      await page.getByRole('menuitem', { name: 'Co-authored-by: Ana Cruz' }).click()
      await expect(description(page)).toBeVisible()
      await expect(description(page)).toHaveValue('Co-authored-by: Ana Cruz <ana@example.com>')
    })

    test('a key leaves the cursor ready for its value', async ({ page }) => {
      await openHelper(page)
      await page.getByRole('menuitem', { name: 'Closes:' }).click()
      await expect(description(page)).toBeFocused()
      await page.keyboard.type('#12')
      await expect(description(page)).toHaveValue('Closes: #12')
    })

    test('signing off fills in who you are', async ({ page }) => {
      await openHelper(page)
      await page.getByRole('menuitem', { name: 'Signed-off-by:' }).click()
      await expect(description(page)).toHaveValue('Signed-off-by: Ris Adams <ris@example.com>')
    })

    test('still offers the common keys without any history', async ({ page }) => {
      await open(page, 'repoView', { ...scenario, failures: { TrailerHints: 'nope' } })
      await page.getByRole('button', { name: 'Trailer' }).click()
      await expect(page.getByRole('menuitem', { name: 'Closes:' })).toBeVisible()
    })
  })

  // #425: two lists, and paths cut off where the filename is.
  test.describe('the list of changes', () => {
    const file = (path: string, index: string, workTree: string, extra: object = {}) => ({ path, index, workTree, untracked: false, conflicted: false, ...extra })
    const scenario = (files: unknown[]) => ({ responses: { WorkingTree: { status: { files }, identity: { name: 'A', email: 'a@b' }, signing: { enabled: false } } } })
    const tick = (page: Page, path: string) => page.getByRole('checkbox', { name: `Include ${path} in the commit` })

    test('shows files by name under their folders, with line counts', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('src/lib/a.ts', '.', 'M', { added: 12, deleted: 3 }), file('b.png', '.', 'M', { binary: true })]))
      await expect(page.getByText('src/lib/', { exact: true })).toBeVisible()
      const row = page.locator('button.name', { hasText: 'a.ts' })
      await expect(row).toContainText('+12')
      await expect(row).toContainText('−3')
      await expect(page.locator('button.name', { hasText: 'b.png' })).toContainText('binary')
    })

    test('a tick shows whether a file is going in, all or part of it', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('none.txt', '.', 'M'), file('all.txt', 'M', '.'), file('part.txt', 'M', 'M')]))
      await expect(tick(page, 'none.txt')).not.toBeChecked()
      await expect(tick(page, 'all.txt')).toBeChecked()
      expect(await tick(page, 'part.txt').evaluate((e) => (e as HTMLInputElement).indeterminate)).toBe(true)
      await expect(page.getByText('partly staged')).toBeVisible()
    })

    test('ticking stages a file, and unticking unstages it', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('none.txt', '.', 'M'), file('all.txt', 'M', '.')]))
      await tick(page, 'none.txt').click()
      await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', ['none.txt']]])
      await tick(page, 'all.txt').click()
      await expect.poll(() => calls(page, 'UnstageFiles')).toEqual([['/r', ['all.txt']]])
    })

    test('ticking a partly staged file stages the rest', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('part.txt', 'M', 'M')]))
      await tick(page, 'part.txt').click()
      await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', ['part.txt']]])
    })

    test('a failed stage leaves the tick as it was', async ({ page }) => {
      await open(page, 'workingTree', { ...scenario([file('none.txt', '.', 'M')]), failures: { StageFiles: 'nope' } })
      await tick(page, 'none.txt').click()
      await expect.poll(() => calls(page, 'StageFiles').then((c) => c.length)).toBe(1)
      await expect(tick(page, 'none.txt')).not.toBeChecked()
    })

    test('the tick at the top stages everything, then unstages everything', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', '.', 'M'), file('b.txt', 'M', '.')]))
      await expect(page.getByText('1 of 2 ticked')).toBeVisible()
      await page.getByRole('checkbox', { name: '1 of 2 ticked' }).click()
      await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', []]])
    })

    test('opening a partly staged file shows what is still unstaged', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('part.txt', 'M', 'M')]))
      await page.locator('button.name', { hasText: 'part.txt' }).click()
      await expect.poll(async () => (await events(page, 'open')).at(-1)?.[0]).toMatchObject({ mode: 'unstaged', file: 'part.txt' })
    })

    // #426: "Commit 0 files" sat disabled without saying what was missing.
    test('the button and the line under the heading say the next step', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', '.', 'M')]))
      await expect(page.getByRole('button', { name: 'Tick a file to commit it' })).toBeDisabled()
      await expect(page.getByText('Tick what belongs in this commit')).toBeVisible()
    })

    test('then asks what the commit does, and then names the files', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', 'M', '.'), file('b.txt', 'A', '.')]))
      await expect(page.getByRole('button', { name: 'Describe it to commit 2 files' })).toBeDisabled()
      await expect(page.getByText('2 files going in. Now say what the commit does.')).toBeVisible()
      await page.getByRole('textbox', { name: 'Commit summary' }).fill('Change things')
      await expect(page.getByRole('button', { name: /^Commit 2 files/ })).toBeEnabled()
    })

    test('blocks the commit while there are conflicts', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', 'U', 'U', { conflicted: true }), file('b.txt', 'M', '.')]))
      await page.getByRole('textbox', { name: 'Commit summary' }).fill('Merge')
      await expect(page.getByRole('button', { name: 'Resolve 1 conflict to commit' })).toBeDisabled()
    })

    // #428: a kernel checkout on Windows shows 13 changes nobody made.
    test.describe('files that differ only by case', () => {
      const clash = (path: string) => file(path, '.', 'M', { caseClash: true })

      test('says they may not be your edits, and marks them', async ({ page }) => {
        await open(page, 'workingTree', scenario([clash('net/xt_DSCP.c'), clash('net/xt_HL.c'), file('real.txt', '.', 'M')]))
        await expect(page.getByRole('note')).toContainText('These 2 may not be your edits.')
        await expect(page.getByText('case clash')).toHaveCount(2)
      })

      test('the tick at the top leaves them out', async ({ page }) => {
        await open(page, 'workingTree', scenario([clash('net/xt_DSCP.c'), file('real.txt', '.', 'M')]))
        await page.getByRole('checkbox', { name: 'Tick all 1' }).click()
        await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', ['real.txt']]])
      })

      test('can still be ticked one at a time', async ({ page }) => {
        await open(page, 'workingTree', scenario([clash('net/xt_DSCP.c'), file('real.txt', '.', 'M')]))
        await tick(page, 'net/xt_DSCP.c').click()
        await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', ['net/xt_DSCP.c']]])
      })

      test('the notice can be set aside, without staging or changing anything', async ({ page }) => {
        await open(page, 'workingTree', scenario([clash('net/xt_DSCP.c'), clash('net/xt_HL.c')]))
        await page.getByRole('button', { name: 'Leave these 2 out' }).click()
        await expect(page.getByRole('note')).toHaveCount(0)
        await expect(tick(page, 'net/xt_DSCP.c')).not.toBeChecked()
        expect(await calls(page, 'StageFiles')).toEqual([])
      })

      test('no notice without a clash', async ({ page }) => {
        await open(page, 'workingTree', scenario([file('real.txt', '.', 'M')]))
        await expect(page.getByRole('checkbox', { name: 'Tick all 1' })).toBeVisible()
        await expect(page.getByRole('note')).toHaveCount(0)
      })
    })

    // #429: a commit on a detached HEAD is easy to lose, and the panel didn't say so.
    test.describe('on a detached HEAD', () => {
      const detached = (files: unknown[]) => ({ ...scenario(files), head: 'detached at c600b04' })

      test('says so, and offers a branch first', async ({ page }) => {
        await open(page, 'workingTree', detached([file('a.txt', '.', 'M')]))
        await expect(page.getByRole('note')).toContainText("You're not on a branch.")
        await expect(page.getByRole('note')).toContainText('detached at c600b04')
        await page.getByRole('button', { name: 'Create a branch here' }).click()
        expect(await events(page, 'new-branch')).toHaveLength(1)
      })

      test('committing anyway says what it is', async ({ page }) => {
        await open(page, 'workingTree', detached([file('a.txt', 'M', '.')]))
        await page.getByRole('textbox', { name: 'Commit summary' }).fill('Quick fix')
        const button = page.getByRole('button', { name: 'Commit anyway, without a branch' })
        await expect(button).toBeEnabled()
      })

      test('says nothing without changes', async ({ page }) => {
        await open(page, 'workingTree', detached([]))
        await expect(page.getByText('Nothing to commit')).toBeVisible()
        await expect(page.getByRole('note')).toHaveCount(0)
      })

      test('on a branch, names it and says nothing more', async ({ page }) => {
        await open(page, 'workingTree', { ...scenario([file('a.txt', 'M', '.')]), head: 'main' })
        await expect(page.getByRole('note')).toHaveCount(0)
        await page.getByRole('textbox', { name: 'Commit summary' }).fill('Fix')
        await expect(page.getByRole('button', { name: 'Commit 1 file to main' })).toBeEnabled()
      })
    })

    // #440: the same tools in the working tree.
    test('filters a long list, and offers a tree', async ({ page }) => {
      const names = ['src/a.ts', 'src/b.ts', 'src/c.ts', 'docs/d.md', 'docs/e.md']
      await open(page, 'workingTree', scenario(names.map((n) => file(n, '.', 'M'))))
      await page.getByRole('searchbox', { name: 'Filter files' }).fill('docs')
      await expect(page.locator('button.name')).toHaveCount(2)
      await page.getByRole('searchbox', { name: 'Filter files' }).fill('')
      await page.getByRole('button', { name: 'Tree' }).click()
      await page.getByRole('button', { name: 'src/', exact: true }).click()
      await expect(page.locator('button.name')).toHaveCount(2)
    })

    // #437: reviewing files took a lot of mouse travel.
    test.describe('from the keyboard', () => {
      const three = [file('a/one.txt', '.', 'M'), file('a/two.txt', '.', 'M'), file('b/three.txt', '.', 'M')]
      const row = (page: Page, name: string) => page.locator('button.name', { hasText: name })

      test('the arrow keys move between files, past folder headings', async ({ page }) => {
        await open(page, 'workingTree', scenario(three))
        await row(page, 'one.txt').focus()
        await page.keyboard.press('ArrowDown')
        await expect(row(page, 'two.txt')).toBeFocused()
        await page.keyboard.press('ArrowDown')
        await expect(row(page, 'three.txt')).toBeFocused()
        await page.keyboard.press('Home')
        await expect(row(page, 'one.txt')).toBeFocused()
        await page.keyboard.press('End')
        await expect(row(page, 'three.txt')).toBeFocused()
        await page.keyboard.press('ArrowUp')
        await expect(row(page, 'two.txt')).toBeFocused()
      })

      test('Space ticks the file, without opening it', async ({ page }) => {
        await open(page, 'workingTree', scenario(three))
        await row(page, 'two.txt').focus()
        await page.keyboard.press('Space')
        await expect.poll(() => calls(page, 'StageFiles')).toEqual([['/r', ['a/two.txt']]])
        expect(await events(page, 'open')).toEqual([])
      })

      test('Enter opens the file', async ({ page }) => {
        await open(page, 'workingTree', scenario(three))
        await row(page, 'two.txt').focus()
        await page.keyboard.press('Enter')
        await expect.poll(async () => (await events(page, 'open')).at(-1)?.[0]).toMatchObject({ file: 'a/two.txt' })
      })

      test('with a diff open, Alt and the arrows move to the next and previous file', async ({ page }) => {
        await open(page, 'workingTree', { ...scenario(three), commandKeys: true, diffFile: 'a/two.txt' })
        await expect(row(page, 'two.txt')).toBeVisible()
        await page.keyboard.press('Alt+ArrowDown')
        await expect.poll(async () => (await events(page, 'open')).at(-1)?.[0]).toMatchObject({ file: 'b/three.txt' })
        await page.keyboard.press('Alt+ArrowUp')
        await expect.poll(async () => (await events(page, 'open')).at(-1)?.[0]).toMatchObject({ file: 'a/one.txt' })
      })
    })

    // #438: nothing confirmed a commit, or offered a way back.
    test.describe('after committing', () => {
      const ready = (undo = 'Commit “Fix”') => ({
        ...scenario([file('a.txt', 'M', '.')]),
        responses: { ...scenario([file('a.txt', 'M', '.')]).responses, CommitChanges: 'abc1234def', UndoState: { undo } },
      })
      const commit = async (page: Page) => {
        await page.getByRole('textbox', { name: 'Commit summary' }).fill('Fix')
        await page.getByRole('button', { name: /^Commit 1 file/ }).click()
        await expect(page.getByRole('status')).toContainText('Committed abc1234')
      }

      test('says so, and clears the form', async ({ page }) => {
        await open(page, 'workingTree', ready())
        await commit(page)
        await expect(page.getByRole('textbox', { name: 'Commit summary' })).toHaveValue('')
      })

      test('Undo takes the commit back, and restores the message', async ({ page }) => {
        await open(page, 'workingTree', ready())
        await commit(page)
        await page.getByRole('button', { name: 'Undo', exact: true }).click()
        await expect.poll(() => calls(page, 'Undo')).toEqual([['/r']])
        await expect(page.getByRole('textbox', { name: 'Commit summary' })).toHaveValue('Fix')
        await expect(page.getByRole('status')).toHaveCount(0)
      })

      test("Undo keeps a message you've started since", async ({ page }) => {
        await open(page, 'workingTree', ready())
        await commit(page)
        await page.getByRole('textbox', { name: 'Commit summary' }).fill('Newer')
        await page.getByRole('button', { name: 'Undo', exact: true }).click()
        await expect.poll(() => calls(page, 'Undo')).toEqual([['/r']])
        await expect(page.getByRole('textbox', { name: 'Commit summary' })).toHaveValue('Newer')
      })

      test("doesn't undo something else that happened since", async ({ page }) => {
        await open(page, 'workingTree', ready())
        await commit(page)
        await page.evaluate(() => {
          ;(window as unknown as { __scenario: { responses: { UndoState: { undo: string } } } }).__scenario.responses.UndoState.undo = 'Delete branch x'
        })
        await page.getByRole('button', { name: 'Undo', exact: true }).click()
        await expect(page.getByRole('status')).toHaveCount(0)
        expect(await calls(page, 'Undo')).toEqual([])
      })
    })

    test('says so when there is nothing to commit', async ({ page }) => {
      await open(page, 'workingTree', scenario([]))
      await expect(page.getByText('Nothing to commit')).toBeVisible()
    })

    test('conflicts come first', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', 'U', 'U', { conflicted: true }), file('b.txt', '.', 'M')]))
      await expect(page.getByText('Fix these first · 1')).toBeVisible()
      await expect(tick(page, 'a.txt')).toHaveCount(0)
    })

    test('the description opens on request', async ({ page }) => {
      await open(page, 'workingTree', scenario([file('a.txt', '.', 'M')]))
      const description = page.getByRole('textbox', { name: 'Commit description' })
      await expect(description).toBeHidden()
      await page.getByRole('button', { name: 'Description' }).click()
      await expect(description).toBeVisible()
    })
  })

  const tree = (path: string, main: boolean) => ({ path, head: 'abc', branch: 'b', main, current: main, locked: false, prunable: false })
  const heading = (page: Page) => page.getByRole('heading', { name: /Worktrees/ })

  test('the Repository panel shows no worktree count until it knows', async ({ page }) => {
    await open(page, 'repoPanel', { delays: { Worktrees: 500 }, responses: { Worktrees: [tree('/r', true), tree('/r-wt', false)], Submodules: [] } })
    await expect(heading(page)).toHaveText('Worktrees')
    await expect(heading(page)).toHaveText('Worktrees 2')
  })

  // #388: worktrees waited for submodules and LFS, which can take seconds.
  test('the Repository panel shows worktrees without waiting for the slower sections', async ({ page }) => {
    await open(page, 'repoPanel', { delays: { Submodules: 5000, LFS: 5000 }, responses: { Worktrees: [tree('/r', true), tree('/r-wt', false)], Submodules: [] } })
    await expect(heading(page)).toHaveText('Worktrees 2', { timeout: 2000 })
    await expect(page.getByRole('button', { name: 'r-wt' })).toBeVisible()
  })

  test('a repository with no worktrees listed still reads 0 once loaded', async ({ page }) => {
    await open(page, 'repoPanel', { responses: { Worktrees: [], Submodules: [] } })
    await expect(heading(page)).toHaveText('Worktrees 0')
  })
})

// #391: staging finished after the diff closed, and reloading it showed a null error.
test('closing a diff while staging a hunk shows no error', async ({ page }) => {
  const side = (text: string) => ({ text, exists: true, binary: false, tooBig: false, size: text.length })
  await open(page, 'closingDiff', { delays: { SetStagedContent: 400 }, responses: { FileContents: { old: side('one\n'), new: side('one\ntwo\n') } } })
  await page.getByRole('button', { name: 'Stage hunk' }).click()
  await page.evaluate(() => Object.assign(window, { __diffTarget: null }))
  await expect.poll(() => calls(page, 'SetStagedContent').then((c) => c.length)).toBe(1)
  await page.waitForTimeout(800)
  expect(await page.evaluate(() => (window as unknown as { __sessionError: () => string }).__sessionError())).toBe('')
})

test.describe('pulling a branch that is not checked out', () => {
  const refs = [branch('main', { head: true }), branch('feat', { upstream: 'origin/feat' })]
  const pull = async (page: Page) => {
    await page.getByRole('menuitem', { name: 'feat', exact: true }).hover()
    await page.getByRole('menuitem', { name: 'Pull feat' }).click()
  }

  test('asks the backend for that branch', async ({ page }) => {
    await open(page, 'commitMenu', { responses: { Refs: refs } })
    await pull(page)
    await expect.poll(() => calls(page, 'PullBranch')).toEqual([['/r', 'feat']])
  })

  test('shows why when the branch has diverged', async ({ page }) => {
    const message = 'feat has diverged from its upstream. Check it out to merge or rebase.'
    await open(page, 'commitMenu', { responses: { Refs: refs }, failures: { PullBranch: message } })
    await pull(page)
    await expect.poll(() => page.evaluate(() => (window as unknown as { __sessionError: () => string }).__sessionError())).toBe(message)
  })
})

// #517
test.describe('an MDX file in the rendered preview', () => {
  const text = `---
title: Getting started
---

import Tabs from './Tabs'
export const version = alert('ran')

# Getting started

{/* a note for authors */}

Some **bold** text with a [link](other.mdx) and \`<Inline />\` code.

<Tabs>
  <TabItem value="one" onClick={() => alert('ran')}>
    Inside a tab

    - first
    - second
  </TabItem>
</Tabs>

<Figure src="a.png" />

\`\`\`jsx
import Kept from 'kept'
<Kept />
\`\`\`
`

  test('renders the Markdown and hides the code around it', async ({ page }) => {
    const dialogs: string[] = []
    page.on('dialog', (d) => {
      dialogs.push(d.message())
      void d.dismiss()
    })
    await open(page, 'markdownPreview', { responses: { File: { path: 'docs/start.mdx', text } } })
    await expect(page.getByRole('heading', { name: 'Getting started' })).toBeVisible()
    await expect(page.locator('strong')).toHaveText('bold')
    await expect(page.locator('li')).toHaveText(['first', 'second'])
    await expect(page.locator('p', { hasText: 'Inside a tab' })).toBeVisible()
    await expect(page.locator('code', { hasText: '<Inline />' })).toBeVisible()
    await expect(page.locator('code', { hasText: '<Figure />' })).toBeVisible()
    await expect(page.locator('pre code')).toHaveText("import Kept from 'kept'\n<Kept />\n")

    const shown = (await page.locator('.preview').innerText()).replace(/\s+/g, ' ')
    for (const hidden of ['title:', 'import Tabs', 'export const', 'a note for authors', '<Tabs', 'TabItem', 'onClick']) expect(shown).not.toContain(hidden)
    expect(dialogs).toEqual([])
  })

  test('opens a relative link', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'docs/start.mdx', text } } })
    await page.getByRole('link', { name: 'link' }).click()
    await expect.poll(() => events(page, 'openFile')).toEqual([['docs/other.mdx']])
  })

  test('leaves a plain Markdown file as it was', async ({ page }) => {
    await open(page, 'markdownPreview', { responses: { File: { path: 'a.md', text: '# T\n\nimport x\n\n<Tabs>\n' } } })
    await expect(page.locator('.preview')).toContainText('import x')
    await expect(page.locator('.preview')).toContainText('<Tabs>')
  })
})

// #516
test.describe('pushing and pulling one branch from its menu', () => {
  const refs = [branch('main', { head: true, upstream: 'origin/main' }), branch('feat', { upstream: 'origin/other' }), branch('fresh')]
  const item = async (page: Page, submenu: string, name: string) => {
    await open(page, 'commitMenu', { responses: { Refs: refs } })
    await page.getByRole('menuitem', { name: submenu, exact: true }).hover()
    await page.getByRole('menuitem', { name, exact: true }).click()
  }

  test('pushes a branch that is not checked out', async ({ page }) => {
    await item(page, 'feat', 'Push feat to origin/other')
    await expect.poll(() => calls(page, 'PushBranch')).toEqual([['/r', 'feat']])
  })

  test('pushes a branch that has no upstream yet', async ({ page }) => {
    await item(page, 'fresh', 'Push fresh')
    await expect.poll(() => calls(page, 'PushBranch')).toEqual([['/r', 'fresh']])
  })

  test('pushes the checked-out branch', async ({ page }) => {
    await item(page, 'main', 'Push main to origin')
    await expect.poll(() => calls(page, 'PushBranch')).toEqual([['/r', 'main']])
  })

  test('pulls the checked-out branch the way the toolbar does', async ({ page }) => {
    await item(page, 'main', 'Pull main')
    await expect.poll(() => calls(page, 'Pull')).toEqual([['/r', '']])
    expect(await calls(page, 'PullBranch')).toEqual([])
  })
})

test.describe('views loaded on demand', () => {
  test('shows the view once its code loads', async ({ page }) => {
    await open(page, 'lazy')
    await expect(page.locator('#host svg')).toBeVisible()
  })

  test('says so when its code can\'t load', async ({ page }) => {
    await open(page, 'lazy', { failures: { lazy: 'the file is missing' } })
    await expect(page.getByRole('alert')).toContainText("Couldn't load this view: the file is missing")
  })

  test('the real diff view loads and gets its props', async ({ page }) => {
    await open(page, 'lazyDiff')
    await expect(page.getByText('lazy-file.ts').first()).toBeVisible()
  })

  test('math renders once KaTeX loads', async ({ page }) => {
    await open(page, 'math')
    await expect(page.locator('math')).toBeAttached()
  })
})

test('no text is under the 12px floor', async ({ page }) => {
  await open(page, 'sidebar', { responses: { Refs: [branch('fix/a', { ahead: 1 }), branch('fix/b', { gone: true }), tag('v1')], ShowingHidden: false } })
  await expect(page.locator('.row').first()).toBeVisible()
  expect(await textUnderFloor(page)).toEqual([])
})

test('draws a commit with several refs', async ({ page }) => {
  const id = '0'.repeat(40)
  await open(page, 'graph', { graphRows: 50, responses: { Refs: [], CommitSignatures: [] }, graphRefs: { [id]: [branch('main', { head: true, branchKind: 'trunk' }), remote('origin/main', { branchKind: 'trunk' }), branch('dependabot/x', { branchKind: 'bot' }), tag('v1.0')] } })
  await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '50')
  await page.waitForTimeout(300)
  expect(await events(page, 'select')).not.toHaveLength(0)
})

test('loading text waits, so a fast load does not flash', async ({ page }) => {
  // On a stopped clock, so a slow browser can't reach the delay before the first check.
  await page.clock.install({ time: 0 })
  await page.clock.pauseAt(1000)
  await open(page, 'loading')
  await expect(page.getByRole('status')).toHaveCount(0)
  await page.clock.runFor(1000)
  await expect(page.getByRole('status')).toContainText('Loading…')
})

// #406: a component's own .danger rule beat the button family's, so the label matched its fill.
test.describe('confirm buttons are legible', () => {
  const contrast = (button: Locator) =>
    button.evaluate((el) => {
      const channels = (c: string) => (c.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number)
      const luminance = (c: string) => {
        const [r, g, b] = channels(c).map((v) => {
          const x = v / 255
          return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4
        })
        return 0.2126 * r + 0.7152 * g + 0.0722 * b
      }
      const [a, b] = [luminance(getComputedStyle(el).color), luminance(getComputedStyle(el, '::after').backgroundColor)]
      return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
    })
  const tree = (path: string, main: boolean) => ({ path, head: 'abc', branch: 'b', main, current: main, locked: false, prunable: false })
  const files = ['a.txt', 'b.txt'].map((path) => ({ path, index: '.', workTree: 'M', untracked: false, conflicted: false }))
  const responses = { WorkingTree: { status: { files }, identity: { name: 'A', email: 'a@b' }, signing: { enabled: false } } }

  for (const theme of ['light', 'dark'] as const) {
    test(`Discard all, ${theme}`, async ({ page }) => {
      await open(page, 'workingTree', { theme, responses })
      await page.getByRole('button', { name: 'Discard all…' }).click()
      const yes = page.getByRole('button', { name: 'Yes', exact: true })
      expect(await contrast(yes)).toBeGreaterThanOrEqual(4.5)
      await yes.hover()
      expect(await contrast(yes)).toBeGreaterThanOrEqual(4.5)
    })

    test(`discarding one file, ${theme}`, async ({ page }) => {
      await open(page, 'workingTree', { theme, responses })
      await page.getByText('a.txt').hover()
      await page.getByRole('button', { name: 'Discard changes to a.txt' }).click()
      expect(await contrast(page.getByRole('button', { name: 'Yes', exact: true }))).toBeGreaterThanOrEqual(4.5)
    })

    test(`removing a worktree, ${theme}`, async ({ page }) => {
      await open(page, 'worktreesDialog', { theme, responses: { Worktrees: [tree('/r', true), tree('/r-wt', false)] } })
      await page.getByRole('button', { name: 'Remove' }).click()
      expect(await contrast(page.getByRole('button', { name: 'Remove', exact: true }))).toBeGreaterThanOrEqual(4.5)
    })
  }
})

import { expect, test, type Locator, type Page } from '@playwright/test'
import { branch, calls, events, open, remote, tag, textUnderFloor } from './support'

test.describe('commit menu', () => {
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

  // #385: the words that tell two merge items apart were the ones cut off.
  test.describe('a long label', () => {
    const merge = (name: string) => `Merge ${name} into main (no fast-forward)`
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

test.describe('commit graph', () => {
  /** The row just inside the top edge, read by right-clicking it. */
  async function topRow(page: Page) {
    const box = (await page.locator('.scroller').boundingBox())!
    await page.mouse.click(box.x + 400, box.y + 2, { button: 'right' })
    return (await events(page, 'context-row')).at(-1)?.[0]
  }

  // #383: the new toggles took the room the count had, and the search count ran off the edge.
  test.describe('in a narrow column', () => {
    const narrow = { graphRows: 32, responses: { CommitSignatures: [], Refs: [branch('docs/site', { branchKind: 'docs' }), branch('dependabot/x', { branchKind: 'bot' })] } }

    test('the header keeps the commit count on one line', async ({ page }) => {
      await page.setViewportSize({ width: 640, height: 600 })
      await open(page, 'graph', narrow)
      await expect(page.getByRole('button', { name: 'Bots' })).toBeVisible()
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

    test('the search takes its own line when nothing else fits', async ({ page }) => {
      await page.setViewportSize({ width: 420, height: 600 })
      await open(page, 'graph', narrow)
      const count = (await page.getByText('32 commits').boundingBox())!
      const search = (await page.getByRole('searchbox', { name: 'Search commits' }).boundingBox())!
      expect(search.y).toBeGreaterThan(count.y + count.height)
      expect(search.x + search.width).toBeLessThanOrEqual(420)
    })
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
      page.locator('canvas').evaluate(
        (el, [px, py]) => {
          const dpr = window.devicePixelRatio || 1
          return (el as HTMLCanvasElement).getContext('2d')!.getImageData(px * dpr, py * dpr, 1, 1).data[3]
        },
        [x, y],
      )

    test('headers name the columns when there is room', async ({ page }) => {
      await open(page, 'graph', scenario)
      const header = page.locator('.columns')
      for (const name of ['Branch / tag', 'Graph', 'Message', 'SHA']) await expect(header.getByText(name)).toBeVisible()
      // The timeline shows the author and date under the subject, not in columns of their own.
      await expect(header.getByText('Author')).toHaveCount(0)
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

    test("a branch's row carries its lane's colour from the node toward the message", async ({ page }) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      // Between the node and the message, on an unselected row: lane 2's wash and nothing else.
      await expect.poll(() => inkAt(page, lane(2) + 15, 78)).toBeGreaterThan(0)
      // Left of the node, between two idle lanes: nothing at all.
      expect(await inkAt(page, lane(1) + 8, 78)).toBe(0)
      // The gap between rows stays clear, so a run of commits doesn't merge into a block.
      expect(await inkAt(page, lane(2) + 15, 104)).toBe(0)
    })

    for (const kind of ['trunk', 'docs']) {
      test(`a ${kind} row has no wash`, async ({ page }) => {
        const rows = graphData.map((r) => ({ ...r, kind }))
        await open(page, 'graph', { ...scenario, graphData: rows })
        await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
        // The lane itself draws, so the row has painted by now.
        await expect.poll(() => inkAt(page, lane(2), 78)).toBeGreaterThan(0)
        expect(await inkAt(page, lane(2) + 15, 78)).toBe(0)
      })
    }

    test('the wash stops before the message', async ({ page }) => {
      await open(page, 'graph', scenario)
      await expect(page.locator('.scroller')).toHaveAttribute('aria-rowcount', '20')
      await expect.poll(() => inkAt(page, lane(2) + 15, 78)).toBeGreaterThan(0)
      // Three lanes wide: the message starts at 150 + (2 * 10 + 3 * 16) + 6 = 224. Past the
      // subject's first letters, on the row's clear lower edge, is nothing.
      expect(await inkAt(page, 230, 78 + 18)).toBe(0)
    })

    test("the author button searches for the selected commit's author", async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 800 })
      await open(page, 'graph', scenario)
      await page.getByRole('button', { name: 'Compact' }).or(page.getByRole('button', { name: 'Timeline' })).click()
      await page.getByRole('button', { name: "Search for the selected commit's author" }).click()
      await expect(page.getByRole('searchbox', { name: 'Search commits' })).toHaveValue('Ana Cruz')
      await expect.poll(() => calls(page, 'SearchGraph')).toContainEqual(['/repo', 'Ana Cruz'])
    })

    test('the message column stays put when a wider row scrolls by', async ({ page }) => {
      const wideLater = graphData.map((r, i) => (i === 19 ? { ...r, width: 9 } : r))
      await page.setViewportSize({ width: 1280, height: 300 })
      await open(page, 'graph', { ...scenario, graphData: wideLater })
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

  // #418: +N didn't say what it was hiding.
  test.describe('hovering over labels', () => {
    const id = '0'.repeat(40)
    const graphRefs = { [id]: [branch('main', { head: true, branchKind: 'trunk' }), remote('origin/main', { branchKind: 'trunk' }), remote('fork/main', { branchKind: 'trunk' }), tag('v1.0'), branch('feature/a-rather-long-branch-name')] }
    const scenario = { graphRows: 5, graphRefs, responses: { Refs: graphRefs[id], CommitSignatures: [] } }
    /** The first row's labels, in the label column. */
    const overLabels = async (page: Page) => {
      const box = (await page.locator('.scroller').boundingBox())!
      return { x: box.x + 30, y: box.y + 26 }
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
      await page.mouse.move(x, y)
      const list = page.locator('.ref-expand')
      await expect(list).toBeVisible()
      const opening = await list.evaluate((el) => el.getAnimations().length)
      expect(opening).toBeGreaterThan(0)
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
      await page.mouse.click(box.x + 500, box.y + 26)
      await expect(page.getByRole('heading', { name: 'Commit 0' })).toBeVisible()
    }
    const tab = (page: Page, name: string) => page.getByRole('tab', { name, exact: true })

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
      await page.mouse.click(box.x + 500, box.y + 78)
      await expect(tab(page, 'Details')).toHaveAttribute('aria-selected', 'true')
      await page.reload()
      const again = (await page.locator('.scroller').boundingBox())!
      await page.mouse.click(again.x + 500, again.y + 26)
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

    test('says so when there is no description', async ({ page }) => {
      await openCommit(page, { body: '', trailers: [] })
      await expect(tab(page, 'Trailers')).toHaveCount(0)
      await expect(page.getByRole('tabpanel')).toContainText('No description.')
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
    await page.mouse.click(box.x + 500, box.y + 26)
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
    const before = await page.evaluate(() => {
      const scroller = [...document.querySelectorAll('*')].find((e) => e.scrollHeight > e.clientHeight + 40 && getComputedStyle(e).overflowY !== 'visible')
      return (scroller ?? document.scrollingElement)!.scrollTop
    })
    await page.mouse.wheel(0, 60)
    await page.waitForTimeout(150)
    const moved = await page.evaluate(() => {
      const scroller = [...document.querySelectorAll('*')].find((e) => e.scrollHeight > e.clientHeight + 40 && getComputedStyle(e).overflowY !== 'visible')
      return (scroller ?? document.scrollingElement)!.scrollTop
    })
    expect(moved).toBeGreaterThan(before)
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

test.describe('rebase editor', () => {
  const plan = [
    { commit: 'a'.repeat(40), subject: 'First', message: 'First', action: 'pick' },
    { commit: 'c'.repeat(40), subject: 'Second', message: 'Second', action: 'pick' },
  ]

  // #387: an empty list counted as every commit being dropped.
  test('warns about nothing while the commits load', async ({ page }) => {
    await open(page, 'rebaseEditor', { delays: { RebasePlan: 500 }, responses: { RebasePlan: plan } })
    await expect(page.getByText('Loading commits…')).toBeVisible()
    await expect(page.getByRole('status')).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Start rebase' })).toBeDisabled()
    await expect(page.getByText('2 commits · newest first')).toBeVisible()
    await expect(page.getByRole('status')).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Start rebase' })).toBeEnabled()
  })

  test('still warns when every commit really is dropped', async ({ page }) => {
    await open(page, 'rebaseEditor', { responses: { RebasePlan: plan } })
    await page.getByRole('combobox', { name: 'Action for First' }).selectOption('drop')
    await page.getByRole('combobox', { name: 'Action for Second' }).selectOption('drop')
    await expect(page.getByRole('status')).toHaveText('Every commit is dropped. Use Reset instead.')
  })

  test('says so when there is nothing to rebase', async ({ page }) => {
    await open(page, 'rebaseEditor', { responses: { RebasePlan: [] } })
    await expect(page.getByRole('status')).toHaveText('There are no commits to rebase onto this one.')
  })

  test('keeps its buttons on one line in a narrow column', async ({ page }) => {
    await page.setViewportSize({ width: 560, height: 600 })
    await open(page, 'rebaseEditor', { responses: { RebasePlan: plan } })
    const middle = async (name: string) => {
      const box = (await page.getByRole('button', { name }).boundingBox())!
      return box.y + box.height / 2
    }
    expect(await middle('Start rebase')).toBeCloseTo(await middle('Cancel'), 0)
    expect(await middle('Start rebase')).toBeCloseTo(await middle('Back'), 0)
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
    // The row lines up with the graph, which has to draw once to say where that is.
    await page.waitForTimeout(100)
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
      delays: { WorkingTree: 600 },
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

    test('lines its words up with the commit messages', async ({ page }) => {
      await page.setViewportSize({ width: 1600, height: 800 })
      await open(page, 'repoView', dirty([one]))
      await expect(bar(page)).toContainText('1 file changed')
      await expect.poll(async () => {
        const title = (await bar(page).locator('.wip-title').boundingBox())!
        const message = (await page.locator('.columns').getByText('Message').boundingBox())!
        return Math.abs(title.x - message.x)
      }).toBeLessThanOrEqual(1)
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
  await open(page, 'loading')
  await expect(page.getByRole('status')).toHaveCount(0)
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
      await page.getByRole('button', { name: 'Discard all' }).click()
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

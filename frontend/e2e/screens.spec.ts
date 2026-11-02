import { expect, test, type Page } from '@playwright/test'
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
      delays: { WorkingTree: 600 },
      responses: { CommitChanges: 'abc', WorkingTree: { status: { files }, identity: { name: 'A', email: 'a@b' }, signing: { enabled: false } } },
    })
    const summary = (page: Page) => page.getByRole('textbox', { name: 'Commit summary' })

    test('claims no counts until status has loaded', async ({ page }) => {
      await open(page, 'workingTree', panel([file('a.txt', 'M', '.'), file('b.txt', '.', 'M')]))
      await expect(page.getByText('Unstaged', { exact: true })).toBeVisible()
      await expect(page.getByText('Staged', { exact: true })).toBeVisible()
      await expect(page.getByRole('button', { name: 'Commit', exact: true })).toBeVisible()
      await expect(page.getByText('Unstaged · 1')).toBeVisible()
      await expect(page.getByText('Staged · 1', { exact: true })).toBeVisible()
      await expect(page.getByRole('button', { name: 'Commit 1 file' })).toBeVisible()
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
      await expect(page.getByRole('button', { name: 'Commit 0 files' })).toBeVisible()
      expect(await calls(page, 'CommitChanges')).toEqual([])
      await expect(summary(page)).toHaveValue('Early')
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

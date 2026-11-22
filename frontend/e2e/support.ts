import { expect, type Page } from '@playwright/test'

/** What a test tells the stubbed backend. See harness.ts. */
export interface Scenario {
  /** Answers by App method name. Anything else answers null. */
  responses?: Record<string, unknown>
  /** Methods that fail, with their error message. */
  failures?: Record<string, string>
  /** Methods that take this long to answer, in milliseconds. */
  delays?: Record<string, number>
  graphRows?: number
  /** The graph's rows as given, for a test about lanes; graphRows draws one straight lane. */
  graphData?: unknown[]
  /** The refs on each commit in the graph, by commit id. */
  graphRefs?: Record<string, unknown[]>
  selectedCommit?: string
  current?: string
  /** Dispatches shortcuts to commands, as the app does. */
  commandKeys?: boolean
  /** The file whose diff is open in the working tree panel. */
  diffFile?: string
  /** What's checked out, as the tab shows it. */
  head?: string
  menuX?: number
  /** Which preview the preview dialog shows; a merge unless said. */
  previewKind?: 'merge' | 'rebase'
  theme?: 'light' | 'dark'
  /** The graph is under a smart filter that takes commits out. */
  sparse?: boolean
  /** The graph is under a smart filter that highlights: the rows to tick on its ruler. */
  highlight?: number[]
  /** The trailer keys pinned to the graph's rows, by ID, as the sidebar is told. */
  pinnedKeys?: string[]
  /** The chips of pinned keys on the graph's rows, by commit id. With them the graph is
   * given a filter to pick into. */
  graphChips?: Record<string, { key: string; keyLabel: string; value: string; label: string; kind: string }[]>
  /** The share of the history whose trailers aren't read yet, as the graph is told. */
  trailersUnread?: number
  /** The graph is given a smart filter to add to, so its search box suggests. */
  suggests?: boolean
  /** The values the graph is filtered by, as the sidebar is told. */
  filterPicks?: { key: string; value: string }[]
  /** What the filter strip is given: the filter, the mode in force, and whether Only these is on offer. */
  filterStrip?: { filter: unknown; only?: boolean; onlyOffered?: boolean }
  /** Someone the People tab is opened on, as an author's card in the graph asks. */
  peopleAsk?: { person: unknown; pick: boolean }
  /** Gives the graph a working tree to show, with this many changed files. */
  wip?: { changes: number | null }
  /** Mounts the toolbar under the error banner, for the commands its actions run. */
  toolbar?: boolean
  /** A repository the backend won't open until it's trusted, and what it would ask (a TrustQuestion). */
  untrusted?: { path: string; name: string; findings: { kind: string; name: string; value: string; why: string }[] }
}

/** Opens one real component against the stub, and fails on any uncaught error. */
export async function open(page: Page, component: string, scenario: Scenario = {}): Promise<void> {
  page.on('pageerror', (e) => {
    throw e
  })
  await page.addInitScript((s) => Object.assign(window, { __scenario: s }), scenario)
  await page.goto(`/e2e/harness.html?mount=${component}`)
  await page.waitForFunction(() => (window as unknown as { __ready?: boolean }).__ready === true)
}

export const calls = (page: Page, method: string) =>
  page.evaluate((m) => (window as unknown as { __calls: unknown[][] }).__calls.filter((c) => c[0] === m).map((c) => c.slice(1)), method)

export const events = (page: Page, name: string) =>
  page.evaluate((n) => (window as unknown as { __events: unknown[][] }).__events.filter((e) => e[0] === n).map((e) => e.slice(1)), name)

export interface RefOptions {
  head?: boolean
  branchKind?: string
  gone?: boolean
  ahead?: number
  behind?: number
  upstream?: string
  stackParent?: string
  stackParentMerged?: boolean
  hidden?: boolean
  focused?: boolean
  worktree?: string
  subject?: string
  commit?: string
}

export function branch(short: string, options: RefOptions = {}) {
  return { name: `refs/heads/${short}`, short, kind: 'local', commit: `c-${short}`, head: false, upstream: '', ahead: 0, behind: 0, gone: false, ...options }
}

export const remote = (short: string, options: RefOptions = {}) => ({ ...branch(short, options), name: `refs/remotes/${short}`, kind: 'remote' })
export const tag = (short: string) => ({ ...branch(short), name: `refs/tags/${short}`, kind: 'tag' })

/** The visible rows of the sidebar's list, indented by depth. */
export const outline = (page: Page) =>
  page.locator('[role=tree] [role=treeitem]').evaluateAll((items) =>
    items.map((li) => {
      const level = Number(li.getAttribute('aria-level')) - 1
      const name = li.querySelector('.name')?.textContent ?? ''
      return '  '.repeat(level) + name + (li.getAttribute('aria-expanded') === null ? '' : '/')
    }),
  )

export const expectOutline = (page: Page, expected: string[]) => expect.poll(() => outline(page)).toEqual(expected)

/** Text smaller than the design system's 12px floor on the page, as "size text". */
export const textUnderFloor = (page: Page) =>
  page.evaluate(() => {
    const out: string[] = []
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const el = n.parentElement
      if (!n.textContent?.trim() || !el || el.closest('.visually-hidden, script, style')) continue
      const size = parseFloat(getComputedStyle(el).fontSize)
      if (size < 12) out.push(size + ' ' + n.textContent.trim().slice(0, 30))
    }
    return out
  })

/** Chooses a view of the graph from the bar's view menu, by the name on its menu item. */
export async function pickView(page: Page, name: string | RegExp): Promise<void> {
  await page.getByRole('button', { name: /^Graph view/ }).click()
  await page.getByRole('menuitemradio', { name }).click()
}

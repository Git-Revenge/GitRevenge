import { expect, type Page } from '@playwright/test'

/** What a test tells the stubbed backend. See harness.ts. */
export interface Scenario {
  /** Answers by App method name. Anything else answers null. */
  responses?: Record<string, unknown>
  /** Methods that fail, with their error message. */
  failures?: Record<string, string>
  graphRows?: number
  /** The refs on each commit in the graph, by commit id. */
  graphRefs?: Record<string, unknown[]>
  selectedCommit?: string
  current?: string
  theme?: 'light' | 'dark'
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

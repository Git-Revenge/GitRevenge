import { panelMatches, type PanelKey, type Panels, type SortOrder } from './sidebarTree'

/** One of the rail's panels. Repository isn't a ref panel, so it isn't in the tree. */
export type SidebarPanel = PanelKey | 'repo'

/** What the sidebar remembers for each repository, on this computer. */
export interface SidebarState {
  panel: SidebarPanel
  collapsed: boolean
  /** The panel's width in pixels, not counting the rail. */
  width: number
  /** Open folders the person chose, by folder key. */
  open: Record<string, boolean>
  sort: SortOrder
}

export const DEFAULT_WIDTH = 236
export const MIN_WIDTH = 160
export const MAX_WIDTH = 560

const PANELS: readonly SidebarPanel[] = ['branches', 'remotes', 'tags', 'stashes', 'repo']

export function defaultSidebar(): SidebarState {
  return { panel: 'branches', collapsed: false, width: DEFAULT_WIDTH, open: {}, sort: 'name' }
}

export function clampWidth(width: number): number {
  return Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, width)))
}

const keyFor = (path: string) => `gitrevenge.sidebar:${path}`

type Store = Pick<Storage, 'getItem' | 'setItem'>

/** The saved state for a repository. Anything missing or unreadable uses its default. */
export function loadSidebar(path: string, store: Store): SidebarState {
  const state = defaultSidebar()
  let saved: Partial<SidebarState> | null
  try {
    saved = JSON.parse(store.getItem(keyFor(path)) ?? 'null')
  } catch {
    return state
  }
  if (typeof saved !== 'object' || saved === null) return state
  if (saved.panel && PANELS.includes(saved.panel)) state.panel = saved.panel
  if (typeof saved.collapsed === 'boolean') state.collapsed = saved.collapsed
  if (typeof saved.width === 'number' && Number.isFinite(saved.width)) state.width = clampWidth(saved.width)
  if (saved.sort === 'recent') state.sort = 'recent'
  if (typeof saved.open === 'object' && saved.open !== null && !Array.isArray(saved.open)) {
    for (const [key, value] of Object.entries(saved.open)) if (typeof value === 'boolean') state.open[key] = value
  }
  return state
}

/** Saves the state. It's only a convenience, so a full or blocked store is ignored. */
export function saveSidebar(path: string, state: SidebarState, store: Store): void {
  try {
    store.setItem(keyFor(path), JSON.stringify(state))
  } catch {
    // Nothing to do: the sidebar still works, and just won't remember this.
  }
}

/** The first ref panel other than this one with filter matches, for an empty panel to name. */
export function otherPanelWithMatches(panels: Panels, current: SidebarPanel, query: string): { panel: PanelKey; count: number } | null {
  if (!query.trim()) return null
  for (const panel of Object.keys(panels) as PanelKey[]) {
    if (panel === current) continue
    const count = panelMatches(panels[panel], query)
    if (count > 0) return { panel, count }
  }
  return null
}

export const ROW_HEIGHT = 28

/** Which rows to draw for a scroll position: enough to fill the view, plus some either side. */
export function visibleRange(scrollTop: number, viewHeight: number, total: number, overscan = 12): { start: number; end: number } {
  const start = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - overscan)
  const end = Math.min(total, Math.ceil((scrollTop + viewHeight) / ROW_HEIGHT) + overscan)
  return { start: Math.min(start, end), end }
}

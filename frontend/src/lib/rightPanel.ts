/** What the right-hand panel remembers for each repository, on this computer. */
export interface RightPanelState {
  /** The width the person chose. What fits the window right now comes from fitWidth. */
  width: number
}

export const DEFAULT_WIDTH = 420
export const MIN_WIDTH = 320
/** The panel never takes more than this share of the window. */
const MAX_SHARE = 0.6
/** What the sidebar and the graph are left with, at least. */
const LEAVE_FOR_REST = 560
/** From this width, the panels lay out in two columns (see their container queries). */
export const TWO_COLUMNS = 620

/** The widest the panel can be in a window this wide. */
export function maxWidth(windowWidth: number): number {
  return Math.max(MIN_WIDTH, Math.round(Math.min(windowWidth * MAX_SHARE, windowWidth - LEAVE_FOR_REST)))
}

/** A width that fits. The chosen width is kept as it is, so a window made wider gets it back. */
export function fitWidth(width: number, windowWidth: number): number {
  return Math.round(Math.max(MIN_WIDTH, Math.min(width, maxWidth(windowWidth))))
}

const keyFor = (path: string) => `gitrevenge.rightPanel:${path}`

type Store = Pick<Storage, 'getItem' | 'setItem'>

/** The saved state for a repository. Anything missing or unreadable uses its default. */
export function loadRightPanel(path: string, store: Store): RightPanelState {
  const state: RightPanelState = { width: DEFAULT_WIDTH }
  let saved: Partial<RightPanelState> | null
  try {
    saved = JSON.parse(store.getItem(keyFor(path)) ?? 'null')
  } catch {
    return state
  }
  if (typeof saved !== 'object' || saved === null) return state
  if (typeof saved.width === 'number' && Number.isFinite(saved.width)) state.width = Math.max(MIN_WIDTH, Math.round(saved.width))
  return state
}

/** Saves the state. It's only a convenience, so a full or blocked store is ignored. */
export function saveRightPanel(path: string, state: RightPanelState, store: Store): void {
  try {
    store.setItem(keyFor(path), JSON.stringify(state))
  } catch {
    // Nothing to do: the panel still works, and just won't remember this.
  }
}

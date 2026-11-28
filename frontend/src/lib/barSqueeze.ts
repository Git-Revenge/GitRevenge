/**
 * How far the repository's bar has given way to stay on one line. Each step keeps the ones before
 * it, in the order things are least missed:
 *
 * 1. the commit count leaves the bar for the search field's placeholder, and the view menu's
 *    button loses its word;
 * 2. the search field becomes an icon that opens it over the bar;
 * 3. Undo and Redo lose their words;
 * 4. Fetch, Pull and Push lose theirs, and so does Kinds;
 * 5. in a panel narrower than a phone, the view menu goes: Settings and the command palette
 *    still switch the rows' density.
 */
export const SQUEEZE = { count: 1, search: 2, undo: 3, network: 4, density: 5 } as const

/** The bar's widths, in px, under which each step applies when the graph's tools share the bar
 * with the repository's actions. Measured with the app's fonts, with a little to spare. */
export const SHARED_STEPS = [1110, 920, 840, 720, 440]

/** The same for a graph whose tools have a bar to themselves: only the first two steps apply. */
export const ALONE_STEPS = [580, 380]

/** How many steps a bar this wide has taken. A width of 0 is a bar not measured yet: no steps. */
export function squeezeFor(width: number, steps: number[]): number {
  if (width <= 0) return 0
  return steps.filter((s) => width < s).length
}

/** The focus chip's width, in px, once the search is an icon and the chip's name has gone with
 * it: the focus icon and ×. Measured with the app's fonts. */
export const FOCUS_ICON_W = 56

/** The most a focus chip's name takes before it's cut, as the chip's style has it. */
export const FOCUS_NAME_MAX = 260

/** The chip's whole width with a name that takes nameW px: the icon, the name and × (#1072). */
export const focusChipWidth = (nameW: number) => FOCUS_ICON_W + 5 + Math.min(FOCUS_NAME_MAX, Math.ceil(nameW))

/**
 * How many steps a bar this wide has taken with the focus chip on it (#1072), which the steps'
 * widths leave out. chip is the chip's whole width, 0 with no focus. The chip loses its name
 * once the search is an icon, so from that step on it only takes FOCUS_ICON_W, and the steps
 * are worked out again with that: never fewer than the search's step, so the chip's size never
 * decides the step that decides its size.
 */
export function squeezeWithChip(width: number, steps: number[], chip: number): number {
  if (width <= 0 || chip <= 0) return squeezeFor(width, steps)
  const full = squeezeFor(Math.max(1, width - chip), steps)
  if (full < SQUEEZE.search) return full
  return Math.max(Math.min(SQUEEZE.search, steps.length), squeezeFor(Math.max(1, width - FOCUS_ICON_W), steps))
}

/** What the search field says when empty: what it searches, or, once the count has left the bar,
 * how many commits that is. */
export function searchPlaceholder(level: number, commits: number, more: boolean, since = ''): string {
  if (level < SQUEEZE.count) return 'Search message, author or SHA'
  // A shallow clone's count is of the history it has (#1006).
  return `Search ${commits.toLocaleString()}${more ? '+' : ''} ${commits === 1 && !more ? 'commit' : 'commits'}${since ? ` since ${since}` : ''}`
}

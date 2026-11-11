/**
 * How far the repository's bar has given way to stay on one line. Each step keeps the ones before
 * it, in the order things are least missed:
 *
 * 1. the commit count leaves the bar for the search field's placeholder;
 * 2. the search field becomes an icon that opens it over the bar;
 * 3. Undo and Redo lose their words;
 * 4. Fetch, Pull and Push lose theirs, and so does Kinds;
 * 5. in a panel narrower than a phone, the density switch goes: Settings and the command
 *    palette still have it.
 */
export const SQUEEZE = { count: 1, search: 2, undo: 3, network: 4, density: 5 } as const

/** The bar's widths, in px, under which each step applies when the graph's tools share the bar
 * with the repository's actions. Measured with the app's fonts, with a little to spare. */
export const SHARED_STEPS = [1060, 920, 840, 720, 440]

/** The same for a graph whose tools have a bar to themselves: only the first two steps apply. */
export const ALONE_STEPS = [530, 380]

/** How many steps a bar this wide has taken. A width of 0 is a bar not measured yet: no steps. */
export function squeezeFor(width: number, steps: number[]): number {
  if (width <= 0) return 0
  return steps.filter((s) => width < s).length
}

/** What the search field says when empty: what it searches, or, once the count has left the bar,
 * how many commits that is. */
export function searchPlaceholder(level: number, commits: number, more: boolean): string {
  if (level < SQUEEZE.count) return 'Search message, author or SHA'
  return `Search ${commits.toLocaleString()}${more ? '+' : ''} ${commits === 1 && !more ? 'commit' : 'commits'}`
}

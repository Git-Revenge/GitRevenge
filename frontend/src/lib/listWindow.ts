// Windowing for long lists: only the rows in view are in the DOM. The sidebar's rows are all
// one height (see visibleRange in sidebarState); a file list's folder headings are shorter than
// its files, so these work from where each row starts.

/** The rows to draw, from start up to but not including end. */
export interface RowRange {
  start: number
  end: number
}

/** Where each row starts, given each row's height. One entry more than there are rows: the
 * last is where the list ends, which is its full height. */
export function rowTops<R>(rows: readonly R[], heightOf: (row: R) => number): number[] {
  const tops = new Array<number>(rows.length + 1)
  let top = 0
  for (let i = 0; i < rows.length; i++) {
    tops[i] = top
    top += heightOf(rows[i])
  }
  tops[rows.length] = top
  return tops
}

/** How many rows start above y, or at it if asked. */
function startingBy(tops: readonly number[], y: number, orAt: boolean): number {
  let low = 0
  let high = tops.length - 1
  while (low < high) {
    const mid = (low + high) >> 1
    if (tops[mid] < y || (orAt && tops[mid] === y)) low = mid + 1
    else high = mid
  }
  return low
}

/** Which rows to draw for a scroll position: enough to fill the view, plus some either side.
 * The position is measured from the list's own top, so it's negative while the list is still
 * below the top of the view, and past the end once the list has scrolled away above it. */
export function visibleRows(tops: readonly number[], scrollTop: number, viewHeight: number, overscan = 12): RowRange {
  const total = tops.length - 1
  const start = Math.max(0, startingBy(tops, scrollTop, true) - 1 - overscan)
  const end = Math.min(total, startingBy(tops, scrollTop + viewHeight, false) + overscan)
  return { start: Math.min(start, end), end }
}

/** The rows to draw, in order: those in range, and one more that has to stay in the DOM
 * wherever it is, such as the row with focus. */
export function drawnRows({ start, end }: RowRange, keep = -1): number[] {
  const drawn: number[] = []
  if (keep >= 0 && keep < start) drawn.push(keep)
  for (let i = start; i < end; i++) drawn.push(i)
  if (keep >= end) drawn.push(keep)
  return drawn
}

/** The row an arrow key, Home or End moves to from this one, skipping rows that don't take
 * focus. The same row when there's nowhere further to go, and null for any other key. */
export function moveTarget<R>(key: string, rows: readonly R[], from: number, takesFocus: (row: R) => boolean): number | null {
  const step = key === 'ArrowDown' || key === 'Home' ? 1 : key === 'ArrowUp' || key === 'End' ? -1 : 0
  if (!step) return null
  let to = key === 'Home' ? 0 : key === 'End' ? rows.length - 1 : from + step
  while (to >= 0 && to < rows.length && !takesFocus(rows[to])) to += step
  return to >= 0 && to < rows.length ? to : from
}

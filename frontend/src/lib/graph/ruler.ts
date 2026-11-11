/**
 * The ruler down the graph's right edge: the scrollbar, with the history marked on it. Its
 * length stands for every row there is (the estimate, while a long history is still being read);
 * a box shows what's in view, and ticks show where the checked-out commit, branches, tags, notes
 * and search matches are among the rows read so far.
 */

/** How wide the ruler is. */
export const RULER_W = 14

/** The box is never shorter than this, so it can always be grabbed. */
export const MIN_BOX = 24

/** Ticks nearer than this, in px, are drawn as one. */
export const MERGE_PX = 2

/** Where things are, as rows of the ruler (the graph's slots). A row of -1 is nowhere. */
export interface RulerRows {
  head: number
  tips: readonly number[]
  tags: readonly number[]
  notes: readonly number[]
  matches: readonly number[]
}

/** One tick: everything at about one height. */
export interface Tick {
  y: number
  head: boolean
  tips: number
  tags: number
  notes: number
  matches: number
}

/** How far down a ruler `height` tall a row is, of `count` rows. */
export function rowToY(row: number, count: number, height: number): number {
  return count > 0 ? (Math.max(0, Math.min(count, row)) / count) * height : 0
}

/** The ticks to draw, top to bottom: each thing at its row's height, with those nearer than
 * MERGE_PX to the first of a group drawn as one tick that counts them. */
export function placeTicks(rows: RulerRows, count: number, height: number): Tick[] {
  const all: { y: number; kind: 'head' | 'tips' | 'tags' | 'notes' | 'matches' }[] = []
  const add = (kind: 'tips' | 'tags' | 'notes' | 'matches', list: readonly number[]) => {
    for (const row of list) if (row >= 0 && row < count) all.push({ y: rowToY(row, count, height), kind })
  }
  if (rows.head >= 0 && rows.head < count) all.push({ y: rowToY(rows.head, count, height), kind: 'head' })
  add('tips', rows.tips)
  add('tags', rows.tags)
  add('notes', rows.notes)
  add('matches', rows.matches)
  all.sort((a, b) => a.y - b.y)
  const ticks: Tick[] = []
  for (const one of all) {
    let tick = ticks[ticks.length - 1]
    if (!tick || one.y - tick.y >= MERGE_PX) {
      tick = { y: one.y, head: false, tips: 0, tags: 0, notes: 0, matches: 0 }
      ticks.push(tick)
    }
    if (one.kind === 'head') tick.head = true
    else tick[one.kind]++
  }
  return ticks
}

const count = (n: number, one: string, many: string) => `${n.toLocaleString()} ${n === 1 ? one : many}`

/** What a tick is, in words: "the checked-out commit, 3 tags, 1 note". */
export function tickSummary(t: Tick): string {
  const parts: string[] = []
  if (t.head) parts.push('the checked-out commit')
  if (t.tips) parts.push(count(t.tips, 'branch', 'branches'))
  if (t.tags) parts.push(count(t.tags, 'tag', 'tags'))
  if (t.notes) parts.push(count(t.notes, 'note', 'notes'))
  if (t.matches) parts.push(count(t.matches, 'match', 'matches'))
  return parts.join(', ')
}

/** The tick nearest a height, within `reach` px of it. */
export function tickAt(ticks: readonly Tick[], y: number, reach = 3): Tick | undefined {
  let best: Tick | undefined
  for (const t of ticks) if (Math.abs(t.y - y) <= reach && (!best || Math.abs(t.y - y) < Math.abs(best.y - y))) best = t
  return best
}

/** The box that shows what's in view: `position` is the view's top edge in rows, `visible` how
 * many rows show. It's as tall as its share of the rows, but never shorter than MIN_BOX; either
 * way it reaches the bottom of the ruler exactly when the view reaches the end. */
export function boxFor(position: number, visible: number, count: number, height: number): { top: number; height: number } {
  if (count <= 0 || height <= 0) return { top: 0, height: 0 }
  const h = Math.min(height, Math.max(MIN_BOX, (Math.min(visible, count) / count) * height))
  const travel = count - Math.min(visible, count)
  const top = travel > 0 ? (Math.max(0, Math.min(travel, position)) / travel) * (height - h) : 0
  return { top, height: h }
}

/** The view's top edge, in rows, for a box whose top edge is at `top`: boxFor backwards. */
export function positionFor(top: number, visible: number, count: number, height: number): number {
  const box = boxFor(0, visible, count, height)
  const travel = count - Math.min(visible, count)
  const room = height - box.height
  return room > 0 ? (Math.max(0, Math.min(room, top)) / room) * travel : 0
}

/** A month's first row. */
export interface MonthMark {
  row: number
  time: number
}

/** When the commit at a row was made, to the month: the time of the last month mark at or
 * above it. Null before the first mark. */
export function monthAt(months: readonly MonthMark[], row: number): number | null {
  let lo = 0
  let hi = months.length - 1
  let found = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (months[mid].row <= row) {
      found = mid
      lo = mid + 1
    } else hi = mid - 1
  }
  return found < 0 ? null : months[found].time
}

const monthName = new Intl.DateTimeFormat(undefined, { month: 'short', year: 'numeric' })

/** A month as the ruler names it while it's dragged: "Nov 2026". */
export function monthLabel(unixSeconds: number): string {
  return monthName.format(new Date(unixSeconds * 1000))
}

/** Where the rows that haven't been read begin on the ruler, or null when all have been. */
export function unreadFrom(loaded: number, count: number, height: number): number | null {
  return loaded < count ? rowToY(loaded, count, height) : null
}

/** How far through the history the view is, as a scrollbar says it: 0 to 100. */
export function percentThrough(position: number, visible: number, count: number): number {
  const travel = count - Math.min(visible, count)
  return travel > 0 ? Math.round((Math.max(0, Math.min(travel, position)) / travel) * 100) : 0
}

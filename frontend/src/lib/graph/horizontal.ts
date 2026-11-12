// The Horizontal view's geometry: the timeline turned a quarter. Each commit is a column, lanes
// are rows, and the newest commit is on the left, where the top of the timeline would be.
import { sameDay } from './draw'
import type { Row, Seg } from './rowCache'

/** A commit's column, in px. */
export const COL = 44
/** The day band across the top. */
export const DAY_BAND = 26
/** The band the subjects lean in, at its tallest and shortest. */
export const WORDS = { max: 200, min: 64 }
/** The row of branch and tag labels, between the subjects and the lanes. */
export const LABELS_H = 28
/** A lane's height, at its roomiest and tightest, and the space above the first lane. */
export const LANE = { max: 26, min: 12, pad: 20 }
/** How far the subjects lean back from the horizontal, in degrees. */
export const LEAN = 40
/** How many columns to the right a leaning subject reaches, so the columns just off the left
 * edge are still drawn. */
export const LEAN_REACH = 6
/** The scrollbar's height, and the shortest its thumb gets. */
export const BAR_H = 14
export const THUMB_MIN = 28

/** The x of column i's centre, from the start of the history. */
export function colX(i: number): number {
  return i * COL + COL / 2
}

/** The furthest the view scrolls: the last column against the right edge. */
export function maxOffset(width: number, count: number): number {
  return Math.max(0, count * COL - width)
}

export function clampOffset(offset: number, width: number, count: number): number {
  return Math.min(maxOffset(width, count), Math.max(0, offset))
}

/** The columns to draw, first to last (exclusive): what's in view, the few to the left whose
 * subjects lean into it, and one to the right. */
export function visibleRange(offset: number, width: number, count: number): { first: number; last: number } {
  const first = Math.max(0, Math.floor(offset / COL) - LEAN_REACH)
  const last = Math.min(count, Math.ceil((offset + width) / COL) + 1)
  return { first, last: Math.max(first, last) }
}

/** The column under a point x px from the view's left edge, or -1 past the end. */
export function colAt(x: number, offset: number, count: number): number {
  const i = Math.floor((x + offset) / COL)
  return i >= 0 && i < count ? i : -1
}

/** Where to scroll so column i is in view with a column to spare either side; the offset as it
 * is when the column already shows. */
export function offsetToShow(offset: number, width: number, i: number, count: number): number {
  const left = i * COL - COL
  const right = (i + 2) * COL
  if (left < offset) return clampOffset(left, width, count)
  if (right > offset + width) return clampOffset(right - width, width, count)
  return clampOffset(offset, width, count)
}

/** How tall the subjects' band is in a view this tall with this many lanes: the lanes come
 * first, and the words take what's left, within their limits. */
export function wordsHeight(height: number, lanes: number): number {
  const lanesWant = LANE.pad * 2 + lanes * LANE.max
  return Math.min(WORDS.max, Math.max(WORDS.min, height - DAY_BAND - LABELS_H - lanesWant))
}

/** How long a leaning subject can be before it leaves the top of its band. */
export function subjectLength(words: number): number {
  return Math.max(40, Math.floor((words - 12) / Math.sin((LEAN * Math.PI) / 180)))
}

/** A lane's height when this many share this much room. */
export function lanePitch(lanes: number, height: number): number {
  const room = (height - LANE.pad * 2) / Math.max(1, lanes)
  return Math.min(LANE.max, Math.max(LANE.min, Math.floor(room)))
}

/** The y of a lane's line, from the top of the lanes. */
export function laneY(col: number, pitch: number): number {
  return LANE.pad + col * pitch
}

const ELBOW = 8

/**
 * The path of one of a row's lines, for a commit whose column is centred on cx. An `up` line
 * comes from the newer commit's side (the left edge of the column) to the middle; a `down`
 * line leaves the middle for the older side. A line that changes lane runs along its lane to
 * the column's middle and turns there, so lines join up from one column to the next.
 */
export function segPath(seg: Pick<Seg, 'from' | 'to'>, cx: number, side: 'up' | 'down', pitch: number): string {
  const from = laneY(seg.from, pitch)
  const to = laneY(seg.to, pitch)
  const edge = side === 'up' ? cx - COL / 2 : cx + COL / 2
  if (seg.from === seg.to) return side === 'up' ? `M${edge} ${from}H${cx}` : `M${cx} ${from}H${edge}`
  const dir = to > from ? 1 : -1
  const r = Math.min(ELBOW, Math.abs(to - from), COL / 2)
  if (side === 'up') return `M${edge} ${from}H${cx - r}Q${cx} ${from} ${cx} ${from + dir * r}V${to}`
  return `M${cx} ${from}V${to - dir * r}Q${cx} ${to} ${cx + r} ${to}H${edge}`
}

/** A run of columns made on one day: `start` to `end` (exclusive), and when its first was made. */
export interface DaySpan {
  start: number
  end: number
  time: number
}

/** The days among rows[0..], whose first row is commit `first`. Rows not read yet end a day:
 * what day they belong to isn't known. */
export function daySpans(rows: (Pick<Row, 'authorTime'> | undefined)[], first: number): DaySpan[] {
  const out: DaySpan[] = []
  let open: DaySpan | null = null
  rows.forEach((row, k) => {
    const i = first + k
    if (!row) {
      open = null
      return
    }
    if (open && sameDay(open.time, row.authorTime)) open.end = i + 1
    else {
      open = { start: i, end: i + 1, time: row.authorTime }
      out.push(open)
    }
  })
  return out
}

/** How many commits a day has, counting outwards from one of them through the rows read so
 * far, or null when the day runs into rows not read yet (or goes on past `limit`). */
export function dayCount(get: (i: number) => Pick<Row, 'authorTime'> | undefined, from: number, count: number, limit = 600): number | null {
  const day = get(from)
  if (!day) return null
  let n = 1
  for (const dir of [-1, 1]) {
    for (let i = from + dir; i >= 0 && i < count; i += dir) {
      const row = get(i)
      if (!row) return null
      if (!sameDay(row.authorTime, day.authorTime)) break
      if (++n > limit) return null
    }
  }
  return n
}

/** The scrollbar's thumb in a track this long: where it starts and how long it is. */
export function thumbFor(offset: number, width: number, count: number, track: number): { left: number; width: number } {
  const all = count * COL
  if (all <= width || track <= 0) return { left: 0, width: Math.max(0, track) }
  const w = Math.min(track, Math.max(THUMB_MIN, (width / all) * track))
  return { left: (offset / (all - width)) * (track - w), width: w }
}

/** The offset that puts the thumb's start at `left`. */
export function offsetForThumb(left: number, width: number, count: number, track: number): number {
  const { width: w } = thumbFor(0, width, count, track)
  if (track - w <= 0) return 0
  return clampOffset((left / (track - w)) * maxOffset(width, count), width, count)
}

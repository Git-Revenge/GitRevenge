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
/** The row of tags, between the subjects and the lanes, and the widest a chip in it gets. */
export const TAGS_H = 28
export const LABEL_MAX = 220
/** A tag with less room than this before the next one is a mark alone, named on hover. */
export const CHIP_MIN = 56
/** The pinned column of lane names, at its widest, and the share of a narrow view it takes. */
export const NAMES = { max: 184, share: 0.34 }
/** A lane's height, and the space above the first lane and below the last. Lanes keep their
 * height however many there are: past what fits, they scroll. */
export const LANE = { max: 26, pad: 20 }
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

/** How tall the subjects' band is in a view this tall with this many lanes: the lanes come
 * first, and the words take what's left, within their limits. */
export function wordsHeight(height: number, lanes: number): number {
  // A history with many lanes scrolls them: the words don't give up everything for it.
  const lanesWant = LANE.pad * 2 + Math.min(lanes, 6) * LANE.max
  return Math.min(WORDS.max, Math.max(WORDS.min, height - DAY_BAND - TAGS_H - lanesWant))
}

/** How long a leaning subject can be before it leaves the top of its band. */
export function subjectLength(words: number): number {
  return Math.max(40, Math.floor((words - 12) / Math.sin((LEAN * Math.PI) / 180)))
}

/** How tall the lanes are, all of them. */
export function lanesExtent(lanes: number): number {
  return LANE.pad * 2 + Math.max(0, lanes - 1) * LANE.max
}

/** How far down the lanes can scroll in a view this tall. */
export function maxLaneScroll(lanes: number, view: number): number {
  return Math.max(0, lanesExtent(lanes) - view)
}

/** Where to scroll the lanes so this one shows, with its neighbours' worth of room; the scroll
 * as it is when it already does. */
export function laneScrollToShow(scroll: number, lane: number, lanes: number, view: number): number {
  const y = laneY(lane, LANE.max)
  let next = scroll
  if (y - LANE.max < next) next = y - LANE.max
  else if (y + LANE.max > next + view) next = y + LANE.max - view
  return Math.min(maxLaneScroll(lanes, view), Math.max(0, next))
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

/** The lanes that run through a commit's column: the one its node is on, and those passing
 * straight by. Lane number to the lane's colour number, which names the lane. */
export function lanesAt(row: Pick<Row, 'col' | 'color' | 'up' | 'down'>): Map<number, number> {
  const out = new Map<number, number>()
  for (const s of [...row.up, ...row.down]) if (s.from === s.to) out.set(s.from, s.color)
  out.set(row.col, row.color)
  return out
}

/** A lane as the pinned column names it: its colour, and whether it only starts further right
 * in the view, with nothing on its row at the left edge. */
export interface LaneAtEdge {
  color: number
  later: boolean
}

/**
 * What the pinned column says for each lane row, from the rows in view, left edge first: the
 * lane on that row at the left edge, or, for a row that's empty there, the first lane to turn
 * up on it further right.
 */
export function lanesAtEdge(rows: (Pick<Row, 'col' | 'color' | 'up' | 'down'> | undefined)[]): Map<number, LaneAtEdge> {
  const out = new Map<number, LaneAtEdge>()
  let edge = true
  for (const row of rows) {
    if (!row) continue
    for (const [lane, color] of lanesAt(row)) if (!out.has(lane)) out.set(lane, { color, later: !edge })
    edge = false
  }
  return out
}

/** How wide each chip in the tags row can be, given the columns that have one, in order: up
 * to the next chip. */
export function chipRooms(columns: number[]): Map<number, number> {
  const out = new Map<number, number>()
  columns.forEach((c, k) => out.set(c, k + 1 < columns.length ? Math.min(LABEL_MAX, (columns[k + 1] - c) * COL - 6) : LABEL_MAX))
  return out
}

/** The pinned column's width in a view this wide. */
export function namesWidth(width: number): number {
  return Math.round(Math.min(NAMES.max, Math.max(96, width * NAMES.share)))
}

const monthDay = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
const dayOnly = new Intl.DateTimeFormat(undefined, { day: 'numeric' })
/** About how wide the day band's letters and its count's are, and the band's padding. */
const BAND = { letter: 8.4, digit: 6.6, pad: 20, gap: 8 }

/**
 * What a day says in a band this wide: its full name and its count when both fit, then the
 * name alone, then the month and day ("Nov 3"), then just the day ("3"). A name is never cut
 * part way: a one-commit day is 44px, which holds two digits.
 */
export function dayBandText(full: string, count: string, time: number, width: number): { label: string; count: string } {
  const room = width - BAND.pad
  const wide = (text: string) => text.length * BAND.letter
  if (count && wide(full) + BAND.gap + count.length * BAND.digit <= room) return { label: full, count }
  if (wide(full) <= room) return { label: full, count: '' }
  const date = new Date(time * 1000)
  const short = monthDay.format(date)
  return { label: wide(short) <= room ? short : dayOnly.format(date), count: '' }
}

/**
 * What a merge says it merged, from its subject, for naming the lane it opened: the branch or
 * tag in git's own messages ("Merge branch 'x'", "Merge tag 'v1' of ..."), or the branch of a
 * pull request ("Merge pull request #12 from someone/topic" is "topic #12"). '' when the
 * subject doesn't say.
 */
export function mergedName(subject: string): string {
  const quoted = /^Merge (?:remote-tracking )?(?:branch|branches|tag|tags|commit) '([^']+)'/.exec(subject)
  if (quoted) return quoted[1]
  const pull = /^Merge pull request #(\d+) from (\S+)/.exec(subject)
  if (pull) return `${pull[2].includes('/') ? pull[2].slice(pull[2].indexOf('/') + 1) : pull[2]} #${pull[1]}`
  // GitLab's and Bitbucket's own words, and a plain "Merge x into y".
  const request = /^Merged? (?:in )?(\S+) (?:into|\(pull request)/.exec(subject)
  if (request) return request[1].replace(/^'|'$/g, '')
  return ''
}

// Columns that aren't all there. A folded lane's commits of a day show as one column, and the
// rest take no room, so a commit's place can't be worked out from its number. The view is
// placed by an anchor instead, as the timeline's rows are: the column at its left edge and how
// far into it the view has scrolled. Everything else is found by walking from there.

/** Where the view's left edge is: `dx` px into column `i`. A lead column (the working tree)
 * comes before commit 0, as -1. */
export interface Anchor {
  i: number
  dx: number
}

/** A column as placed in the view: commit `i` (or a lead column, below 0) with its left edge
 * at `x`, from the view's. */
export interface Spot {
  i: number
  x: number
}

/** How many columns a walk will step over before giving up on finding one that shows. */
const SKIP_LIMIT = 20000

/** The next column that shows after `i` in direction `dir`, or null at the end. Lead columns
 * always show. */
export function nextShown(i: number, dir: 1 | -1, count: number, shown: (i: number) => boolean, lead = 0): number | null {
  for (let j = i + dir, n = 0; j >= -lead && j < count && n < SKIP_LIMIT; j += dir, n++) if (j < 0 || shown(j)) return j
  return null
}

/** An anchor that names a real column: one that exists and shows, with an offset inside it. */
export function settle(a: Anchor, count: number, shown: (i: number) => boolean, lead = 0): Anchor {
  if (count + lead <= 0) return { i: -lead, dx: 0 }
  let i = Math.min(count - 1, Math.max(-lead, Math.round(a.i)))
  if (i >= 0 && !shown(i)) i = nextShown(i, 1, count, shown, lead) ?? nextShown(i, -1, count, shown, lead) ?? i
  return { i, dx: i === a.i ? Math.min(COL - 0.001, Math.max(0, a.dx)) : 0 }
}

/** The anchor `dx` px further right (or left, when negative), stopping at either end. */
export function moveAnchor(a: Anchor, dx: number, count: number, shown: (i: number) => boolean, lead = 0): Anchor {
  let { i, dx: off } = settle(a, count, shown, lead)
  off += dx
  while (off >= COL) {
    const next = nextShown(i, 1, count, shown, lead)
    if (next === null) return { i, dx: 0 }
    i = next
    off -= COL
  }
  while (off < 0) {
    const prev = nextShown(i, -1, count, shown, lead)
    if (prev === null) return { i, dx: 0 }
    i = prev
    off += COL
  }
  return { i, dx: off }
}

/** The columns to draw in a view `width` wide: those in it, `reach` to the left whose subjects
 * lean into it, and one past the right edge. */
export function placeColumns(a: Anchor, width: number, count: number, shown: (i: number) => boolean, lead = 0, reach = LEAN_REACH): Spot[] {
  const at = settle(a, count, shown, lead)
  const out: Spot[] = []
  let x = 0 - at.dx
  for (let i: number | null = at.i; i !== null && x < width + COL; i = nextShown(i, 1, count, shown, lead)) {
    out.push({ i, x })
    x += COL
  }
  let bx = 0 - at.dx
  let j = nextShown(at.i, -1, count, shown, lead)
  for (let k = 0; j !== null && k < reach; k++) {
    bx -= COL
    out.unshift({ i: j, x: bx })
    j = nextShown(j, -1, count, shown, lead)
  }
  return out
}

/** The anchor that brings column `target` into view with a column to spare either side; the
 * anchor as it is when it already shows so. `spots` is what's placed now. */
export function anchorToShow(a: Anchor, spots: readonly Spot[], width: number, target: number, count: number, shown: (i: number) => boolean, lead = 0): Anchor {
  const spot = spots.find((s) => s.i === target)
  if (spot) {
    if (spot.x < COL && nextShown(target, -1, count, shown, lead) !== null) return moveAnchor(a, spot.x - COL, count, shown, lead)
    if (spot.x < 0) return moveAnchor(a, spot.x, count, shown, lead)
    if (spot.x + 2 * COL > width) return moveAnchor(a, spot.x + 2 * COL - width, count, shown, lead)
    return a
  }
  // Somewhere else altogether: one column in from the left edge.
  return { i: nextShown(target, -1, count, shown, lead) ?? target, dx: 0 }
}

/** The anchor as a distance in px along every column there is, shown or not: what the
 * scrollbar's thumb is placed by, since nothing knows how many columns are folded away. */
export function anchorOffset(a: Anchor, lead = 0): number {
  return (a.i + lead) * COL + a.dx
}

/** And back: the anchor for such a distance. */
export function anchorAt(offset: number, count: number, shown: (i: number) => boolean, lead = 0): Anchor {
  const k = Math.floor(Math.max(0, offset) / COL)
  return settle({ i: k - lead, dx: Math.max(0, offset) - k * COL }, count, shown, lead)
}

/** The placed column under a point `x` px from the view's left edge, or undefined. */
export function spotAt(spots: readonly Spot[], x: number): Spot | undefined {
  return spots.find((s) => x >= s.x && x < s.x + COL)
}

/** The runs of placed commits made on one day, as stretches of the view: where each starts
 * and how wide it is. `time(i)` is when commit i was made, or undefined when it isn't read. */
export function dayStretches(spots: readonly Spot[], time: (i: number) => number | undefined): { first: number; x: number; width: number; time: number }[] {
  const out: { first: number; x: number; width: number; time: number }[] = []
  let open: { first: number; x: number; width: number; time: number } | null = null
  for (const s of spots) {
    const t = s.i < 0 ? undefined : time(s.i)
    if (t === undefined) {
      open = null
      continue
    }
    if (open && sameDay(open.time, t)) open.width += COL
    else {
      open = { first: s.i, x: s.x, width: COL, time: t }
      out.push(open)
    }
  }
  return out
}

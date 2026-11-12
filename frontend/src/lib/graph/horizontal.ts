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

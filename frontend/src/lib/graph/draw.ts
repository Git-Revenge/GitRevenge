import { kindInfo, type KindInfo } from '../branchKinds'
import { PATHS, type IconName } from '../ui/icons'
import type { Ref, Row, Seg } from './rowCache'

/** Row density: the two-line timeline, or single-line rows that fit twice as many commits. */
export type Density = 'comfortable' | 'compact'

export interface Metrics {
  rowH: number
  laneW: number
  /** Padding either side of the lanes. */
  pad: number
}

export const METRICS: Record<Density, Metrics> = {
  comfortable: { rowH: 52, laneW: 16, pad: 10 },
  compact: { rowH: 30, laneW: 11, pad: 8 },
}

/** Lane colours are the theme's --lane-0 … --lane-5 tokens, cycled by colour index. */
export const LANE_COUNT = 6

// Resolved from the tokens by setLaneColors, since a canvas can't read CSS variables.
let palette: string[] = Array.from({ length: LANE_COUNT }, () => '#888')

/** Sets the lane colours the canvas paints with, resolved from the current theme. */
export function setLaneColors(colors: string[]) {
  palette = colors
}

function laneIndex(i: number): number {
  return ((i % LANE_COUNT) + LANE_COUNT) % LANE_COUNT
}

/** Lane colour i as a concrete colour, for the canvas. */
export function laneColor(i: number): string {
  return palette[laneIndex(i)]
}

/** Lane colour i as a CSS value, for DOM elements; it follows theme changes by itself. */
export function laneVar(i: number): string {
  return `var(--lane-${laneIndex(i)})`
}

/** X coordinate of the center of lane column col. */
export function laneX(col: number, m: Metrics): number {
  return m.pad + col * m.laneW + m.laneW / 2
}

/**
 * The first lane to show when the graph is wider than its column: 0 when every node in view
 * fits; otherwise far enough right that they do, or, when they can't all fit, that the
 * selected node does (else the leftmost node in view).
 */
export function laneWindow(minCol: number, maxCol: number, selectedCol: number | null, capacity: number): number {
  if (capacity <= 0 || maxCol < capacity) return 0
  if (maxCol - minCol < capacity) return Math.max(0, maxCol - capacity + 1)
  const anchor = selectedCol ?? minCol
  return Math.max(0, anchor - Math.floor(capacity / 2))
}

/** How many lanes fit in a graph column graphW pixels wide. */
export function laneCapacity(graphW: number, m: Metrics): number {
  return Math.max(1, Math.floor((graphW - m.pad * 2) / m.laneW))
}

/** Width in pixels of the graph column for rows using up to `lanes` columns. */
export function graphWidth(lanes: number, max: number, m: Metrics): number {
  return Math.min(max, m.pad * 2 + Math.max(1, lanes) * m.laneW)
}

const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

export function formatTime(unixSeconds: number): string {
  return dateFmt.format(new Date(unixSeconds * 1000))
}

const shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
const shortDateYear = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' })

/** "now", "14m ago", "5h ago", "3d ago", then a short date ("Oct 26", with the year once
 * it isn't this year's). */
export function relativeTime(unixSeconds: number, nowSeconds: number): string {
  const s = nowSeconds - unixSeconds
  if (s < 60) return 'now'
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  if (s < 7 * 86400) return `${Math.floor(s / 86400)}d ago`
  const d = new Date(unixSeconds * 1000)
  return (d.getFullYear() === new Date(nowSeconds * 1000).getFullYear() ? shortDate : shortDateYear).format(d)
}

const dayFmt = new Intl.DateTimeFormat(undefined, { weekday: 'short', month: 'short', day: 'numeric' })
const dayFmtYear = new Intl.DateTimeFormat(undefined, { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' })
const timeFmt = new Intl.DateTimeFormat(undefined, { timeStyle: 'short' })

/** Whether two moments fall on the same local calendar day. */
export function sameDay(a: number, b: number): boolean {
  const da = new Date(a * 1000)
  const db = new Date(b * 1000)
  return da.getFullYear() === db.getFullYear() && da.getMonth() === db.getMonth() && da.getDate() === db.getDate()
}

/** The day a commit belongs to, as the timeline names it: "Today", "Yesterday", then "Mon, Oct 26"
 * (with the year once it isn't this year's). */
export function dayLabel(unixSeconds: number, nowSeconds: number): string {
  if (sameDay(unixSeconds, nowSeconds)) return 'Today'
  if (sameDay(unixSeconds, nowSeconds - 86400)) return 'Yesterday'
  const d = new Date(unixSeconds * 1000)
  return (d.getFullYear() === new Date(nowSeconds * 1000).getFullYear() ? dayFmt : dayFmtYear).format(d)
}

/** A commit's time of day, in the local short form. */
export function timeLabel(unixSeconds: number): string {
  return timeFmt.format(new Date(unixSeconds * 1000))
}

/** What the second line says under a commit that starts a run, given its neighbours. */
export interface MetaLine {
  text: string
  /** The row starts a new day after a loaded row, so it draws the day's hairline. */
  divider: boolean
}

/** The second line of a timeline row, which only a commit that starts a run has (see
 * `startsRun`): who and when. The first row of a day names the day too. A row whose previous
 * row isn't loaded yet names its day, to be safe, but draws no divider. */
export function metaLine(row: Pick<Row, 'author' | 'authorTime'>, near: Near, nowSeconds: number): MetaLine {
  const prev = near.prev
  const newDay = !prev || !sameDay(prev.authorTime, row.authorTime)
  const time = timeLabel(row.authorTime)
  if (newDay) return { text: `${dayLabel(row.authorTime, nowSeconds)} · ${row.author} · ${time}`, divider: !!prev }
  return { text: `${row.author} · ${time}`, divider: false }
}

/** How tall a timeline row is for a commit that carries on a run: one line. */
export const FOLLOWER_H = 30

/** How long a pause between an author's commits ends their run, in seconds: a sitting is over
 * after two hours. Fixed, not a setting. */
export const RUN_PAUSE = 2 * 60 * 60

/**
 * Whether a commit starts a run in the timeline, and so takes two lines: its author's initials
 * and who and when. A commit that carries a run on takes one line. It starts one when:
 *
 * - it's the checked-out commit, so "you're here" is never a thin row;
 * - it's a merge, or the commit above it on its lane is;
 * - it's the first row of its day;
 * - the commit above it on its lane (within two rows) is someone else's, or there isn't one;
 * - that commit is two hours or more away: the same person, another sitting.
 *
 * A row whose neighbours aren't loaded yet starts a run until they are.
 */
export function startsRun(row: Pick<Row, 'author' | 'authorTime' | 'parents' | 'col'>, near: Near, head: boolean): boolean {
  if (head || row.parents > 1) return true
  const above = near.prev
  if (!above || !sameDay(above.authorTime, row.authorTime)) return true
  const before = above.col === row.col ? above : near.prev2?.col === row.col ? near.prev2 : undefined
  if (!before || before.author !== row.author || before.parents > 1) return true
  return Math.abs(before.authorTime - row.authorTime) >= RUN_PAUSE
}

/** Up to two initials for an author's node: "Ris Adams" → "RA", "dependabot[bot]" → "D". */
export function initials(name: string): string {
  const words = name.replace(/\[.*?\]|<.*?>/g, ' ').split(/[\s._-]+/).filter((w) => /\p{L}|\p{N}/u.test(w))
  const letters = words.map((w) => [...w].find((c) => /\p{L}|\p{N}/u.test(c)) ?? '')
  return (letters.length > 1 ? letters[0] + letters[letters.length - 1] : (letters[0] ?? '?')).toUpperCase()
}

export interface Theme {
  bg: string
  fg: string
  soft: string
  muted: string
  /** The smallest labels: the time on a row that carries on a run. */
  faint: string
  deep: string
  selected: string
  hover: string
  /** Tags wear this wash whatever their lane. */
  tag: string
  /** Signature seals: verified, failed, and couldn't verify. */
  good: string
  bad: string
  warn: string
  /** Branch kind colours, keyed by kind (trunk, release, environment, docs, bot). */
  kinds?: Record<string, string>
  /** The rose for the one handwritten note on the canvas. */
  link: string
  /** The teal of a bisect's range and the commit under test. */
  teal: string
  /** The sepia of a note's mark. */
  note: string
}

// Icons drawn on the canvas, built once each from the same paths as the Icon component.
const iconPaths = new Map<IconName, Path2D>()
function iconPath(name: IconName): Path2D | undefined {
  if (typeof Path2D === 'undefined') return undefined
  let p = iconPaths.get(name)
  if (!p) {
    p = new Path2D(PATHS[name])
    iconPaths.set(name, p)
  }
  return p
}

/** How strongly Docs lanes, nodes and messages fade (ADR 0009). */
const MUTED = 0.4

/** A busy lane's line, and an idle lane's: thin and faint, so the commits stand out. */
const LINE_WIDTH = 1.75
const QUIET_WIDTH = 1
const QUIET_ALPHA = 0.3
const HALO_WIDTH = 4.5
const ELBOW_RADIUS = 8

/** How far past a label's right edge drawLabels' returned position is. */
const LABEL_TRAIL = 8

/** The selected row's mark: a bar in the lane's colour at the row's left edge, the same mark as
 * the active view in Insights and Git Config and the ink line under the active tab. */
const SEL_BAR_WIDTH = 3
const SEL_BAR_INSET = 4

/** Where the selection bar goes for a row whose top edge is at y and height is h. */
export function selectionBar(y: number, h: number): { x: number; y: number; w: number; h: number; r: number } {
  return { x: 0, y: y + SEL_BAR_INSET, w: SEL_BAR_WIDTH, h: h - SEL_BAR_INSET * 2, r: 2 }
}

/** How far a row that doesn't match the search fades: to 0.3 of its ink. */
const DIM = 0.7

/** What the compact Author column says for an author carrying on from the row above. */
export const DITTO = '〃'

/** Width a kind icon takes in a ref label, gap included. */
const KIND_ICON_W = 15

/** The kind of a branch label, for ref kinds that can have one. */
function labelKind(r: Ref): (KindInfo & { kind: string }) | undefined {
  if (r.kind !== 'local' && r.kind !== 'remote') return undefined
  const info = kindInfo(r.branchKind)
  return info && r.branchKind ? { ...info, kind: r.branchKind } : undefined
}

/** A commit's signature as the graph shows it; unsigned commits get no seal. */
export type Seal = 'good' | 'bad' | 'warn'

/** The seal for a git signature status ("good", "bad", "unknown", "expired", …, "none"). */
export function sealOf(status: string | undefined): Seal | undefined {
  if (!status || status === 'none') return undefined
  return status === 'good' ? 'good' : status === 'bad' ? 'bad' : 'warn'
}

/** A small seal: a ring with a tick, a cross or a bang, so the mark carries the meaning. */
function drawSeal(ctx: CanvasRenderingContext2D, cx: number, cy: number, seal: Seal, theme: Theme) {
  const color = theme[seal]
  ctx.save()
  ctx.strokeStyle = color
  ctx.lineWidth = 1.5
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.beginPath()
  ctx.arc(cx, cy, 5.5, 0, Math.PI * 2)
  ctx.stroke()
  ctx.beginPath()
  if (seal === 'good') {
    ctx.moveTo(cx - 2.5, cy)
    ctx.lineTo(cx - 0.5, cy + 2)
    ctx.lineTo(cx + 2.7, cy - 2)
  } else if (seal === 'bad') {
    ctx.moveTo(cx - 2, cy - 2)
    ctx.lineTo(cx + 2, cy + 2)
    ctx.moveTo(cx + 2, cy - 2)
    ctx.lineTo(cx - 2, cy + 2)
  } else {
    ctx.moveTo(cx, cy - 2.6)
    ctx.lineTo(cx, cy + 0.6)
    ctx.moveTo(cx, cy + 2.6)
    ctx.lineTo(cx, cy + 2.7)
  }
  ctx.stroke()
  ctx.restore()
}

/** A folded page: this commit has a git note. Its shape says so, not only its colour. */
function drawNoteMark(ctx: CanvasRenderingContext2D, cx: number, cy: number, color: string) {
  ctx.save()
  ctx.strokeStyle = color
  ctx.lineWidth = 1.5
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.beginPath()
  ctx.moveTo(cx - 4.5, cy - 6)
  ctx.lineTo(cx + 4.5, cy - 6)
  ctx.lineTo(cx + 4.5, cy + 2)
  ctx.lineTo(cx + 0.5, cy + 6)
  ctx.lineTo(cx - 4.5, cy + 6)
  ctx.closePath()
  ctx.moveTo(cx + 4.5, cy + 2)
  ctx.lineTo(cx + 0.5, cy + 2)
  ctx.lineTo(cx + 0.5, cy + 6)
  ctx.stroke()
  ctx.restore()
}

export interface Layout {
  width: number
  graphW: number
  /** Lanes scrolled out of view on the left (see laneWindow). */
  firstLane: number
  density: Density
  m: Metrics
  /** The branch and tag column left of the graph; 0 when labels sit inline with the subject. */
  refW: number
  /** Where the message starts. */
  textX: number
  /** Compact's right-hand columns: author, date, sha. */
  authorW: number
  dateW: number
  shaW: number
  /** Where those columns start. */
  authorX: number
  dateX: number
  shaX: number
  /** The repository's main remote (see mainRemote). */
  remote: string
}

/** The branch and tag column is as wide as the labels in view need, within these bounds;
 * `default` is its width before anything has been measured. */
export const LABEL_COLUMN = { min: 120, max: 240, default: 150 }

/** A label column width within its bounds, in whole pixels. */
export function fitLabelColumn(wanted: number): number {
  return Math.round(Math.max(LABEL_COLUMN.min, Math.min(wanted, LABEL_COLUMN.max)))
}

/** The branch and tag column's width, given the width its labels want: labels go back inline
 * when the window can't spare a column. */
export function refColumnWidth(width: number, wanted = LABEL_COLUMN.default): number {
  return width >= 640 ? fitLabelColumn(wanted) : 0
}

/** The margins around the labels in their column: where they start, and what stays clear after
 * them. */
const LABEL_LEFT = 8
const LABEL_RIGHT = 8

/** Compact's date column: the longest date, `Sep 17, 2024`, then room for the two marks, a
 * note's and a signature's seal, which sit at the column's right-hand end. */
export const DATE_COLUMN = 136
/** How far left of the SHA a seal's centre sits. */
export const SEAL_OFFSET = 12
/** How far left of the SHA a note mark's centre sits: a slot of its own, left of the seal's, so
 * marks line up down the column whether or not a commit is signed. */
export const NOTE_OFFSET = 28
/** Half the width of the note mark's slot, for pointing at it. */
const NOTE_REACH = 9
/** What the marks take from the end of the subject when they have no date column to sit in. */
const MARKS_W = NOTE_OFFSET + NOTE_REACH + 1

/** Where a row's note mark is, from its left edge to its right, for hovering over it. */
export function noteMarkSpan(lay: Pick<Layout, 'shaX'>): [start: number, end: number] {
  return [lay.shaX - NOTE_OFFSET - NOTE_REACH, lay.shaX - NOTE_OFFSET + NOTE_REACH]
}

export function layoutColumns(width: number, graphW: number, density: Density, firstLane = 0, remote = 'origin', labelW = LABEL_COLUMN.default): Layout {
  const compact = density === 'compact'
  const refW = refColumnWidth(width, labelW)
  const shaW = 76
  const dateW = compact && width >= 960 ? DATE_COLUMN : 0
  const authorW = compact && width >= 800 ? 150 : 0
  const shaX = width - shaW
  const dateX = shaX - dateW
  return {
    width,
    graphW,
    firstLane,
    density,
    m: METRICS[density],
    refW,
    textX: refW + graphW + (compact ? 8 : 6),
    authorW,
    dateW,
    shaW,
    authorX: dateX - authorW,
    dateX,
    shaX,
    remote,
  }
}

const SERIF = '"Fraunces Variable", Georgia, serif'
const MONO = '"JetBrains Mono Variable", ui-monospace, Consolas, monospace'
const HAND = '"Caveat Variable", "Segoe Script", cursive'

/** The graph's one handwritten note, beside the checked-out commit's label. */
export const HERE_NOTE = 'you’re here'
const HERE_GAP = 8
const HERE_TILT = (-3 * Math.PI) / 180

/** Where the note starts, given where the labels ended, how wide the note is and how wide the
 * column is; null when it wouldn't fit with the column's right-hand margin. */
export function hereNotePlace(labelsEnd: number, noteWidth: number, columnWidth: number): number | null {
  const x = labelsEnd + HERE_GAP
  return x + noteWidth <= columnWidth - 12 ? x : null
}

/** Draws the note after the labels, tilted like every hand note in the app, and returns where
 * it ends; or returns labelsEnd when there wasn't room. */
function drawNote(ctx: CanvasRenderingContext2D, text: string, color: string, labelsEnd: number, mid: number, columnWidth: number): number {
  setFont(ctx, `18px ${HAND}`)
  const w = ctx.measureText(text).width
  const x = hereNotePlace(labelsEnd, w, columnWidth)
  if (x === null) return labelsEnd
  ctx.save()
  ctx.translate(x, mid + 1)
  ctx.rotate(HERE_TILT)
  ctx.fillStyle = color
  ctx.textAlign = 'left'
  ctx.textBaseline = 'middle'
  ctx.fillText(text, 0, 0)
  ctx.restore()
  return x + w
}

function setFont(ctx: CanvasRenderingContext2D, font: string, spacing = '0px') {
  ctx.font = font
  ctx.letterSpacing = spacing
}

/** The rows around a row, for what depends on them. Any of them may not be loaded yet. */
export interface Near {
  prev?: Row
  prev2?: Row
  next?: Row
}

/** Whether a lane is busy at this row: it holds the row's commit, or sits between two commits
 * a row either side. Everything else is idle and draws quietly. */
export function laneActive(col: number, row: Row, near: Near): boolean {
  return col === row.col || (near.prev?.col === col && near.next?.col === col)
}

/** Whether this commit continues its author's run on the lane: their previous commit is on the
 * same lane, within two rows. Merges break a run. */
export function repeatsAuthor(row: Row, near: Near): boolean {
  const before = near.prev?.col === row.col ? near.prev : near.prev2?.col === row.col ? near.prev2 : undefined
  return !!before && before.author === row.author && before.parents <= 1 && row.parents <= 1
}

export type Point = [x: number, y: number]

/** A line between lanes, as a rounded corner. A line leaving a node runs sideways along the
 * node's row then turns down; one arriving comes down then turns in along the row. Both ends
 * away from the node are vertical, so lines join up across rows. */
export function elbow(x1: number, y1: number, x2: number, y2: number, leaving: boolean): { start: Point; before: Point; corner: Point; after: Point; end: Point } {
  const dx = x2 - x1
  const side = dx < 0 ? -1 : 1
  const r = Math.min(Math.abs(dx), Math.abs(y2 - y1), ELBOW_RADIUS)
  return leaving
    ? { start: [x1, y1], before: [x2 - side * r, y1], corner: [x2, y1], after: [x2, y1 + r], end: [x2, y2] }
    : { start: [x1, y1], before: [x1, y2 - r], corner: [x1, y2], after: [x1 + side * r, y2], end: [x2, y2] }
}

/** Where a row's labels were drawn, from their left edge to their right. */
export type LabelSpan = [start: number, end: number]

/** What a bisect says about a commit: an answer already given, the commit under test, or the
 * first bad commit once it's found. */
export type RowMark = 'good' | 'bad' | 'skip' | 'testing' | 'found'

/** Things a row wears on top of what the commit itself is, such as its place in a bisect. */
export interface RowExtra {
  /** Replaces the commit's node. */
  mark?: RowMark
  /** What the 'testing' mark holds in place of its question mark: a rebase's step number. */
  glyph?: string
  /** A teal wash across the row: still in question. */
  band?: boolean
  /** A handwritten note in the label column, in place of "you're here". */
  note?: string
  noteColor?: string
  /** The commit has a git note: a folded-page mark beside the seal. */
  noted?: boolean
  /** The commit just arrived on its branch, by a pull or the like: a teal bar at the right edge. */
  arrived?: boolean
  /** In the timeline, the commit carries on a run: one line, FOLLOWER_H tall. */
  follower?: boolean
}

/** How strong the wash is on a row still in question during a bisect. */
const BAND = 0.12

/** A bisect's mark in place of a commit's node. Each is a shape as well as a colour: a tick, a
 * cross, a dash for a skip, a question mark for the commit under test. */
function drawMark(ctx: CanvasRenderingContext2D, x: number, y: number, mark: RowMark, theme: Theme, cut: (r: number) => void, comfortable: boolean, glyph = '?') {
  const r = comfortable ? 11 : 7
  ctx.save()
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  if (mark === 'found') {
    ctx.globalAlpha *= 0.2
    disc(ctx, x, y, r + 7, theme.bad)
    ctx.globalAlpha /= 0.2
  }
  cut(r + 2)
  const s = r / 11
  if (mark === 'testing') {
    disc(ctx, x, y, r, theme.teal)
    cut(r - 2.5)
    setFont(ctx, `600 12px ${MONO}`)
    ctx.fillStyle = theme.teal
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(glyph, x, y + 0.5)
  } else if (mark === 'skip') {
    disc(ctx, x, y, r, theme.muted)
    cut(r - 1.5)
    ctx.strokeStyle = theme.muted
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(x - 4 * s, y)
    ctx.lineTo(x + 4 * s, y)
    ctx.stroke()
  } else {
    disc(ctx, x, y, r, mark === 'good' ? theme.good : theme.bad)
    ctx.strokeStyle = theme.bg
    ctx.lineWidth = 2.4 * Math.max(s, 0.8)
    ctx.beginPath()
    if (mark === 'good') {
      ctx.moveTo(x - 5 * s, y + 0.5 * s)
      ctx.lineTo(x - 1.5 * s, y + 4 * s)
      ctx.lineTo(x + 5 * s, y - 3.5 * s)
    } else {
      ctx.moveTo(x - 4 * s, y - 4 * s)
      ctx.lineTo(x + 4 * s, y + 4 * s)
      ctx.moveTo(x + 4 * s, y - 4 * s)
      ctx.lineTo(x - 4 * s, y + 4 * s)
    }
    ctx.stroke()
  }
  ctx.restore()
}

/** What a commit wears during a bisect, and whether it's out of the running (faded). `terms`
 * holds the answers by commit; `inQuestion` the commits that could still be the first bad one,
 * or null when that isn't known. */
export function bisectRow(
  id: string,
  b: { terms: Record<string, string>; inQuestion: ReadonlySet<string> | null; current: string; found: string; currentNote?: string; currentGlyph?: string },
  colors: { bad: string },
): { extra: RowExtra; faded: boolean } {
  if (b.found === id) return { extra: { mark: 'found', note: 'it broke here', noteColor: colors.bad }, faded: false }
  if (b.current === id) return { extra: { mark: 'testing', band: true, glyph: b.currentGlyph, note: b.currentNote ?? 'testing this one' }, faded: false }
  const term = b.terms[id]
  if (term === 'good' || term === 'bad' || term === 'skip') return { extra: { mark: term }, faded: false }
  const open = !!b.inQuestion?.has(id)
  return { extra: open ? { band: true } : {}, faded: !open && (b.inQuestion !== null || !!b.found) }
}

/** Draws one row whose top edge is at y. `selected`, `hovered` and `dimmed` are strengths from 0
 * to 1, so the caller can ease them between frames; 1 is the plain state. nowSeconds dates the
 * relative times. Returns where its labels went, if it has any. */
export function drawRow(
  ctx: CanvasRenderingContext2D,
  row: Row,
  refs: Ref[],
  y: number,
  lay: Layout,
  theme: Theme,
  selected: number,
  hovered: number,
  dimmed: number,
  nowSeconds: number,
  seal?: Seal,
  near: Near = {},
  extra: RowExtra = {},
): LabelSpan | null {
  let labelSpan: LabelSpan | null = null
  const { m } = lay
  const comfortable = lay.density === 'comfortable'
  /** A two-line timeline row: a commit that starts a run. */
  const tall = comfortable && !extra.follower
  const h = comfortable && extra.follower ? FOLLOWER_H : m.rowH
  const unit = (v: number) => Math.min(1, Math.max(0, v))
  const sel = unit(selected)
  // Hovering the selected row shows the selection alone.
  const hov = unit(hovered) * (1 - sel)
  const wash = sel > 0 ? theme.selected : hov > 0 ? theme.hover : ''
  const washAlpha = sel > 0 ? sel : hov
  ctx.save()
  // Rows that don't match an active search fade back.
  ctx.globalAlpha = 1 - DIM * unit(dimmed) * (1 - sel)
  const base = ctx.globalAlpha
  /** Paints in the row's wash colour at the wash's current strength. */
  const washOver = (paint: () => void) => {
    if (!wash) return
    const a = ctx.globalAlpha
    ctx.globalAlpha = a * washAlpha
    paint()
    ctx.globalAlpha = a
  }
  if (extra.band) {
    // Still in question during a bisect: a teal wash under whatever else the row wears.
    ctx.globalAlpha = base * BAND
    ctx.fillStyle = theme.teal
    ctx.fillRect(0, y, lay.width, h)
    ctx.globalAlpha = base
  }
  washOver(() => {
    ctx.fillStyle = wash
    ctx.fillRect(0, y, lay.width, h)
  })
  if (sel > 0) {
    const bar = selectionBar(y, h)
    ctx.globalAlpha = base * sel
    ctx.fillStyle = laneColor(row.color)
    ctx.beginPath()
    ctx.roundRect(bar.x, bar.y, bar.w, bar.h, bar.r)
    ctx.fill()
    ctx.globalAlpha = base
  }

  if (extra.arrived) {
    // The mirror of the selection's bar, in teal: this commit is new here.
    const bar = selectionBar(y, h)
    ctx.fillStyle = theme.teal
    ctx.beginPath()
    ctx.roundRect(lay.width - bar.w, bar.y, bar.w, bar.h, bar.r)
    ctx.fill()
  }

  const mid = y + h / 2
  const lx = (col: number) => lay.refW + laneX(col - lay.firstLane, m)
  const graphLeft = lay.refW
  const graphRight = lay.refW + lay.graphW
  const x = lx(row.col)
  const color = laneColor(row.color)
  const docs = row.kind === 'docs'

  const before = ctx.globalAlpha

  // Labels in their own column. A label wears its node's colour, which is all that ties the two.
  if (lay.refW && refs.length) {
    const end = drawLabels(ctx, refs, LABEL_LEFT, mid, lay.refW - LABEL_LEFT - LABEL_RIGHT, color, theme, comfortable, lay.remote) - LABEL_TRAIL
    labelSpan = [LABEL_LEFT, end]
    // The checked-out commit gets the graph's one handwritten word, when the column has room.
    if (comfortable && !extra.note && refs.some((r) => r.head)) drawNote(ctx, HERE_NOTE, theme.link, end, mid, lay.refW)
    if (extra.note) drawNote(ctx, extra.note, extra.noteColor ?? theme.teal, end, mid, lay.refW)
  }

  // A row with no labels still gets its note, from the column's edge.
  if (lay.refW && !refs.length && extra.note) drawNote(ctx, extra.note, extra.noteColor ?? theme.teal, 0, mid, lay.refW)

  // Lanes, clipped to the graph column so wide graphs don't run under the text.
  ctx.save()
  ctx.beginPath()
  ctx.rect(graphLeft, y, lay.graphW, h)
  ctx.clip()
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  // Docs lanes (a published site, not code) draw muted.
  const strength = (s: Seg) => before * (s.kind === 'docs' ? MUTED : 1)
  const straight = (s: Seg, y1: number, y2: number) => {
    const sx = lx(s.from)
    if (sx < graphLeft || sx > graphRight) return
    const active = s.kind === 'trunk' || laneActive(s.from, row, near)
    ctx.globalAlpha = strength(s) * (active ? 1 : QUIET_ALPHA)
    ctx.lineWidth = active ? LINE_WIDTH : QUIET_WIDTH
    ctx.strokeStyle = laneColor(s.color)
    ctx.beginPath()
    ctx.moveTo(sx, y1)
    ctx.lineTo(sx, y2)
    ctx.stroke()
  }
  // A halo in the row's background lets a line pass over the lanes it crosses.
  const stroke = (e: ReturnType<typeof elbow>, width: number, style: string) => {
    ctx.lineWidth = width
    ctx.strokeStyle = style
    ctx.beginPath()
    ctx.moveTo(...e.start)
    ctx.lineTo(...e.before)
    ctx.quadraticCurveTo(...e.corner, ...e.after)
    ctx.lineTo(...e.end)
    ctx.stroke()
  }
  const link = (s: Seg, y1: number, y2: number, leaving: boolean) => {
    const e = elbow(lx(s.from), y1, lx(s.to), y2, leaving)
    ctx.globalAlpha = strength(s)
    stroke(e, HALO_WIDTH, theme.bg)
    washOver(() => stroke(e, HALO_WIDTH, wash))
    stroke(e, LINE_WIDTH, laneColor(s.color))
  }
  for (const s of row.up) if (s.from === s.to) straight(s, y, mid)
  for (const s of row.down) if (s.from === s.to) straight(s, mid, y + h)
  for (const s of row.up) if (s.from !== s.to) link(s, y, mid, false)
  for (const s of row.down) if (s.from !== s.to) link(s, mid, y + h, true)
  ctx.globalAlpha = before
  ctx.lineWidth = LINE_WIDTH
  if (docs) ctx.globalAlpha *= MUTED

  const isHead = refs.some((r) => r.head)
  const isStash = refs.some((r) => r.kind === 'stash')
  /** Fills a disc in the row's own background, so a node cuts the lines behind it. */
  const cut = (r: number) => {
    disc(ctx, x, mid, r, theme.bg)
    washOver(() => disc(ctx, x, mid, r, wash))
  }
  if (isHead) {
    // A soft halo marks the checked-out commit.
    ctx.globalAlpha *= 0.3
    disc(ctx, x, mid, tall ? 16 : 8.5, color)
    ctx.globalAlpha /= 0.3
  }
  if (extra.mark) {
    drawMark(ctx, x, mid, extra.mark, theme, cut, tall, extra.glyph)
  } else if (isStash) {
    // Stashes aren't real history: a dashed ring, like the working tree's.
    const r = tall ? 9 : 5
    cut(r + 2)
    ctx.setLineDash([3, 2.5])
    ctx.strokeStyle = color
    ctx.beginPath()
    ctx.arc(x, mid, r, 0, Math.PI * 2)
    ctx.stroke()
    ctx.setLineDash([])
  } else if (row.parents > 1) {
    // Merges: a ringed dot.
    const r = tall ? 7.5 : 5.5
    cut(r + 1.5)
    disc(ctx, x, mid, r, color)
    cut(r - 2)
    disc(ctx, x, mid, r - 4, color)
  } else if (tall) {
    // A commit that starts a run: the author's initials in a lane-coloured ring; on a
    // Release, Environment, Docs or Bot lane, the kind's icon instead. Trunk and Feature keep
    // the initials, because who did the work matters on those lanes.
    cut(13.5)
    disc(ctx, x, mid, 12, color)
    cut(10.25)
    disc(ctx, x, mid, 9.5, theme.deep)
    const kind = row.kind && row.kind !== 'trunk' && row.kind !== 'feature' ? kindInfo(row.kind) : undefined
    const icon = kind ? iconPath(kind.icon) : undefined
    if (icon) {
      ctx.save()
      ctx.translate(x - 6, mid - 6)
      ctx.scale(12 / 24, 12 / 24)
      ctx.lineWidth = 2.2
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.strokeStyle = theme.soft
      ctx.stroke(icon)
      ctx.restore()
    } else {
      setFont(ctx, `500 12px ${MONO}`, '-0.03em')
      ctx.fillStyle = theme.soft
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillText(initials(row.author), x, mid + 0.5)
      ctx.letterSpacing = '0px'
    }
  } else if (comfortable) {
    // Carrying on a run: a dot, since the initials at the top of the run already say who.
    cut(6.5)
    disc(ctx, x, mid, 5, color)
  } else {
    cut(6)
    disc(ctx, x, mid, 4.5, color)
  }
  if (docs) ctx.globalAlpha /= MUTED
  ctx.restore()

  ctx.textBaseline = 'middle'
  const line1 = tall ? y + 19 : mid
  ctx.textAlign = 'left'
  setFont(ctx, `12px ${MONO}`)
  ctx.fillStyle = theme.muted
  ctx.fillText(row.id.slice(0, 7), lay.shaX, line1)
  if (seal) drawSeal(ctx, lay.shaX - SEAL_OFFSET, line1, seal, theme)
  if (extra.noted) drawNoteMark(ctx, lay.shaX - NOTE_OFFSET, line1, theme.note)
  if (lay.dateW) {
    setFont(ctx, `12px ${MONO}`)
    ctx.fillStyle = theme.muted
    ctx.fillText(relativeTime(row.authorTime, nowSeconds), lay.dateX, line1)
  }
  if (lay.authorW) {
    // Inside an author's run the column says "same again" with a ditto, so one name isn't
    // shouted down the column; the node's dot already said so.
    const ditto = !isHead && repeatsAuthor(row, near)
    setFont(ctx, `13px ${SERIF}`)
    ctx.fillStyle = theme.muted
    if (ditto) ctx.globalAlpha *= 0.6
    // Without a date column the marks sit at the end of this one.
    ctx.fillText(ditto ? DITTO : clip(ctx, row.author, lay.authorW - (lay.dateW ? 16 : MARKS_W)), lay.authorX, line1)
    if (ditto) ctx.globalAlpha /= 0.6
  }

  let left = lay.textX
  // The subject stops short of the next column, or of the marks when the SHA is next.
  let subjectRight = lay.authorW ? lay.authorX - 22 : lay.dateW ? lay.dateX - 22 : lay.shaX - MARKS_W
  if (comfortable && extra.follower) {
    // A row that carries on a run says only when, quietly, at the end of its one line.
    const time = timeLabel(row.authorTime).toLowerCase()
    setFont(ctx, `13px ${SERIF}`)
    ctx.fillStyle = theme.faint
    ctx.textAlign = 'right'
    ctx.fillText(time, subjectRight - 4, line1)
    ctx.textAlign = 'left'
    subjectRight -= ctx.measureText(time).width + 18
  }
  if (!lay.refW && refs.length) {
    left = drawLabels(ctx, refs, left, line1, Math.min(subjectRight - left, (subjectRight - left) * 0.6), color, theme, comfortable, lay.remote)
    labelSpan = [lay.textX, left - LABEL_TRAIL]
  }
  setFont(ctx, `${comfortable ? 14 : 13}px ${SERIF}`)
  ctx.fillStyle = docs ? theme.muted : theme.fg
  ctx.fillText(clip(ctx, row.subject, subjectRight - left), left, line1)

  if (tall) {
    const meta = metaLine(row, near, nowSeconds)
    if (meta.divider) {
      // The day changes here: a dashed hairline along the row's top edge, from the message on.
      ctx.save()
      ctx.globalAlpha *= 0.45
      ctx.strokeStyle = theme.muted
      ctx.lineWidth = 1
      ctx.setLineDash([3, 3])
      ctx.beginPath()
      ctx.moveTo(lay.textX, y + 0.5)
      ctx.lineTo(lay.width - 14, y + 0.5)
      ctx.stroke()
      ctx.restore()
    }
    setFont(ctx, `12px ${MONO}`, '0.08em')
    ctx.fillStyle = theme.muted
    ctx.fillText(clip(ctx, meta.text.toUpperCase(), lay.width - 14 - lay.textX), lay.textX, y + 37)
    ctx.letterSpacing = '0px'
  }
  ctx.restore()
  return labelSpan
}

function disc(ctx: CanvasRenderingContext2D, x: number, y: number, r: number, color: string) {
  ctx.beginPath()
  ctx.arc(x, y, r, 0, Math.PI * 2)
  ctx.fillStyle = color
  ctx.fill()
}

/** Where a branch kind's labels rank; see labelRank. */
const BRANCH_RANK: Record<string, number> = { trunk: 1, release: 2, environment: 4, feature: 5, docs: 7, bot: 7 }

/** What a label ranks by, lowest first. The label column runs out of room, so what you'd most
 * want to see comes first: the checked-out branch, Trunk, Release, tags, Environment, Feature,
 * other branches, Docs and Bot, then stashes. */
function labelRank(r: Ref): number {
  if (r.head) return 0
  if (r.kind === 'tag') return 3
  if (r.kind === 'stash') return 8
  return BRANCH_RANK[r.branchKind ?? ''] ?? 6
}

export function sortRefs(refs: Ref[]): Ref[] {
  return [...refs].sort((a, b) => labelRank(a) - labelRank(b) || Number(a.kind === 'remote') - Number(b.kind === 'remote'))
}

/** A label as drawn: what it says, and how many remotes the name is on. */
export interface RefLabel {
  ref: Ref
  text: string
  remotes: number
  /** Where this name lives, for spelling it out: "local", then each remote. Empty for tags. */
  places: string[]
}

/** The remote a remote branch is on, assuming a remote's name has no slash in it. */
export function remoteOf(r: Pick<Ref, 'short'>): string {
  const slash = r.short.indexOf('/')
  return slash < 0 ? '' : r.short.slice(0, slash)
}

/** The repository's main remote, whose prefix labels leave off: the checked-out branch's
 * upstream remote, else `origin` if there is one, else the only remote. */
export function mainRemote(refs: Ref[]): string {
  const upstream = refs.find((r) => r.head && r.kind === 'local')?.upstream
  if (upstream && remoteOf({ short: upstream })) return remoteOf({ short: upstream })
  const remotes = new Set(refs.filter((r) => r.kind === 'remote').map(remoteOf))
  if (remotes.has('origin')) return 'origin'
  return remotes.size === 1 ? [...remotes][0] : ''
}

/** Labels for a commit's refs, in display order. Remote branches of the same name share a
 * label, with a count of the remotes. A local branch shares a label with the remote branch it
 * tracks on this commit, and its namesakes; one that tracks nothing shares a label with its
 * namesake on the main remote. The main remote's prefix gives way to the remote icon, and a
 * branch on just one other remote keeps its prefix, so `fork/x` stays apart from `x`. */
export function refLabels(refs: Ref[], main = 'origin'): RefLabel[] {
  const sorted = sortRefs(refs)
  const branchOf = (r: Pick<Ref, 'short'>) => r.short.slice(remoteOf(r).length + 1)
  const isHeadRef = (r: Ref) => branchOf(r) === 'HEAD'
  /** The remote branches on this commit, by branch name. */
  const onRemotes = new Map<string, Ref[]>()
  for (const r of sorted) {
    if (r.kind !== 'remote' || isHeadRef(r)) continue
    const name = branchOf(r)
    onRemotes.set(name, [...(onRemotes.get(name) ?? []), r])
  }
  /** The branch name a local branch folds with, if it's on this commit. */
  const foldsWith = (r: Ref) => {
    if (r.upstream) return onRemotes.get(branchOf({ short: r.upstream }))?.some((o) => o.short === r.upstream) ? branchOf({ short: r.upstream }) : null
    return onRemotes.get(r.short)?.some((o) => remoteOf(o) === main) ? r.short : null
  }
  const labelled = new Set<string>()
  const labels: RefLabel[] = []
  for (const r of sorted) {
    if (r.kind === 'remote') {
      if (labelled.has(r.short)) continue
      const same = isHeadRef(r) ? [r] : (onRemotes.get(branchOf(r)) ?? [r])
      for (const o of same) labelled.add(o.short)
      labels.push({ ref: r, text: !isHeadRef(r) && (same.length > 1 || remoteOf(r) === main) ? branchOf(r) : r.short, remotes: same.length, places: same.map(remoteOf) })
    } else if (r.kind === 'local') {
      const name = foldsWith(r)
      const same = name === null ? [] : (onRemotes.get(name) ?? [])
      for (const o of same) labelled.add(o.short)
      labels.push({ ref: r, text: r.short, remotes: same.length, places: ['local', ...same.map(remoteOf)] })
    } else {
      labels.push({ ref: r, text: (r.kind === 'tag' ? '◆ ' : '≡ ') + r.short, remotes: 0, places: [] })
    }
  }
  return labels
}

/** What a label needs besides its name: its padding, its kind and stack icons, its remote mark. */
function labelChrome(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, r: Ref, remotes: number): number {
  return 14 + (labelKind(r) ? KIND_ICON_W : 0) + (r.stackDepth ? KIND_ICON_W : 0) + remoteMarkWidth(ctx, remotes)
}

/** The gap between a label and what follows it. */
const LABEL_GAP = 5

/** The font labels are set in, for measuring them. */
export const LABEL_FONT = `12px ${MONO}`

/** The count after a commit's one label: "+2" for two more, nothing for a label alone. */
export function moreLabels(count: number): string {
  return count > 1 ? `+${count - 1}` : ''
}

/** How wide the label column has to be to show a commit's first label whole, with the count of
 * the rest; 0 for a commit with no labels. The context's font must be LABEL_FONT. */
export function labelColumnNeed(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, refs: Ref[], remoteName: string): number {
  if (refs.length === 0) return 0
  const labels = refLabels(refs, remoteName)
  const { ref, text, remotes } = labels[0]
  const more = moreLabels(labels.length)
  return LABEL_LEFT + ctx.measureText(text).width + labelChrome(ctx, ref, remotes) + (more ? LABEL_GAP + ctx.measureText(more).width : 0) + LABEL_RIGHT
}

/** Draws a commit's labels starting at x: the one that ranks first, then a count of the rest.
 * A name too long for maxWidth is cut in the middle. Returns the x where the subject should
 * start. */
function drawLabels(
  ctx: CanvasRenderingContext2D,
  refs: Ref[],
  x: number,
  mid: number,
  maxWidth: number,
  color: string,
  theme: Theme,
  comfortable: boolean,
  remoteName: string,
): number {
  if (refs.length === 0) return x
  const h = comfortable ? 18 : 17
  const labels = refLabels(refs, remoteName)
  const { ref: r, text, remotes } = labels[0]
  ctx.save()
  setFont(ctx, LABEL_FONT)
  let more = moreLabels(labels.length)
  const moreW = more ? LABEL_GAP + ctx.measureText(more).width : 0
  const chrome = labelChrome(ctx, r, remotes)
  const name = middleCut(ctx, text, maxWidth - moreW - chrome)
  if (name === null) {
    // No room for even a cut name: only say how many labels there are.
    more = `+${labels.length}`
  } else {
    const w = ctx.measureText(name).width + chrome
    drawLabel(ctx, name, x, mid, w, h, r, color, theme, remotes)
    x += w + LABEL_GAP
  }
  if (more) {
    setFont(ctx, LABEL_FONT)
    const w = ctx.measureText(more).width
    if (name !== null || w <= maxWidth) {
      ctx.fillStyle = theme.muted
      ctx.textAlign = 'left'
      ctx.textBaseline = 'middle'
      ctx.fillText(more, x, mid + 0.5)
      x += w + LABEL_GAP
    }
  }
  ctx.restore()
  return x + 3
}

/** The width of a label's remote icon, and its count when there's more than one remote. */
function remoteMarkWidth(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, remotes: number): number {
  if (!remotes) return 0
  return KIND_ICON_W + (remotes > 1 ? ctx.measureText(String(remotes)).width + 2 : 0)
}

/** One ref label: a soft wash of the lane colour, like the brand's tags. The checked-out
 * branch is solid; tags take the ochre wash whatever their lane. */
function drawLabel(ctx: CanvasRenderingContext2D, text: string, x: number, mid: number, w: number, h: number, r: Ref, color: string, theme: Theme, remotes: number) {
  const y = mid - h / 2
  const kind = labelKind(r)
  const kindColor = kind ? theme.kinds?.[kind.kind] : undefined
  const fill = r.kind === 'tag' ? theme.tag : (kindColor ?? color)
  ctx.beginPath()
  ctx.roundRect(x, y, w, h, 4)
  if (r.head) {
    ctx.fillStyle = fill
    ctx.fill()
  } else if (kind?.kind === 'docs') {
    // Docs: an outline, not a wash, so a published site reads as "not code".
    ctx.save()
    ctx.strokeStyle = fill
    ctx.lineWidth = 1
    ctx.stroke()
    ctx.restore()
  } else {
    ctx.save()
    ctx.globalAlpha *= r.kind === 'remote' ? 0.12 : 0.22
    ctx.fillStyle = fill
    ctx.fill()
    ctx.restore()
  }
  if (r.kind === 'stash') {
    ctx.save()
    ctx.setLineDash([2, 2])
    ctx.strokeStyle = fill
    ctx.lineWidth = 1
    ctx.stroke()
    ctx.restore()
  }
  let tx = x + 7
  const icon = kind ? iconPath(kind.icon) : undefined
  if (kind && icon) {
    const size = 11
    ctx.save()
    ctx.translate(x + 6, mid - size / 2)
    ctx.scale(size / 24, size / 24)
    ctx.lineWidth = 2.4
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = r.head ? theme.bg : fill
    ctx.stroke(icon)
    ctx.restore()
  }
  if (kind) tx += KIND_ICON_W - 1
  const stack = r.stackDepth ? iconPath('stack') : undefined
  if (stack) {
    const size = 11
    ctx.save()
    ctx.translate(tx - 1, mid - size / 2)
    ctx.scale(size / 24, size / 24)
    ctx.lineWidth = 2.4
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = r.head ? theme.bg : theme.muted
    ctx.stroke(stack)
    ctx.restore()
    tx += KIND_ICON_W
  }
  const weight = r.head || kind?.kind === 'trunk' ? '600 ' : ''
  setFont(ctx, `${weight}12px ${MONO}`)
  ctx.fillStyle = r.head ? theme.bg : r.kind === 'remote' || kind?.kind === 'bot' ? theme.muted : theme.fg
  ctx.textBaseline = 'middle'
  ctx.fillText(text, tx, mid + 0.5)
  const cloud = remotes ? iconPath('remote') : undefined
  if (cloud) {
    const size = 11
    const cloudX = tx + ctx.measureText(text).width + 3
    if (remotes > 1) {
      ctx.fillStyle = r.head ? theme.bg : theme.muted
      ctx.fillText(String(remotes), cloudX + size + 2, mid + 0.5)
    }
    ctx.save()
    ctx.translate(cloudX, mid - size / 2)
    ctx.scale(size / 24, size / 24)
    ctx.lineWidth = 2.4
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = r.head ? theme.bg : theme.muted
    ctx.stroke(cloud)
    ctx.restore()
  }
}

/** The fewest characters of a cut name worth showing: "617-…ends" is about the least that says
 * anything. */
const MIN_CUT_NAME = 8

/** A ref's name cut in the middle to fit maxWidth, or null if too little of it would be left to
 * read. Branch names in a team share their beginnings (`feature/`, an issue number, a person),
 * so the end is kept as well as the start; an odd character goes to the start. A tag's or a
 * stash's mark stays in front and isn't counted as part of the name. */
export function middleCut(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, text: string, maxWidth: number): string | null {
  if (ctx.measureText(text).width <= maxWidth) return text
  const mark = /^[◆≡] /.exec(text)?.[0] ?? ''
  const name = [...text.slice(mark.length)]
  const cut = (keep: number) => mark + name.slice(0, Math.ceil(keep / 2)).join('') + '…' + name.slice(name.length - Math.floor(keep / 2)).join('')
  let lo = 0
  let hi = name.length - 1
  while (lo < hi) {
    const m = (lo + hi + 1) >> 1
    if (ctx.measureText(cut(m)).width <= maxWidth) lo = m
    else hi = m - 1
  }
  return lo < MIN_CUT_NAME ? null : cut(lo)
}

/** Truncates text with an ellipsis to fit maxWidth pixels. */
export function clip(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, text: string, maxWidth: number): string {
  if (maxWidth <= 0) return ''
  if (ctx.measureText(text).width <= maxWidth) return text
  let lo = 0
  let hi = text.length
  while (lo < hi) {
    const m = (lo + hi + 1) >> 1
    if (ctx.measureText(text.slice(0, m) + '…').width <= maxWidth) lo = m
    else hi = m - 1
  }
  return text.slice(0, lo) + '…'
}

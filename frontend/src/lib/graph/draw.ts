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

/** How strong the lane's colour is behind a row, and how far past the message's start it runs. */
const LANE_WASH = 0.12
const DOCS_LANE_WASH = 0.07
const LANE_WASH_TAIL = 90

/** How far past a label's right edge drawLabels' returned position is. */
const LABEL_TRAIL = 8

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

/** The branch and tag column's width: labels go back inline when the window can't spare it. */
export function refColumnWidth(width: number): number {
  return width >= 640 ? 150 : 0
}

export function layoutColumns(width: number, graphW: number, density: Density, firstLane = 0, remote = 'origin'): Layout {
  const compact = density === 'compact'
  const refW = refColumnWidth(width)
  const shaW = 76
  const dateW = compact && width >= 960 ? 104 : 0
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

/** The same colour fully transparent, so a fade doesn't drift to grey on the way out. */
export function clearOf(color: string): string {
  return /^#[0-9a-f]{6}$/i.test(color) ? `${color}00` : 'transparent'
}

/** Where a row's labels were drawn, from their left edge to their right. */
export type LabelSpan = [start: number, end: number]

/** Draws one row whose top edge is at y. nowSeconds dates the relative times. Returns where its
 * labels went, if it has any. */
export function drawRow(
  ctx: CanvasRenderingContext2D,
  row: Row,
  refs: Ref[],
  y: number,
  lay: Layout,
  theme: Theme,
  selected: boolean,
  hovered: boolean,
  dimmed: boolean,
  nowSeconds: number,
  seal?: Seal,
  near: Near = {},
): LabelSpan | null {
  let labelSpan: LabelSpan | null = null
  const { m } = lay
  const h = m.rowH
  const comfortable = lay.density === 'comfortable'
  const wash = selected ? theme.selected : hovered ? theme.hover : ''
  ctx.save()
  // Rows that don't match an active search fade back.
  if (dimmed && !selected) ctx.globalAlpha = 0.3
  if (wash) {
    ctx.fillStyle = wash
    ctx.fillRect(0, y, lay.width, h)
  }

  const mid = y + h / 2
  const lx = (col: number) => lay.refW + laneX(col - lay.firstLane, m)
  const graphLeft = lay.refW
  const graphRight = lay.refW + lay.graphW
  const x = lx(row.col)
  const color = laneColor(row.color)
  const docs = row.kind === 'docs'

  // The lane's colour runs from the node into the start of the message, so the row reads as
  // belonging to its lane without tracing the line.
  const washX = Math.max(x, graphLeft)
  const washEnd = lay.textX + LANE_WASH_TAIL
  const tint = ctx.createLinearGradient(washX, 0, washEnd, 0)
  tint.addColorStop(0, color)
  tint.addColorStop(0.74, color)
  tint.addColorStop(1, clearOf(color))
  const before = ctx.globalAlpha
  ctx.globalAlpha = before * (docs ? DOCS_LANE_WASH : LANE_WASH)
  ctx.fillStyle = tint
  ctx.fillRect(washX, y, washEnd - washX, h)
  ctx.globalAlpha = before

  // Labels in their own column, each tied to its node by a thin line.
  if (lay.refW && refs.length) {
    const end = drawLabels(ctx, refs, 8, mid, lay.refW - 16, color, theme, comfortable, lay.remote) - LABEL_TRAIL
    labelSpan = [8, end]
    const to = Math.min(x, graphRight)
    if (end < to && x >= graphLeft) {
      ctx.globalAlpha = before * 0.45
      ctx.lineWidth = 1
      ctx.strokeStyle = color
      ctx.beginPath()
      ctx.moveTo(end, mid)
      ctx.lineTo(to, mid)
      ctx.stroke()
      ctx.globalAlpha = before
    }
  }

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
    if (wash) stroke(e, HALO_WIDTH, wash)
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
    if (wash) disc(ctx, x, mid, r, wash)
  }
  if (isHead) {
    // A soft halo marks the checked-out commit.
    ctx.globalAlpha *= 0.3
    disc(ctx, x, mid, comfortable ? 16 : 8.5, color)
    ctx.globalAlpha /= 0.3
  }
  if (isStash) {
    // Stashes aren't real history: a dashed ring, like the working tree's.
    const r = comfortable ? 9 : 5
    cut(r + 2)
    ctx.setLineDash([3, 2.5])
    ctx.strokeStyle = color
    ctx.beginPath()
    ctx.arc(x, mid, r, 0, Math.PI * 2)
    ctx.stroke()
    ctx.setLineDash([])
  } else if (row.parents > 1) {
    // Merges: a ringed dot.
    const r = comfortable ? 7.5 : 5.5
    cut(r + 1.5)
    disc(ctx, x, mid, r, color)
    cut(r - 2)
    disc(ctx, x, mid, r - 4, color)
  } else if (comfortable && (isHead || !repeatsAuthor(row, near))) {
    // The author's initials in a lane-coloured ring, as on dev-null's timeline; on a
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
    // The same author carrying on: a dot, since the initials just above already say who.
    cut(6.5)
    disc(ctx, x, mid, 5, color)
  } else {
    cut(6)
    disc(ctx, x, mid, 4.5, color)
  }
  if (docs) ctx.globalAlpha /= MUTED
  ctx.restore()

  ctx.textBaseline = 'middle'
  const line1 = comfortable ? y + 19 : mid
  ctx.textAlign = 'left'
  setFont(ctx, `12px ${MONO}`)
  ctx.fillStyle = theme.muted
  ctx.fillText(row.id.slice(0, 7), lay.shaX, line1)
  if (seal) drawSeal(ctx, lay.shaX - 10, line1 - 4, seal, theme)
  if (lay.dateW) {
    setFont(ctx, `12px ${MONO}`)
    ctx.fillStyle = theme.muted
    ctx.fillText(relativeTime(row.authorTime, nowSeconds), lay.dateX, line1)
  }
  if (lay.authorW) {
    setFont(ctx, `13px ${SERIF}`)
    ctx.fillStyle = theme.muted
    ctx.fillText(clip(ctx, row.author, lay.authorW - 16), lay.authorX, line1)
  }

  let left = lay.textX
  const subjectRight = (lay.authorW ? lay.authorX : lay.dateW ? lay.dateX : lay.shaX) - 22
  if (!lay.refW && refs.length) {
    left = drawLabels(ctx, refs, left, line1, Math.min(subjectRight - left, (subjectRight - left) * 0.6), color, theme, comfortable, lay.remote)
    labelSpan = [lay.textX, left - LABEL_TRAIL]
  }
  setFont(ctx, `${comfortable ? 14.5 : 13.5}px ${SERIF}`)
  ctx.fillStyle = docs ? theme.muted : theme.fg
  ctx.fillText(clip(ctx, row.subject, subjectRight - left), left, line1)

  if (comfortable) {
    setFont(ctx, `12px ${MONO}`, '0.08em')
    ctx.fillStyle = theme.muted
    const meta = `${row.author} · ${relativeTime(row.authorTime, nowSeconds)}`.toUpperCase()
    ctx.fillText(clip(ctx, meta, lay.width - 14 - lay.textX), lay.textX, y + 37)
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
      labels.push({ ref: r, text: !isHeadRef(r) && (same.length > 1 || remoteOf(r) === main) ? branchOf(r) : r.short, remotes: same.length })
    } else if (r.kind === 'local') {
      const name = foldsWith(r)
      const same = name === null ? [] : (onRemotes.get(name) ?? [])
      for (const o of same) labelled.add(o.short)
      labels.push({ ref: r, text: r.short, remotes: same.length })
    } else {
      labels.push({ ref: r, text: (r.kind === 'tag' ? '◆ ' : '≡ ') + r.short, remotes: 0 })
    }
  }
  return labels
}

/** Draws ref labels starting at x; returns the x where the subject should start. */
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
  const end = x + maxWidth
  const h = comfortable ? 18 : 17
  const labels = refLabels(refs, remoteName)
  ctx.save()
  setFont(ctx, `12px ${MONO}`)
  for (let i = 0; i < labels.length; i++) {
    const { ref: r, text, remotes } = labels[i]
    const remaining = labels.length - i
    const more = remaining > 1 ? ` +${remaining - 1}` : ''
    const extra = 14 + (labelKind(r) ? KIND_ICON_W : 0) + (r.stackDepth ? KIND_ICON_W : 0) + remoteMarkWidth(ctx, remotes)
    let w = ctx.measureText(text).width + extra
    if (x + w > end) {
      // Out of room: one clipped label that also counts the rest, while enough of its name is left.
      const clipped = clippedName(ctx, text, Math.max(0, end - x - extra - ctx.measureText(more).width))
      if (clipped !== null) {
        const label = clipped + more
        w = ctx.measureText(label).width + extra
        drawLabel(ctx, label, x, mid, w, h, r, color, theme, remotes)
        x += w + 5
        break
      }
      // Otherwise only say how many there are, if even that fits.
      setFont(ctx, `12px ${MONO}`)
      const count = `+${remaining}`
      const countW = ctx.measureText(count).width
      if (x + countW <= end) {
        ctx.fillStyle = theme.muted
        ctx.textBaseline = 'middle'
        ctx.fillText(count, x, mid + 0.5)
        x += countW + 5
      }
      break
    }
    drawLabel(ctx, text, x, mid, w, h, r, color, theme, remotes)
    x += w + 5
  }
  ctx.restore()
  return x + 3
}

/** The width of a label's remote icon, and its count when there's more than one remote. */
function remoteMarkWidth(ctx: CanvasRenderingContext2D, remotes: number): number {
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

/** The fewest characters of a cut-off name worth showing: "origin/f…" is about the least that
 * says anything. */
const MIN_CLIPPED_NAME = 8

/** A ref's name cut to fit maxWidth, or null if too little of it would be left to read. */
export function clippedName(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, text: string, maxWidth: number): string | null {
  const clipped = clip(ctx, text, maxWidth)
  if (clipped === text) return text
  return clipped.replace(/^[◆≡] /, '').length - 1 < MIN_CLIPPED_NAME ? null : clipped
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

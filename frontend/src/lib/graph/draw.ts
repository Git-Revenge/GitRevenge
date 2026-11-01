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
  comfortable: { rowH: 52, laneW: 22, pad: 16 },
  compact: { rowH: 30, laneW: 16, pad: 12 },
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
  /** Compact's right-hand columns: author, date, sha. */
  authorW: number
  dateW: number
  shaW: number
}

export function layoutColumns(width: number, graphW: number, density: Density, firstLane = 0): Layout {
  const compact = density === 'compact'
  return {
    width,
    graphW,
    firstLane,
    density,
    m: METRICS[density],
    shaW: 76,
    dateW: compact && width > 900 ? 104 : 0,
    authorW: compact && width > 700 ? 150 : 0,
  }
}

const SERIF = '"Fraunces Variable", Georgia, serif'
const MONO = '"JetBrains Mono Variable", ui-monospace, Consolas, monospace'

function setFont(ctx: CanvasRenderingContext2D, font: string, spacing = '0px') {
  ctx.font = font
  ctx.letterSpacing = spacing
}

/** Draws one row whose top edge is at y. nowSeconds dates the relative times. */
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
) {
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

  // Lanes, clipped to the graph column so wide graphs don't run under the text.
  ctx.save()
  ctx.beginPath()
  ctx.rect(0, y, lay.graphW, h)
  ctx.clip()
  ctx.lineWidth = 2
  ctx.lineCap = 'round'
  const mid = y + h / 2
  const lx = (col: number) => laneX(col - lay.firstLane, m)
  // Docs lanes (a published site, not code) draw muted.
  const lane = (s: Seg, x1: number, y1: number, x2: number, y2: number) => {
    if (s.kind === 'docs') ctx.globalAlpha *= MUTED
    segment(ctx, x1, y1, x2, y2, laneColor(s.color))
    if (s.kind === 'docs') ctx.globalAlpha /= MUTED
  }
  for (const s of row.up) lane(s, lx(s.from), y, lx(s.to), mid)
  for (const s of row.down) lane(s, lx(s.from), mid, lx(s.to), y + h)
  const docs = row.kind === 'docs'
  if (docs) ctx.globalAlpha *= MUTED

  const x = lx(row.col)
  const color = laneColor(row.color)
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
    disc(ctx, x, mid, comfortable ? 18 : 9.5, color)
    ctx.globalAlpha /= 0.3
  }
  if (isStash) {
    // Stashes aren't real history: a dashed ring, like the working tree's.
    const r = comfortable ? 10 : 6
    cut(r + 2)
    ctx.setLineDash([3, 2.5])
    ctx.strokeStyle = color
    ctx.beginPath()
    ctx.arc(x, mid, r, 0, Math.PI * 2)
    ctx.stroke()
    ctx.setLineDash([])
  } else if (row.parents > 1) {
    // Merges: a ringed dot.
    const r = comfortable ? 8 : 6
    cut(r + 1.5)
    disc(ctx, x, mid, r, color)
    cut(r - 2)
    disc(ctx, x, mid, r - 4, color)
  } else if (comfortable) {
    // The author's initials in a lane-coloured ring, as on dev-null's timeline; on a
    // Release, Environment, Docs or Bot lane, the kind's icon instead. Trunk and Feature keep
    // the initials, because who did the work matters on those lanes.
    cut(15)
    disc(ctx, x, mid, 13.5, color)
    cut(11.5)
    disc(ctx, x, mid, 10.5, theme.deep)
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
      setFont(ctx, `500 9.5px ${MONO}`)
      ctx.fillStyle = theme.soft
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillText(initials(row.author), x, mid + 0.5)
    }
  } else {
    cut(7)
    disc(ctx, x, mid, 5.5, color)
  }
  if (docs) ctx.globalAlpha /= MUTED
  ctx.restore()

  ctx.textBaseline = 'middle'
  const line1 = comfortable ? y + 19 : mid
  let right = lay.width - 14
  setFont(ctx, `11.5px ${MONO}`)
  ctx.fillStyle = theme.muted
  ctx.textAlign = 'right'
  ctx.fillText(row.id.slice(0, 7), right, line1)
  if (seal) drawSeal(ctx, right - ctx.measureText(row.id.slice(0, 7)).width - 10, line1 - 4, seal, theme)
  right -= lay.shaW
  if (lay.dateW) {
    setFont(ctx, `11px ${MONO}`)
    ctx.fillText(relativeTime(row.authorTime, nowSeconds), right, line1)
    right -= lay.dateW
  }
  if (lay.authorW) {
    setFont(ctx, `13px ${SERIF}`)
    ctx.fillText(clip(ctx, row.author, lay.authorW - 16), right, line1)
    right -= lay.authorW
  }

  ctx.textAlign = 'left'
  let left = lay.graphW + (comfortable ? 6 : 8)
  const subjectRight = right - 12
  left = drawLabels(ctx, refs, left, line1, Math.min(subjectRight - left, (subjectRight - left) * 0.6), color, theme, comfortable)
  setFont(ctx, `${comfortable ? 14.5 : 13.5}px ${SERIF}`)
  ctx.fillStyle = docs ? theme.muted : theme.fg
  ctx.fillText(clip(ctx, row.subject, subjectRight - left), left, line1)

  if (comfortable) {
    setFont(ctx, `10.5px ${MONO}`, '0.08em')
    ctx.fillStyle = theme.muted
    const meta = `${row.author} · ${relativeTime(row.authorTime, nowSeconds)}`.toUpperCase()
    ctx.fillText(clip(ctx, meta, lay.width - 14 - (lay.graphW + 6)), lay.graphW + 6, y + 37)
    ctx.letterSpacing = '0px'
  }
  ctx.restore()
}

function disc(ctx: CanvasRenderingContext2D, x: number, y: number, r: number, color: string) {
  ctx.beginPath()
  ctx.arc(x, y, r, 0, Math.PI * 2)
  ctx.fillStyle = color
  ctx.fill()
}

/** Sort order for labels: checked-out branch, local, remote, tags, stashes. */
const KIND_ORDER: Record<Ref['kind'], number> = { local: 1, remote: 2, tag: 3, stash: 4 }

export function sortRefs(refs: Ref[]): Ref[] {
  return [...refs].sort((a, b) => Number(b.head) - Number(a.head) || KIND_ORDER[a.kind] - KIND_ORDER[b.kind])
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
): number {
  if (refs.length === 0) return x
  const end = x + maxWidth
  const h = comfortable ? 18 : 17
  const sorted = sortRefs(refs)
  ctx.save()
  setFont(ctx, `10.5px ${MONO}`)
  for (let i = 0; i < sorted.length; i++) {
    const r = sorted[i]
    const remaining = sorted.length - i
    const more = remaining > 1 ? ` +${remaining - 1}` : ''
    const prefix = r.kind === 'tag' ? '◆ ' : r.kind === 'stash' ? '≡ ' : ''
    const text = prefix + r.short
    const extra = 14 + (labelKind(r) ? KIND_ICON_W : 0) + (r.stackDepth ? KIND_ICON_W : 0)
    let w = ctx.measureText(text).width + extra
    if (x + w > end) {
      // Out of room: one clipped label that also counts the rest.
      const label = clip(ctx, text, Math.max(0, end - x - extra - ctx.measureText(more).width)) + more
      w = ctx.measureText(label).width + extra
      if (w > 26) drawLabel(ctx, label, x, mid, w, h, r, color, theme)
      x += w + 5
      break
    }
    drawLabel(ctx, text, x, mid, w, h, r, color, theme)
    x += w + 5
  }
  ctx.restore()
  return x + 3
}

/** One ref label: a soft wash of the lane colour, like the brand's tags. The checked-out
 * branch is solid; tags take the ochre wash whatever their lane. */
function drawLabel(ctx: CanvasRenderingContext2D, text: string, x: number, mid: number, w: number, h: number, r: Ref, color: string, theme: Theme) {
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
  setFont(ctx, `${weight}${kind?.kind === 'bot' ? 9.5 : 10.5}px ${MONO}`)
  ctx.fillStyle = r.head ? theme.bg : r.kind === 'remote' || kind?.kind === 'bot' ? theme.muted : theme.fg
  ctx.textBaseline = 'middle'
  ctx.fillText(text, tx, mid + 0.5)
}

function segment(ctx: CanvasRenderingContext2D, x1: number, y1: number, x2: number, y2: number, color: string) {
  ctx.strokeStyle = color
  ctx.beginPath()
  ctx.moveTo(x1, y1)
  if (x1 === x2) {
    ctx.lineTo(x2, y2)
  } else {
    // Vertical tangents at both ends so lines enter and leave rows straight.
    const cy = (y1 + y2) / 2
    ctx.bezierCurveTo(x1, cy, x2, cy, x2, y2)
  }
  ctx.stroke()
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

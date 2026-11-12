// The Map view's layout: one node per branch and tag, held around the trunk by springs. A
// node's size is its commits and its distance from the trunk is how long since it moved. The
// forces are run to rest here, the same way every time, so the picture doesn't jump about
// between two looks at the same repository.

/** A node as the backend sends it (app.MapNode in Go). */
export interface MapNode {
  name: string
  kind: 'trunk' | 'branch' | 'remote' | 'tag' | 'tags' | 'group'
  commit: string
  commits: number
  /** When it last moved, in Unix seconds. */
  updated: number
  /** The branch node it hangs off, or '' for the trunk: the branch that holds a tag, or the
   * one a stacked branch is stacked on. */
  parent: string
  stacked?: boolean
  /** The folder of the other worktree the branch is checked out in. */
  worktree?: string
  head: boolean
  branchKind?: string
  /** How many tags a fold holds. */
  tags?: number
  /** How many branches a group stands for: remote-only branches that share a first path
   * segment are one node until it's opened. */
  branches?: number
}

export interface Placed extends MapNode {
  x: number
  y: number
  /** How far the node reaches from its centre: a branch's radius, half a tag's diagonal. */
  r: number
  /** What's drawn: a branch's diameter, a tag's side. */
  size: number
  /** Which ring it sits within, counting out from the trunk: 0 is the innermost, and one more
   * than the last ring is older than all of them. */
  band: number
  /** The name goes over the node, not under it: the side away from the trunk. */
  above: boolean
  /** Someone put it here by hand: the springs leave it alone. */
  pinned?: boolean
  /** Its name shows. In a busy map a name that would run over another, or over a node, waits
   * for the pointer or the focus. */
  named: boolean
}

/** Where a node was put by hand, by its kind and name ("branch:topic"). */
export type Pins = ReadonlyMap<string, { x: number; y: number }>

/** A node's key among the pins. A branch and a tag can share a name. */
export function pinKey(n: Pick<MapNode, 'kind' | 'name'>): string {
  return `${n.kind}:${n.name}`
}

/** One ring: how old something is at it, in days, and what it's called. */
export interface RingMark {
  days: number
  name: string
}

/** A zoom level: what the rings mean. */
export interface Scale {
  /** What the whole map spans at this zoom, for the control that changes it. */
  name: string
  rings: RingMark[]
}

/**
 * The zoom levels, closest first. Zoomed all the way in the map is one week and each ring is a
 * day; it goes no closer. Zooming out, the rings become weeks, then months, then years.
 */
export const SCALES: Scale[] = [
  { name: 'A week, day by day', rings: [1, 2, 3, 4, 5, 6, 7].map((d) => ({ days: d, name: d === 1 ? 'today' : d === 7 ? '1 week' : `${d} days` })) },
  { name: 'Two weeks', rings: [{ days: 7, name: 'this week' }, { days: 14, name: 'last week' }] },
  { name: 'Two months', rings: [{ days: 7, name: '1 week' }, { days: 14, name: '2 weeks' }, { days: 30, name: '1 month' }, { days: 61, name: '2 months' }] },
  { name: 'Six months', rings: [{ days: 30, name: '1 month' }, { days: 61, name: '2 months' }, { days: 91, name: '3 months' }, { days: 183, name: '6 months' }] },
  { name: 'Two years', rings: [{ days: 91, name: '3 months' }, { days: 183, name: '6 months' }, { days: 365, name: '1 year' }, { days: 730, name: '2 years' }] },
  { name: 'Ten years', rings: [{ days: 365, name: '1 year' }, { days: 730, name: '2 years' }, { days: 1826, name: '5 years' }, { days: 3652, name: '10 years' }] },
]
/** The zoom the map opens at: this week and last. */
export const DEFAULT_SCALE = 1

/** The zoom level one step in (-1) or out (+1) from `level`, no further out than `widest`. */
export function zoomBy(level: number, step: number, widest = SCALES.length - 1): number {
  return Math.min(widest, Math.max(0, Math.round(level) + step))
}

/** How old each node is, in days. */
const ages = (nodes: Pick<MapNode, 'updated'>[], now: number) => nodes.map((n) => (now - n.updated) / DAY)

/** The furthest out worth zooming: the first level whose last ring holds the oldest node.
 * Past it, everything only crowds the middle. */
export function widestScale(nodes: Pick<MapNode, 'updated'>[], now: number): number {
  const oldest = Math.max(0, ...ages(nodes, now))
  const at = SCALES.findIndex((s) => s.rings[s.rings.length - 1].days >= oldest)
  // Never closer than this week and last: that's where a map opens.
  return at < 0 ? SCALES.length - 1 : Math.max(DEFAULT_SCALE, at)
}

/** The level a map opens at when none has been chosen: the closest that holds most of its
 * nodes (three in five), and no closer than this week and last. */
export function fittingScale(nodes: Pick<MapNode, 'updated'>[], now: number): number {
  const all = ages(nodes, now)
  const at = SCALES.findIndex((s) => all.filter((d) => d <= s.rings[s.rings.length - 1].days).length >= all.length * 0.6)
  return Math.min(widestScale(nodes, now), Math.max(DEFAULT_SCALE, at < 0 ? SCALES.length - 1 : at))
}

export interface Rings {
  cx: number
  cy: number
  /** Each ring, innermost first: its radius and what it marks. */
  marks: (RingMark & { r: number })[]
  /** As far out as a node sits. */
  edge: number
}

const DAY = 24 * 60 * 60
export const TRUNK_R = { min: 44, max: 70 }
export const BRANCH_R = { min: 9, max: 40 }
export const TAG_SIDE = { min: 14, max: 40 }
/** A group's diameter: by how many branches it stands for, not by commits, which aren't
 * counted until it's opened. */
export const GROUP_D = { min: 26, max: 72 }

/** How big a group of this many branches is drawn. */
export function groupSize(branches: number): number {
  return Math.min(GROUP_D.max, Math.max(GROUP_D.min, 6 * Math.sqrt(Math.max(0, branches))))
}

/** Which branches the map shows. */
export type Showing = 'all' | 'local' | 'remote'

/** The nodes to show: all of them, only what's here (local branches and tags), or only what's
 * on the remote alone. The checked-out branch always shows. */
export function showingNodes<T extends Pick<MapNode, 'kind' | 'head'>>(nodes: T[], showing: Showing): T[] {
  if (showing === 'local') return nodes.filter((n) => n.kind !== 'remote' && n.kind !== 'group')
  if (showing === 'remote') return nodes.filter((n) => n.kind !== 'branch' || n.head)
  return nodes
}
/** px of radius per square root of a commit, at most: area carries the number. */
const BRANCH_K = 4
const TAG_K = 4.5
/** Room kept between two nodes for the names under them. */
const GAP = 34
/** About how wide a name is drawn: 12px mono, cut at the width the view cuts it. */
export function nameWidth(name: string): number {
  return Math.min(190, Math.max(64, name.length * 7.3)) + 10
}
/** The two lines of words under a node, and the key in the stage's bottom left corner. */
const NAME_H = 46
export const KEY = { width: 200, height: 124 }
export const ZOOM = { width: 210, height: 52 }
/** Whether the stage has room for the key and the rings' names beside the nodes. */
export function keyShows(width: number, height: number): boolean {
  return width >= 560 && height >= 420
}
/** How many nodes it takes before only the newest keep their names showing. */
export const CROWD = 40

const isTag = (n: Pick<MapNode, 'kind'>) => n.kind === 'tag' || n.kind === 'tags'
const lerp = (a: number, b: number, t: number) => a + (b - a) * Math.min(1, Math.max(0, t))

/** px per square root of a commit for branches and for tags: the mock's scale, brought down
 * when the biggest would outgrow its limit, so sizes stay comparable with each other. */
export function scales(nodes: Pick<MapNode, 'kind' | 'commits'>[]): { branch: number; tag: number } {
  const most = (pick: (n: Pick<MapNode, 'kind'>) => boolean) => Math.max(0, ...nodes.filter(pick).map((n) => n.commits))
  const fit = (k: number, limit: number, biggest: number) => (biggest > 0 ? Math.min(k, limit / Math.sqrt(biggest)) : k)
  return {
    branch: fit(BRANCH_K, BRANCH_R.max, most((n) => n.kind === 'branch' || n.kind === 'remote')),
    // A fold is as big as the history before it: it takes the limit, and doesn't set the scale.
    tag: fit(TAG_K, TAG_SIDE.max, most((n) => n.kind === 'tag')),
  }
}

/** A branch's radius: its area is its commits, with a floor so one commit can be clicked. */
export function branchRadius(commits: number, k: number): number {
  return Math.min(BRANCH_R.max, Math.max(BRANCH_R.min, k * Math.sqrt(commits)))
}

/** A tag's side, on the same footing. */
export function tagSide(commits: number, k: number): number {
  return Math.min(TAG_SIDE.max, Math.max(TAG_SIDE.min, k * Math.sqrt(commits)))
}

/** The trunk's radius: on the branches' scale, within what holds its name and stays the middle. */
export function trunkRadius(commits: number, k: number): number {
  return Math.min(TRUNK_R.max, Math.max(TRUNK_R.min, k * Math.sqrt(commits)))
}

/** Which ring a node sits within, by when it last moved: 0 for the innermost, and the number
 * of rings for something older than all of them. */
export function bandOf(updated: number, now: number, scale: Scale = SCALES[DEFAULT_SCALE]): number {
  const days = (now - updated) / DAY
  const at = scale.rings.findIndex((ring) => days <= ring.days)
  return at < 0 ? scale.rings.length : at
}

/** The rings for a stage this size at this zoom, evenly spaced. The outermost touches the top
 * and bottom; what's older goes out to the sides, where a wide window has the room. */
export function ringsFor(width: number, height: number, scale: Scale = SCALES[DEFAULT_SCALE]): Rings {
  const outer = Math.max(150, height / 2 - 6)
  // The first ring clears the trunk at its biggest, with room for a node between.
  const inner = Math.min(outer * 0.42, TRUNK_R.max + 24)
  const n = scale.rings.length
  return {
    cx: width / 2,
    cy: height / 2,
    marks: scale.rings.map((ring, k) => ({ ...ring, r: inner + ((outer - inner) * (k + 1)) / n })),
    edge: Math.max(outer + 50, width / 2 - 70),
  }
}

/** How far from the trunk's centre a node wants to sit: nearer the newer, and always inside
 * its own ring. */
export function restLength(updated: number, now: number, rings: Rings, trunkR: number, r: number): number {
  const days = Math.max(0, (now - updated) / DAY)
  const near = trunkR + r + GAP + 4
  let from = 0
  let inside = near
  for (const ring of rings.marks) {
    if (days <= ring.days) {
      // A ring too tight for the node lets it sit on the line.
      const out = Math.max(inside, ring.r - r - 12)
      return lerp(inside, out, (days - from) / Math.max(1e-9, ring.days - from))
    }
    from = ring.days
    inside = Math.max(near, ring.r + r + 12)
  }
  // Older than the last ring: just outside it, and further the older, up to ten times as old.
  const last = rings.marks[rings.marks.length - 1]
  const start = Math.max(near, (last?.r ?? 0) + r + 22)
  return lerp(start, Math.max(start, rings.edge), Math.log(days / (last?.days ?? 1)) / Math.log(10))
}

/** The golden angle: successive nodes land as far round from each other as they can. */
const GOLDEN = Math.PI * (3 - Math.sqrt(5))
const STEPS = 260

/**
 * Places the trunk in the middle and every other node around it. Each node is pulled to its
 * rest length from what it hangs off (the trunk, or the branch that holds a tag), pushed off
 * any node it would overlap, and kept on the stage.
 */
export function layout(
  trunk: MapNode,
  nodes: MapNode[],
  width: number,
  height: number,
  now: number,
  scale: Scale = SCALES[DEFAULT_SCALE],
  pins: Pins = new Map(),
): { trunk: Placed; nodes: Placed[]; rings: Rings; nameAngle: number; older: number } {
  const rings = ringsFor(width, height, scale)
  // What's older than the last ring isn't drawn at this zoom: it would only be a long spoke to
  // the edge. It's counted instead, and zooming out brings it in. The checked-out branch and
  // anything pinned always show.
  const all = nodes
  nodes = all.filter((n) => bandOf(n.updated, now, scale) < scale.rings.length || n.head || pins.has(pinKey(n)))
  const older = all.length - nodes.length
  const k = scales(nodes)
  const hub: Placed = { ...trunk, x: rings.cx, y: rings.cy, r: trunkRadius(trunk.commits, k.branch), size: 0, band: 0, above: false, named: true }
  hub.size = hub.r * 2

  const placed: Placed[] = nodes.map((n) => {
    const size = n.kind === 'group' ? groupSize(n.branches ?? 0) : n.kind === 'tags' ? TAG_SIDE.max : isTag(n) ? tagSide(n.commits, k.tag) : branchRadius(n.commits, k.branch) * 2
    return { ...n, x: 0, y: 0, size, r: isTag(n) ? (size * Math.SQRT2) / 2 : size / 2, band: bandOf(n.updated, now, scale), above: false, named: false }
  })
  const byName = new Map(placed.filter((p) => !isTag(p)).map((p) => [p.name, p]))
  const holder = (p: Placed) => (p.parent ? byName.get(p.parent) : undefined)
  // What hangs off a node sits a short spring away from it; the more that hang there, the
  // longer the spring, so an opened group's branches have a ring to stand in.
  const hanging = new Map<string, number>()
  for (const p of placed) if (p.parent && byName.has(p.parent)) hanging.set(p.parent, (hanging.get(p.parent) ?? 0) + 1)
  const rest = (p: Placed) => (holder(p) ? holder(p)!.r + p.r + 46 + 9 * Math.sqrt(Math.max(0, (hanging.get(p.parent) ?? 1) - 1)) : restLength(p.updated, now, rings, hub.r, p.r))

  // Seeded newest first, each a golden angle on from the last, starting up and to the right.
  const order = [...placed].sort((a, b) => b.updated - a.updated || a.name.localeCompare(b.name))
  order.forEach((p, i) => {
    const angle = -0.35 + i * GOLDEN
    const from = holder(p) ?? hub
    const d = rest(p)
    p.x = from.x + Math.cos(angle) * d
    p.y = from.y + Math.sin(angle) * d
  })
  // A node that hangs off a branch was seeded before the branch had its place.
  // Several on one node start spread evenly round it, from the side away from the trunk.
  const turn = new Map<string, number>()
  for (const p of order) {
    const from = holder(p)
    if (!from) continue
    const k = turn.get(p.parent) ?? 0
    turn.set(p.parent, k + 1)
    const out = Math.atan2(from.y - hub.y, from.x - hub.x) + 0.6 + (k * 2 * Math.PI) / (hanging.get(p.parent) ?? 1)
    p.x = from.x + Math.cos(out) * rest(p)
    p.y = from.y + Math.sin(out) * rest(p)
  }
  // A pinned node goes where it was put, kept on the stage; the rest make room for it.
  for (const p of order) {
    const at = pins.get(pinKey(p))
    if (!at) continue
    p.pinned = true
    p.x = Math.min(width - p.r - 4, Math.max(p.r + 4, at.x))
    p.y = Math.min(height - p.r - 4, Math.max(p.r + 4, at.y))
  }

  const keepOn = (p: Placed) => {
    if (p.pinned) return
    p.x = Math.min(width - p.r - 56, Math.max(p.r + 56, p.x))
    // Room for the name on either side: it goes whichever way is away from the trunk.
    p.y = Math.min(height - p.r - NAME_H, Math.max(p.r + NAME_H, p.y))
    // The key sits in the bottom left corner: a node keeps itself and its name off it.
    if (!keyShows(width, height)) return
    const box = boxOf(p)
    if (box.y + box.hh > height - KEY.height && p.x < KEY.width + box.hw) p.x = KEY.width + box.hw
    // And the zoom control in the bottom right.
    if (box.y + box.hh > height - ZOOM.height && p.x > width - ZOOM.width - box.hw) p.x = width - ZOOM.width - box.hw
  }
  // A node takes up its shape and the name beside it: a box as wide as the name, reaching out
  // on the side the name is on. In a crowd there isn't room to keep every name clear: only the
  // checked-out branch, pinned nodes and local branches keep room for theirs, the rest are
  // spaced by their shapes alone, and names are given out afterwards to those with room.
  const crowded = placed.length > CROWD
  const isAbove = (p: Placed) => p.y < hub.y - 24 && !holder(p)
  const boxOf = (p: Placed) => {
    // An opened group's branches stand close round it: their names are given out afterwards,
    // where there's room, and keep none here.
    const member = holder(p)?.kind === 'group'
    const named = !member && (!crowded || p.head || !!p.pinned || p.kind === 'branch' || p.kind === 'group')
    const hw = Math.max(p.r, named ? nameWidth(p.name) / 2 : 0) + 5
    const hh = p.r + (named ? NAME_H / 2 : 0) + 3
    return { x: p.x, y: p.y + (named ? (isAbove(p) ? -NAME_H / 2 : NAME_H / 2) : 0), hw, hh }
  }
  const trunkBox = { ...hub, name: '' }
  /** Moves two overlapping boxes apart along whichever way is shorter. `fixed` holds b still. */
  const part = (a: Placed, b: Placed, salt: number, fixed: boolean) => {
    const ba = boxOf(a)
    const bb = fixed ? { x: b.x, y: b.y, hw: b.r + 4, hh: b.r + 4 } : boxOf(b)
    const dx = bb.x - ba.x
    const dy = bb.y - ba.y
    const ox = ba.hw + bb.hw - Math.abs(dx)
    const oy = ba.hh + bb.hh - Math.abs(dy)
    if (ox <= 0 || oy <= 0) return
    // Two boxes on one spot part by which two they are.
    const sx = dx === 0 ? (salt % 2 ? 1 : -1) : Math.sign(dx)
    const sy = dy === 0 ? (salt % 2 ? 1 : -1) : Math.sign(dy)
    // What's held still (the trunk, a pinned node) doesn't give way: the other moves it all.
    const moveA = !a.pinned
    const moveB = !fixed && !b.pinned
    if (!moveA && !moveB) return
    const share = moveA && moveB ? 0.5 : 1
    if (ox < oy) {
      const push = Math.min(ox, 30) * share
      if (moveA) a.x -= sx * push
      if (moveB) b.x += sx * push
    } else {
      const push = Math.min(oy, 30) * share
      if (moveA) a.y -= sy * push
      if (moveB) b.y += sy * push
    }
  }
  for (let step = 0; step < STEPS; step++) {
    for (const p of order) {
      if (p.pinned) continue
      const from = holder(p) ?? hub
      const dx = p.x - from.x
      const dy = p.y - from.y
      const d = Math.hypot(dx, dy) || 1
      const pull = (rest(p) - d) * 0.14
      p.x += (dx / d) * pull
      p.y += (dy / d) * pull
    }
    for (let i = 0; i < order.length; i++) {
      const a = order[i]
      // The trunk doesn't give way: not to a node, and not to a node's name.
      const off = hub.r + a.r + GAP + 12
      const hx = a.x - hub.x
      const hy = a.y - hub.y
      const hd = Math.hypot(hx, hy) || 1
      if (hd < off && !a.pinned) {
        a.x = hub.x + (hx / hd) * off
        a.y = hub.y + (hy / hd) * off
      }
      part(a, trunkBox, i, true)
      for (let j = i + 1; j < order.length; j++) part(a, order[j], i + j, false)
    }
    for (const p of order) keepOn(p)
  }
  // A node above the trunk wears its name on top, so the name doesn't run across the trunk.
  for (const p of placed) p.above = isAbove(p)

  // Names, in order of who most needs one: the checked-out branch, what's pinned, local
  // branches, then the rest by size. A name that would run over a name already given, or over
  // another node, is left out: it shows when the node is pointed at or has the focus.
  type Box = { x: number; y: number; hw: number; hh: number; of?: Placed }
  const hit = (a: Box, b: Box) => Math.abs(a.x - b.x) < a.hw + b.hw && Math.abs(a.y - b.y) < a.hh + b.hh
  const taken: Box[] = [{ x: hub.x, y: hub.y, hw: hub.r, hh: hub.r }, ...placed.map((p) => ({ x: p.x, y: p.y, hw: p.r, hh: p.r, of: p }))]
  const rank = (p: Placed) => (p.head ? 0 : p.pinned ? 1 : p.kind === 'branch' ? 2 : p.kind === 'group' ? 3 : p.kind === 'remote' ? 4 : 5)
  const wanting = [...placed].sort((a, b) => rank(a) - rank(b) || b.commits - a.commits || b.updated - a.updated || a.name.localeCompare(b.name))
  for (const p of wanting) {
    const off = p.r + 4 + NAME_H / 2
    const label: Box = { x: p.x, y: p.above ? p.y - off : p.y + off, hw: nameWidth(p.name) / 2, hh: NAME_H / 2 - 2 }
    // The checked-out branch is always named: it's where you are.
    if (!p.head && taken.some((t) => t.of !== p && hit(label, t))) continue
    p.named = true
    taken.push(label)
  }
  return { trunk: hub, nodes: placed, rings, nameAngle: ringNameAngle(rings, placed.map(boxOf), width, height), older }
}

/** The directions the rings' names can run out along, in degrees from "right", clockwise:
 * the upper right first, as a map's contour labels usually sit, then round the clock. */
const NAME_ANGLES = [-50, -35, -65, -20, -80, -130, -145, -115, -160, 50, 35, 65, 130, 145, 115, 20, 160, -100, 100]

/**
 * The direction the rings' names run out along: the one whose names are furthest from any
 * node or node's name, among those that keep every name on the stage. Boxes are what nodes
 * take up, as centre and half-size.
 */
export function ringNameAngle(rings: Rings, boxes: { x: number; y: number; hw: number; hh: number }[], width: number, height: number): number {
  let best = NAME_ANGLES[0]
  let bestClear = -Infinity
  for (const deg of NAME_ANGLES) {
    const a = (deg * Math.PI) / 180
    let clear = Infinity
    let onStage = true
    for (const ring of rings.marks) {
      const x = rings.cx + Math.cos(a) * ring.r
      const y = rings.cy + Math.sin(a) * ring.r
      if (x < 40 || x > width - 40 || y < 12 || y > height - 12) onStage = false
      for (const b of boxes) {
        // How far the name's middle is from the box, edge to point; inside is negative.
        const dx = Math.abs(x - b.x) - b.hw - RING_NAME.hw
        const dy = Math.abs(y - b.y) - b.hh - RING_NAME.hh
        clear = Math.min(clear, Math.max(dx, dy))
      }
    }
    if (!onStage) continue
    // The first direction that's clear enough wins: the names don't hop about for a pixel.
    if (clear >= 10) return deg
    if (clear > bestClear) {
      bestClear = clear
      best = deg
    }
  }
  return best
}
/** Half the size a ring's name takes, for keeping nodes off it. */
const RING_NAME = { hw: 34, hh: 10 }


/** Where the working tree's mark goes: touching the checked-out node, on the side away from
 * the trunk (up and to the left of the trunk itself), and kept on the stage. */
export function wipPlace(head: Pick<Placed, 'x' | 'y' | 'r'>, hub: Pick<Placed, 'x' | 'y'>, width: number, height: number): { x: number; y: number } {
  const dx = head.x - hub.x
  const dy = head.y - hub.y
  const d = Math.hypot(dx, dy)
  const ux = d < 1 ? -Math.SQRT1_2 : dx / d
  const uy = d < 1 ? -Math.SQRT1_2 : dy / d
  const reach = head.r + WIP_R + 3
  return { x: Math.min(width - WIP_R - 4, Math.max(WIP_R + 4, head.x + ux * reach)), y: Math.min(height - WIP_R - 4, Math.max(WIP_R + 4, head.y + uy * reach)) }
}
/** The working tree mark's radius. */
export const WIP_R = 11

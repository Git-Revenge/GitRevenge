// The Map view's layout: one node per branch and tag, held around the trunk by springs. A
// node's size is its commits and its distance from the trunk is how long since it moved. The
// forces are run to rest here, the same way every time, so the picture doesn't jump about
// between two looks at the same repository.

/** A node as the backend sends it (app.MapNode in Go). */
export interface MapNode {
  name: string
  kind: 'trunk' | 'branch' | 'remote' | 'tag' | 'tags'
  commit: string
  commits: number
  /** When it last moved, in Unix seconds. */
  updated: number
  /** The branch node it hangs off, or '' for the trunk. */
  parent: string
  head: boolean
  branchKind?: string
  /** How many tags a fold holds. */
  tags?: number
}

export interface Placed extends MapNode {
  x: number
  y: number
  /** How far the node reaches from its centre: a branch's radius, half a tag's diagonal. */
  r: number
  /** What's drawn: a branch's diameter, a tag's side. */
  size: number
  /** 0 moved this week, 1 the week before, 2 longer ago. */
  band: 0 | 1 | 2
}

export interface Rings {
  cx: number
  cy: number
  /** The radius of the one-week ring and of the two-week ring. */
  week: number
  fortnight: number
  /** As far out as a node sits. */
  edge: number
}

const DAY = 24 * 60 * 60
export const TRUNK_R = { min: 44, max: 70 }
export const BRANCH_R = { min: 9, max: 40 }
export const TAG_SIDE = { min: 14, max: 40 }
/** px of radius per square root of a commit, at most: area carries the number. */
const BRANCH_K = 4
const TAG_K = 4.5
/** Room kept between two nodes for the names under them. */
const GAP = 34
/** The two lines of words under a node, and the key in the stage's bottom left corner. */
const NAME_H = 46
export const KEY = { width: 200, height: 124 }
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

/** Which ring a node sits within, by when it last moved. */
export function bandOf(updated: number, now: number): 0 | 1 | 2 {
  const days = (now - updated) / DAY
  return days <= 7 ? 0 : days <= 14 ? 1 : 2
}

/** The rings for a stage this size. The two-week ring touches the top and bottom; what's older
 * goes out to the sides, where a wide window has the room. */
export function ringsFor(width: number, height: number): Rings {
  const fortnight = Math.max(150, height / 2 - 6)
  return { cx: width / 2, cy: height / 2, week: fortnight * 0.67, fortnight, edge: Math.max(fortnight + 50, width / 2 - 70) }
}

/** How far from the trunk's centre a node wants to sit: nearer the newer, and always inside
 * its own ring. */
export function restLength(updated: number, now: number, rings: Rings, trunkR: number, r: number): number {
  const days = Math.max(0, (now - updated) / DAY)
  const near = trunkR + r + GAP + 4
  if (days <= 7) return lerp(near, Math.max(near, rings.week - r - 14), days / 7)
  if (days <= 14) return lerp(rings.week + r + 14, Math.max(rings.week + r + 14, rings.fortnight - r - 14), (days - 7) / 7)
  // A year out is as far as it goes; the months in between bunch up, as they do in memory.
  return lerp(rings.fortnight + r + 22, Math.max(rings.fortnight + r + 22, rings.edge), Math.log(days / 14) / Math.log(26))
}

/** The golden angle: successive nodes land as far round from each other as they can. */
const GOLDEN = Math.PI * (3 - Math.sqrt(5))
const STEPS = 260

/**
 * Places the trunk in the middle and every other node around it. Each node is pulled to its
 * rest length from what it hangs off (the trunk, or the branch that holds a tag), pushed off
 * any node it would overlap, and kept on the stage.
 */
export function layout(trunk: MapNode, nodes: MapNode[], width: number, height: number, now: number): { trunk: Placed; nodes: Placed[]; rings: Rings } {
  const rings = ringsFor(width, height)
  const k = scales(nodes)
  const hub: Placed = { ...trunk, x: rings.cx, y: rings.cy, r: trunkRadius(trunk.commits, k.branch), size: 0, band: 0 }
  hub.size = hub.r * 2

  const placed: Placed[] = nodes.map((n) => {
    const size = n.kind === 'tags' ? TAG_SIDE.max : isTag(n) ? tagSide(n.commits, k.tag) : branchRadius(n.commits, k.branch) * 2
    return { ...n, x: 0, y: 0, size, r: isTag(n) ? (size * Math.SQRT2) / 2 : size / 2, band: bandOf(n.updated, now) }
  })
  const byName = new Map(placed.filter((p) => !isTag(p)).map((p) => [p.name, p]))
  const holder = (p: Placed) => (p.parent ? byName.get(p.parent) : undefined)
  const rest = (p: Placed) => (holder(p) ? holder(p)!.r + p.r + 46 : restLength(p.updated, now, rings, hub.r, p.r))

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
  for (const p of order) {
    const from = holder(p)
    if (!from) continue
    const out = Math.atan2(from.y - hub.y, from.x - hub.x) + 0.6
    p.x = from.x + Math.cos(out) * rest(p)
    p.y = from.y + Math.sin(out) * rest(p)
  }

  const keepOn = (p: Placed) => {
    p.x = Math.min(width - p.r - 56, Math.max(p.r + 56, p.x))
    p.y = Math.min(height - p.r - NAME_H, Math.max(p.r + 10, p.y))
    // The key sits in the bottom left corner.
    if (p.y > height - KEY.height - p.r && p.x < KEY.width + p.r) p.x = KEY.width + p.r
  }
  for (let step = 0; step < STEPS; step++) {
    for (const p of order) {
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
      // The trunk doesn't give way.
      const off = hub.r + a.r + GAP
      const hx = a.x - hub.x
      const hy = a.y - hub.y
      const hd = Math.hypot(hx, hy) || 1
      if (hd < off) {
        a.x = hub.x + (hx / hd) * off
        a.y = hub.y + (hy / hd) * off
      }
      for (let j = i + 1; j < order.length; j++) {
        const b = order[j]
        const dx = b.x - a.x
        const dy = b.y - a.y
        const d = Math.hypot(dx, dy) || 0.01
        const apart = a.r + b.r + GAP
        if (d >= apart) continue
        const push = (apart - d) / 2
        // Two nodes on one spot part along a line that depends on which two they are.
        const ux = d < 0.02 ? Math.cos(i + j) : dx / d
        const uy = d < 0.02 ? Math.sin(i + j) : dy / d
        a.x -= ux * push
        a.y -= uy * push
        b.x += ux * push
        b.y += uy * push
      }
    }
    for (const p of order) keepOn(p)
  }
  return { trunk: hub, nodes: placed, rings }
}

/** Whether a node's name waits for a hover or the focus: in a crowd, all but the newest. */
export function nameWaits(node: Pick<Placed, 'band'>, total: number): boolean {
  return total > CROWD && node.band > 0
}

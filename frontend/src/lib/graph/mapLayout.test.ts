import { describe, expect, it } from 'vitest'
import { bandOf, BRANCH_R, branchRadius, CROWD, DEFAULT_SCALE, KEY, keyShows, layout, nameWaits, restLength, ringsFor, scales, SCALES, TAG_SIDE, tagSide, TRUNK_R, trunkRadius, zoomBy, type MapNode } from './mapLayout'

const DAY = 24 * 60 * 60
const NOW = 1_794_480_000
const node = (name: string, over: Partial<MapNode> = {}): MapNode => ({ name, kind: 'branch', commit: name, commits: 1, updated: NOW, parent: '', head: false, ...over })
const trunk = node('main', { kind: 'trunk', commits: 307, head: true })

describe('sizes', () => {
  it('gives a branch an area that follows its commits', () => {
    const k = 4
    // Four times the commits is twice the radius.
    expect(branchRadius(64, k)).toBe(2 * branchRadius(16, k))
    expect(tagSide(64, 4.5)).toBe(2 * tagSide(16, 4.5))
  })

  it('keeps a one-commit branch big enough to click, and the biggest within its limit', () => {
    expect(branchRadius(0, 4)).toBe(BRANCH_R.min)
    expect(branchRadius(1, 4)).toBe(BRANCH_R.min)
    expect(branchRadius(1e6, 4)).toBe(BRANCH_R.max)
    expect(tagSide(0, 4.5)).toBe(TAG_SIDE.min)
    expect(trunkRadius(3, 4)).toBe(TRUNK_R.min)
    expect(trunkRadius(1e6, 4)).toBe(TRUNK_R.max)
  })

  it('brings the scale down when the biggest branch would outgrow the limit', () => {
    expect(scales([node('a', { commits: 65 }), node('b', { commits: 9 })]).branch).toBe(4)
    const big = scales([node('a', { commits: 10000 }), node('b', { commits: 100 })])
    expect(branchRadius(10000, big.branch)).toBeCloseTo(BRANCH_R.max)
    // Still comparable with each other: a hundredth the commits, a tenth the radius.
    expect(big.branch * Math.sqrt(100)).toBeCloseTo(BRANCH_R.max / 10)
  })

  it('does not let a fold of old tags set the tags’ scale', () => {
    const k = scales([node('v2', { kind: 'tag', commits: 20 }), node('old', { kind: 'tags', commits: 50000, tags: 9 })])
    expect(k.tag).toBe(4.5)
  })
})

describe('distance', () => {
  const rings = ringsFor(880, 580)
  const [week, fortnight] = rings.marks

  it('sorts nodes into this week, the week before, and older', () => {
    expect(bandOf(NOW - 2 * DAY, NOW)).toBe(0)
    expect(bandOf(NOW - 7 * DAY, NOW)).toBe(0)
    expect(bandOf(NOW - 10 * DAY, NOW)).toBe(1)
    expect(bandOf(NOW - 40 * DAY, NOW)).toBe(2)
  })

  it('draws the outermost ring to the top and bottom of the stage, and the others inside it', () => {
    expect(rings.cx).toBe(440)
    expect(rings.cy).toBe(290)
    expect(rings.marks.map((m) => m.name)).toEqual(['this week', 'last week'])
    expect(fortnight.r).toBe(284)
    expect(week.r).toBeLessThan(fortnight.r)
    expect(rings.edge).toBeGreaterThan(fortnight.r)
  })

  it('sits a node further out the longer it has been still, each inside its own ring', () => {
    const at = (days: number) => restLength(NOW - days * DAY, NOW, rings, 70, 9)
    expect(at(0)).toBeLessThan(at(3))
    expect(at(3)).toBeLessThan(at(6))
    expect(at(6)).toBeLessThan(week.r)
    expect(at(8)).toBeGreaterThan(week.r)
    expect(at(13)).toBeLessThan(fortnight.r)
    expect(at(20)).toBeGreaterThan(fortnight.r)
    expect(at(20)).toBeLessThan(at(100))
    expect(at(5000)).toBeLessThanOrEqual(rings.edge)
  })

  it('never puts a node inside the trunk', () => {
    expect(restLength(NOW, NOW, rings, 70, 30)).toBeGreaterThan(70 + 30)
  })
})

describe('zoom', () => {
  it('goes no closer than one week with a ring for each day', () => {
    expect(zoomBy(0, -1)).toBe(0)
    expect(SCALES[0].rings.map((r) => r.days)).toEqual([1, 2, 3, 4, 5, 6, 7])
    expect(SCALES[0].rings[0].name).toBe('today')
    expect(SCALES[0].rings[6].name).toBe('1 week')
  })

  it('opens on this week and last, and steps out to months and then years', () => {
    expect(SCALES[DEFAULT_SCALE].rings.map((r) => r.name)).toEqual(['this week', 'last week'])
    expect(zoomBy(DEFAULT_SCALE, 1)).toBe(DEFAULT_SCALE + 1)
    expect(SCALES[2].rings.map((r) => r.name)).toContain('2 months')
    expect(SCALES[SCALES.length - 1].rings.map((r) => r.name)).toContain('10 years')
    expect(zoomBy(SCALES.length - 1, 1)).toBe(SCALES.length - 1)
  })

  it('reaches further back at each level out', () => {
    const reach = SCALES.map((s) => s.rings[s.rings.length - 1].days)
    for (let k = 1; k < reach.length; k++) expect(reach[k]).toBeGreaterThan(reach[k - 1])
    for (const s of SCALES) for (let k = 1; k < s.rings.length; k++) expect(s.rings[k].days).toBeGreaterThan(s.rings[k - 1].days)
  })

  it('spaces a level’s rings evenly, the last at the stage’s top and bottom', () => {
    const days = ringsFor(880, 580, SCALES[0])
    expect(days.marks).toHaveLength(7)
    expect(days.marks[6].r).toBe(284)
    const gaps = days.marks.slice(1).map((m, k) => m.r - days.marks[k].r)
    for (const g of gaps) expect(g).toBeCloseTo(gaps[0])
  })

  it('brings a month-old branch inside the rings once zoomed out to months', () => {
    const old = NOW - 40 * DAY
    expect(bandOf(old, NOW, SCALES[1])).toBe(2)
    expect(bandOf(old, NOW, SCALES[2])).toBe(3)
    const near = ringsFor(880, 580, SCALES[1])
    const far = ringsFor(880, 580, SCALES[2])
    expect(restLength(old, NOW, near, 70, 9)).toBeGreaterThan(near.marks[1].r)
    expect(restLength(old, NOW, far, 70, 9)).toBeLessThan(far.marks[3].r)
  })

  it('lays the same nodes out differently at another zoom, and still clear of the trunk', () => {
    const nodes = [node('a', { updated: NOW - DAY }), node('b', { updated: NOW - 3 * DAY }), node('c', { updated: NOW - 5 * DAY })]
    const close = layout(trunk, nodes, 880, 580, NOW, SCALES[0])
    const wide = layout(trunk, nodes, 880, 580, NOW, SCALES[3])
    const far = (out: typeof close, name: string) => {
      const n = out.nodes.find((x) => x.name === name)!
      return Math.hypot(n.x - out.trunk.x, n.y - out.trunk.y)
    }
    // By day, five days is most of the way out; by month, all three huddle near the trunk.
    expect(far(close, 'c')).toBeGreaterThan(far(wide, 'c'))
    expect(far(close, 'a')).toBeLessThan(far(close, 'c'))
    for (const n of wide.nodes) expect(Math.hypot(n.x - wide.trunk.x, n.y - wide.trunk.y)).toBeGreaterThan(wide.trunk.r + n.r)
  })
})

describe('layout', () => {
  const nodes = [
    node('gh-pages', { kind: 'remote', commits: 65, updated: NOW }),
    node('topic', { commits: 1, updated: NOW - DAY }),
    node('release/v0.7', { commits: 56, updated: NOW - 13 * DAY }),
    node('release/v0.6', { commits: 9, updated: NOW - 30 * DAY }),
    node('merged', { commits: 0, updated: NOW - 9 * DAY }),
    node('v0.11.0', { kind: 'tag', commits: 58, updated: NOW - DAY }),
    node('v0.7.0', { kind: 'tag', commits: 47, updated: NOW - 13 * DAY, parent: 'release/v0.7' }),
    node('6 older tags', { kind: 'tags', commits: 72, updated: NOW - 14 * DAY, tags: 6 }),
  ]
  const W = 880
  const H = 580
  const out = layout(trunk, nodes, W, H, NOW)
  const find = (name: string) => out.nodes.find((n) => n.name === name)!
  const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y)

  it('puts the trunk in the middle', () => {
    expect(out.trunk.x).toBe(W / 2)
    expect(out.trunk.y).toBe(H / 2)
    expect(out.trunk.r).toBe(TRUNK_R.max)
  })

  it('keeps every node on the stage, clear of the trunk and of each other', () => {
    for (const n of out.nodes) {
      expect(n.x - n.r).toBeGreaterThanOrEqual(0)
      expect(n.x + n.r).toBeLessThanOrEqual(W)
      expect(n.y - n.r).toBeGreaterThanOrEqual(0)
      expect(n.y + n.r).toBeLessThanOrEqual(H)
      expect(dist(n, out.trunk)).toBeGreaterThan(out.trunk.r + n.r)
    }
    for (const a of out.nodes) for (const b of out.nodes) if (a !== b) expect(dist(a, b)).toBeGreaterThan(a.r + b.r)
  })

  it('keeps what moved today nearer the trunk than what has sat for a month', () => {
    expect(dist(find('topic'), out.trunk)).toBeLessThan(dist(find('release/v0.6'), out.trunk))
    expect(dist(find('gh-pages'), out.trunk)).toBeLessThan(dist(find('merged'), out.trunk))
  })

  it('hangs a tag off the branch that holds it', () => {
    const tag = find('v0.7.0')
    const branch = find('release/v0.7')
    expect(dist(tag, branch)).toBeLessThan(dist(tag, out.trunk))
    expect(dist(tag, branch)).toBeLessThan(140)
  })

  it('sizes each kind by its commits', () => {
    expect(find('gh-pages').size).toBeGreaterThan(find('release/v0.6').size)
    expect(find('merged').size).toBe(BRANCH_R.min * 2)
    expect(find('6 older tags').size).toBe(TAG_SIDE.max)
    expect(find('v0.11.0').size).toBeGreaterThan(find('v0.7.0').size)
  })

  it('gives names room: more between nodes side by side than one above the other', () => {
    for (const a of out.nodes)
      for (const b of out.nodes) {
        if (a === b) continue
        const dx = Math.abs(a.x - b.x)
        const dy = Math.abs(a.y - b.y)
        // On much the same level, they're at least a short name apart.
        if (dy < 20) expect(dx).toBeGreaterThan(64)
      }
  })

  it('puts a name over a node that sits above the trunk, and under one below it', () => {
    for (const n of out.nodes) {
      if (n.parent) continue
      if (n.y < out.trunk.y - 30) expect(n.above).toBe(true)
      if (n.y > out.trunk.y) expect(n.above).toBe(false)
    }
    expect(out.nodes.some((n) => n.above)).toBe(true)
  })

  it('keeps nodes and their names off the key in the corner', () => {
    const many = Array.from({ length: 24 }, (_, i) => node(`branch-number-${i}`, { updated: NOW - (i % 20) * DAY, commits: 1 + i }))
    const crowd = layout(trunk, many, W, H, NOW)
    for (const n of crowd.nodes) {
      const low = n.y + n.r > H - KEY.height
      if (low) expect(n.x - n.r).toBeGreaterThanOrEqual(KEY.width)
    }
  })

  it('leaves the key out of a stage too small for it', () => {
    expect(keyShows(880, 580)).toBe(true)
    expect(keyShows(280, 520)).toBe(false)
    expect(keyShows(900, 300)).toBe(false)
  })

  it('comes out the same every time', () => {
    expect(layout(trunk, nodes, W, H, NOW)).toEqual(out)
  })

  it('copes with a crowd on one spot', () => {
    const many = Array.from({ length: 60 }, (_, i) => node(`b${i}`, { updated: NOW - (i % 3) * DAY }))
    const crowd = layout(trunk, many, 1200, 700, NOW)
    for (const n of crowd.nodes) {
      expect(Number.isFinite(n.x) && Number.isFinite(n.y)).toBe(true)
      expect(dist(n, crowd.trunk)).toBeGreaterThan(crowd.trunk.r)
    }
  })
})

describe('names', () => {
  it('all show until the map is crowded, then only this week’s', () => {
    expect(nameWaits({ band: 2 }, CROWD)).toBe(false)
    expect(nameWaits({ band: 0 }, CROWD + 1)).toBe(false)
    expect(nameWaits({ band: 1 }, CROWD + 1)).toBe(true)
  })
})

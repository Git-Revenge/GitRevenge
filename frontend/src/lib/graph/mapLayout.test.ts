import { describe, expect, it } from 'vitest'
import { bandOf, BRANCH_R, branchRadius, CROWD, layout, nameWaits, restLength, ringsFor, scales, TAG_SIDE, tagSide, TRUNK_R, trunkRadius, type MapNode } from './mapLayout'

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

  it('sorts nodes into this week, the week before, and older', () => {
    expect(bandOf(NOW - 2 * DAY, NOW)).toBe(0)
    expect(bandOf(NOW - 7 * DAY, NOW)).toBe(0)
    expect(bandOf(NOW - 10 * DAY, NOW)).toBe(1)
    expect(bandOf(NOW - 40 * DAY, NOW)).toBe(2)
  })

  it('draws the two-week ring to the top and bottom of the stage, and the week ring inside it', () => {
    expect(rings.cx).toBe(440)
    expect(rings.cy).toBe(290)
    expect(rings.fortnight).toBe(284)
    expect(rings.week).toBeLessThan(rings.fortnight)
    expect(rings.edge).toBeGreaterThan(rings.fortnight)
  })

  it('sits a node further out the longer it has been still, each inside its own ring', () => {
    const at = (days: number) => restLength(NOW - days * DAY, NOW, rings, 70, 9)
    expect(at(0)).toBeLessThan(at(3))
    expect(at(3)).toBeLessThan(at(6))
    expect(at(6)).toBeLessThan(rings.week)
    expect(at(8)).toBeGreaterThan(rings.week)
    expect(at(13)).toBeLessThan(rings.fortnight)
    expect(at(20)).toBeGreaterThan(rings.fortnight)
    expect(at(20)).toBeLessThan(at(200))
    expect(at(5000)).toBeLessThanOrEqual(rings.edge)
  })

  it('never puts a node inside the trunk', () => {
    expect(restLength(NOW, NOW, rings, 70, 30)).toBeGreaterThan(70 + 30)
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

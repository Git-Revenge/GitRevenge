import { describe, expect, it } from 'vitest'
import {
  AUTHORS_SCALE,
  authorColor,
  authorNode,
  authorsScale,
  authorsSummary,
  BRANCH_R,
  controlBoxes,
  layout,
  MAX_AUTHORS,
  pinKey,
  SCALES,
  shownAuthors,
  spanDays,
  SWITCH,
  type MapAuthor,
  type MapNode,
} from './mapLayout'

// The Map's Authors mode (#816): the parts of the layout that are its own.
const DAY = 24 * 60 * 60
const NOW = 1_794_480_000
const node = (name: string, over: Partial<MapNode> = {}): MapNode => ({ name, kind: 'branch', commit: name, commits: 1, updated: NOW, parent: '', head: false, ...over })
const trunk = node('main', { kind: 'trunk', commits: 307, head: true })
const author = (name: string, commits: number, over: Partial<MapAuthor> = {}): MapAuthor => ({
  key: `${name.toLowerCase()}@example.com`,
  name,
  email: `${name.toLowerCase()}@example.com`,
  identities: [{ name, email: `${name.toLowerCase()}@example.com`, commits }],
  commits,
  authored: commits,
  coAuthored: 0,
  updated: NOW - DAY,
  bot: false,
  you: false,
  trunk: commits,
  branches: [],
  moreBranches: 0,
  ...over,
})

describe('an author as a node', () => {
  it('is sized by commits, with you as where you are', () => {
    const n = authorNode(author('Maya', 38, { you: true, updated: NOW - 3 * DAY }))
    expect(n).toMatchObject({ kind: 'author', name: 'Maya', id: 'maya@example.com', commits: 38, updated: NOW - 3 * DAY, parent: '', head: true, bot: false })
    expect(pinKey(n)).toBe('author:maya@example.com')
  })

  it('takes the same colour each time, whatever the case of the name', () => {
    expect(authorColor('Maya Okafor', 5)).toBe(authorColor('maya okafor', 5))
    for (const name of ['Maya', 'Leo', 'Priya', '', 'dependabot[bot]']) {
      expect(authorColor(name, 5)).toBeGreaterThanOrEqual(0)
      expect(authorColor(name, 5)).toBeLessThan(5)
    }
  })
})

describe('who shows', () => {
  it('is people, bots or everyone, as the switch says', () => {
    const all = [author('Maya', 40), author('renovate', 30, { bot: true }), author('Leo', 20), author('Priya', 2, { you: true })]
    const names = (who: 'everyone' | 'people' | 'bots') => shownAuthors(all, who).map((a) => a.name)
    expect(names('everyone')).toEqual(['Maya', 'renovate', 'Leo', 'Priya'])
    expect(names('people')).toEqual(['Maya', 'Leo', 'Priya'])
    expect(names('bots')).toEqual(['renovate'])
  })

  it('is no more than the limit, and always you', () => {
    const many = Array.from({ length: MAX_AUTHORS + 20 }, (_, i) => author(`P${i}`, 500 - i))
    many.push(author('Me', 1, { you: true }))
    const got = shownAuthors(many, 'people')
    expect(got).toHaveLength(MAX_AUTHORS + 1)
    expect(got[got.length - 1].name).toBe('Me')
    expect(got[MAX_AUTHORS - 1].name).toBe(`P${MAX_AUTHORS - 1}`)
  })

  it('is summed up for the bar, with how many when some are left out', () => {
    expect(authorsSummary(14, 2, 14)).toBe('14 people · 2 bots')
    expect(authorsSummary(1, 0, 1)).toBe('1 person')
    expect(authorsSummary(10339, 1, 60)).toBe(`60 of ${(10339).toLocaleString()} people · 1 bot`)
    expect(authorsSummary(0, 0, 0)).toBe('0 people')
  })
})

describe('the span of time', () => {
  it('is the days out to a zoom level’s last ring', () => {
    expect(spanDays(0)).toBe(7)
    expect(spanDays(1)).toBe(14)
    expect(spanDays(AUTHORS_SCALE)).toBe(61)
    expect(spanDays(SCALES.length - 1)).toBe(3652)
    for (const s of SCALES) expect(s.period, s.name).toMatch(/^the last /)
  })

  it('opens at two months, or as far out as the trunk last moved', () => {
    expect(authorsScale(NOW, NOW)).toBe(AUTHORS_SCALE)
    expect(authorsScale(NOW - 40 * DAY, NOW)).toBe(AUTHORS_SCALE)
    expect(SCALES[authorsScale(NOW - 100 * DAY, NOW)].name).toBe('Six months')
    expect(SCALES[authorsScale(NOW - 400 * DAY, NOW)].name).toBe('Two years')
    // Older than every ring: all the way out.
    expect(authorsScale(NOW - 9000 * DAY, NOW)).toBe(SCALES.length - 1)
  })
})

describe('authors laid out', () => {
  it('are on the branches’ scale: four times the commits, twice the radius', () => {
    const nodes = [authorNode(author('Maya', 64)), authorNode(author('Leo', 16)), authorNode(author('Priya', 1))]
    const out = layout(trunk, nodes, 1000, 640, NOW, SCALES[AUTHORS_SCALE])
    const r = (name: string) => out.nodes.find((n) => n.name === name)!.r
    expect(r('Maya')).toBeCloseTo(2 * r('Leo'))
    expect(r('Priya')).toBe(BRANCH_R.min)
    // The biggest sets the scale, so it never outgrows its limit.
    const big = layout(trunk, [authorNode(author('Maya', 7842)), authorNode(author('Leo', 1960))], 1000, 640, NOW, SCALES[AUTHORS_SCALE])
    expect(big.nodes.find((n) => n.name === 'Maya')!.r).toBeCloseTo(BRANCH_R.max)
    expect(big.nodes.find((n) => n.name === 'Leo')!.r).toBeCloseTo(BRANCH_R.max / 2, 0)
  })

  it('always draw and name you, and name the rest by who wrote most', () => {
    // A few people write most of the commits, as in any repository: a long tail of small nodes.
    const crowd = Array.from({ length: 50 }, (_, i) => authorNode(author(`Person ${i}`, Math.ceil(400 / (i + 1)), { updated: NOW - i * DAY })))
    const me = authorNode(author('Me', 1, { you: true, updated: NOW - 55 * DAY }))
    const out = layout(trunk, [...crowd, me], 1200, 760, NOW, SCALES[AUTHORS_SCALE])
    expect(out.nodes.find((n) => n.name === 'Me')!.named).toBe(true)
    // In a crowd, the ones who wrote most keep room for their names.
    for (const name of ['Person 0', 'Person 1', 'Person 2']) expect(out.nodes.find((n) => n.name === name)!.named, name).toBe(true)
    // You stay on the map when your last commit is older than the last ring.
    const old = layout(trunk, [authorNode(author('Me', 1, { you: true, updated: NOW - 200 * DAY }))], 1200, 760, NOW, SCALES[AUTHORS_SCALE])
    expect(old.nodes.map((n) => n.name)).toEqual(['Me'])
  })

  it('hang a node off the node with its parent’s id, so two authors can each hold a branch of one name', () => {
    const maya = authorNode(author('Maya', 30))
    const leo = authorNode(author('Leo', 20))
    const child = (of: MapNode, commits: number) => node('main', { id: `${of.id}/main`, parent: of.id!, commits })
    const out = layout(trunk, [maya, leo, child(maya, 25), child(leo, 15)], 1200, 760, NOW, SCALES[AUTHORS_SCALE])
    const at = (id: string) => out.nodes.find((n) => n.id === id)!
    const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y)
    expect(new Set(out.nodes.map(pinKey)).size).toBe(4)
    // Each child sits by its own author, nearer to them than to the other.
    expect(dist(at('maya@example.com/main'), at('maya@example.com'))).toBeLessThan(dist(at('maya@example.com/main'), at('leo@example.com')))
    expect(dist(at('leo@example.com/main'), at('leo@example.com'))).toBeLessThan(dist(at('leo@example.com/main'), at('maya@example.com')))
  })

  it('keep off switches wider than the usual', () => {
    const wide = { width: 420, height: SWITCH.height }
    const nodes = Array.from({ length: 24 }, (_, i) => authorNode(author(`Person ${i}`, 60 - i, { updated: NOW - i * 2 * DAY })))
    const out = layout(trunk, nodes, 1000, 640, NOW, SCALES[AUTHORS_SCALE], new Map(), wide)
    for (const n of out.nodes) expect(n.x - n.r < wide.width && n.y - n.r < wide.height, n.name).toBe(false)
    expect(controlBoxes(1000, 640, wide)[0]).toEqual({ x: 210, y: SWITCH.height / 2, hw: 210, hh: SWITCH.height / 2 })
  })
})

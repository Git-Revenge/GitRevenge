import { describe, expect, it } from 'vitest'
import {
  areaId,
  AUTHOR_SWITCHES,
  AUTHORS_SCALE,
  authorAreaNodes,
  authorBranchNodes,
  authorColor,
  authorNode,
  sharedKey,
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
  type AuthorArea,
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

// #811: an author opens into the trunk and the unmerged branches holding their commits.
describe('an author opened by branch', () => {
  const topic = node('topic', { commits: 4, updated: NOW - 2 * DAY, head: true, worktree: '/wt', stacked: true, parent: 'base' })
  const far = node('origin/spike', { kind: 'remote', commits: 2, updated: NOW - 5 * DAY })
  const maya = author('Maya', 30, { trunk: 24, branches: [topic, far] })

  it('holds the trunk for what has landed, then each branch, sized by their commits there', () => {
    const got = authorBranchNodes(maya, trunk)
    expect(got.map((n) => [n.name, n.kind, n.commits, !!n.landed])).toEqual([
      ['main', 'branch', 24, true],
      ['topic', 'branch', 4, false],
      ['origin/spike', 'remote', 2, false],
    ])
    for (const n of got) expect(n).toMatchObject({ parent: 'maya@example.com', owner: 'maya@example.com', head: false, stacked: false })
    // Where the branch is checked out is the Branches mode's business.
    expect(got[1].worktree).toBeUndefined()
  })

  it('leaves the trunk out when nothing of theirs has landed', () => {
    expect(authorBranchNodes(author('Leo', 4, { trunk: 0, branches: [topic] }), trunk).map((n) => n.name)).toEqual(['topic'])
    expect(authorBranchNodes(author('Leo', 4, { trunk: 0 }), trunk)).toEqual([])
  })

  it('gives each author their own node for a branch they share, which the two know as the same', () => {
    const leo = author('Leo', 9, { trunk: 5, branches: [{ ...topic, commits: 1 }] })
    const mine = authorBranchNodes(maya, trunk)
    const theirs = authorBranchNodes(leo, trunk)
    expect(new Set([...mine, ...theirs].map(pinKey)).size).toBe(5)
    expect(sharedKey(mine[0])).toBe(sharedKey(theirs[0]))
    expect(sharedKey(mine[1])).toBe(sharedKey(theirs[1]))
    expect(sharedKey(mine[0])).not.toBe(sharedKey(mine[1]))
    // A local branch and a remote one of the same name aren't the same branch, nor is a
    // branch that happens to be called what the trunk is.
    expect(sharedKey({ kind: 'branch', name: 'x', owner: 'a' })).not.toBe(sharedKey({ kind: 'remote', name: 'x', owner: 'a' }))
    expect(sharedKey({ kind: 'branch', name: 'main', owner: 'a', landed: true })).not.toBe(sharedKey({ kind: 'branch', name: 'main', owner: 'a' }))
    // Something nobody holds shares with nothing.
    expect(sharedKey(node('topic'))).toBe('')
  })

  it('stands what they hold round them, nearer to them than the trunk is', () => {
    const leo = author('Leo', 9, { trunk: 5, updated: NOW - 20 * DAY, branches: [{ ...topic, commits: 1 }] })
    const nodes = [authorNode(maya), authorNode(leo), ...authorBranchNodes(maya, trunk), ...authorBranchNodes(leo, trunk)]
    const out = layout(trunk, nodes, 1200, 760, NOW, SCALES[AUTHORS_SCALE])
    const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y)
    for (const who of ['maya@example.com', 'leo@example.com']) {
      const at = out.nodes.find((n) => n.id === who)!
      for (const n of out.nodes.filter((n) => n.owner === who)) {
        expect(dist(n, at), `${n.name} of ${who}`).toBeLessThan(160)
        // Nothing sits on top of its author.
        expect(dist(n, at), `${n.name} of ${who}`).toBeGreaterThanOrEqual(n.r + at.r)
      }
    }
    // Opening doesn't move who's on the map off it.
    for (const n of out.nodes) {
      expect(n.x - n.r, n.name).toBeGreaterThanOrEqual(0)
      expect(n.x + n.r, n.name).toBeLessThanOrEqual(1200)
    }
  })
})

// #812: an author opens into the folders their commits touched.
describe('an author opened by area', () => {
  const area = (path: string, commits: number, over: Partial<AuthorArea> = {}): AuthorArea => ({ path, name: `${path.split('/').pop()}/`, files: false, commits, updated: NOW - DAY, folders: 0, recent: [{ sha: 'c1', subject: 'Work', when: NOW - DAY }], ...over })
  const maya = author('Maya', 30)
  const top = [area('frontend', 24, { folders: 2 }), area('internal', 11), area('', 3, { name: 'top-level files', files: true })]

  it('holds each folder, and the files beside them, sized by their commits there', () => {
    const got = authorAreaNodes(maya, top)
    expect(got.map((n) => [n.name, n.kind, n.commits, n.path, !!n.files, n.folders])).toEqual([
      ['frontend/', 'folder', 24, 'frontend', false, 2],
      ['internal/', 'folder', 11, 'internal', false, 0],
      ['top-level files', 'folder', 3, '', true, 0],
    ])
    for (const n of got) expect(n).toMatchObject({ parent: 'maya@example.com', owner: 'maya@example.com', head: false, updated: NOW - DAY, commit: 'c1' })
    expect(got.map((n) => n.id)).toEqual([areaId('maya@example.com', 'frontend'), areaId('maya@example.com', 'internal'), areaId('maya@example.com', '', true)])
    expect(new Set(got.map(pinKey)).size).toBe(3)
  })

  it('hangs what’s in a folder off the folder', () => {
    const inside = authorAreaNodes(maya, [area('frontend/src', 20), area('frontend', 2, { name: 'files in frontend/', files: true })], 'frontend')
    for (const n of inside) expect(n.parent).toBe(areaId('maya@example.com', 'frontend'))
    // A folder and the files directly in it are two nodes.
    expect(inside[1].id).not.toBe(areaId('maya@example.com', 'frontend'))
  })

  it('knows a folder as the same one under another author, and apart from its own files', () => {
    const leo = author('Leo', 9)
    const [mine] = authorAreaNodes(maya, top)
    const [theirs] = authorAreaNodes(leo, top)
    expect(mine.id).not.toBe(theirs.id)
    expect(sharedKey(mine)).toBe(sharedKey(theirs))
    expect(sharedKey(mine)).toBe('folder:frontend')
    const files = authorAreaNodes(maya, [area('frontend', 2, { files: true })])[0]
    expect(sharedKey(files)).not.toBe(sharedKey(mine))
  })

  it('takes what an author holds off the map with them, folders in folders too', () => {
    // An author whose last commit is older than the last ring isn't drawn; what they hold
    // would otherwise be drawn as if it hung off the trunk.
    const gone = author('Gone', 12, { updated: NOW - 300 * DAY })
    const inside = authorAreaNodes(gone, [area('frontend/src', 4)], 'frontend')
    const out = layout(trunk, [authorNode(maya), authorNode(gone), ...authorAreaNodes(gone, top), ...inside, ...authorAreaNodes(maya, top)], 1200, 760, NOW, SCALES[AUTHORS_SCALE])
    expect(out.nodes.some((n) => n.owner === 'gone@example.com')).toBe(false)
    expect(out.nodes.filter((n) => n.owner === 'maya@example.com')).toHaveLength(3)
    // Only the author is counted as older than the last ring, not what went with them.
    expect(out.older).toBe(1)
  })

  it('stands a folder’s folders by it, a level further from their author', () => {
    const inside = authorAreaNodes(maya, [area('frontend/src', 20), area('frontend/e2e', 6)], 'frontend')
    const nodes = [authorNode(maya), authorNode(author('Leo', 9, { updated: NOW - 20 * DAY })), ...authorAreaNodes(maya, top), ...inside]
    const out = layout(trunk, nodes, 1200, 760, NOW, SCALES[AUTHORS_SCALE], new Map(), { width: AUTHOR_SWITCHES, height: SWITCH.height })
    const at = (id: string) => out.nodes.find((n) => n.id === id)!
    const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y)
    const fe = at(areaId('maya@example.com', 'frontend'))
    for (const n of out.nodes.filter((n) => n.parent === fe.id)) {
      expect(dist(n, fe), n.name).toBeLessThan(dist(n, at('maya@example.com')))
      expect(dist(n, fe), n.name).toBeGreaterThanOrEqual(n.r + fe.r)
    }
    // A folder is on the branches' scale: more commits, bigger.
    expect(fe.r).toBeGreaterThan(at(areaId('maya@example.com', 'internal')).r)
    // Nothing lands under the wider switches.
    for (const n of out.nodes) expect(n.x - n.r < AUTHOR_SWITCHES && n.y - n.r < SWITCH.height, n.name).toBe(false)
  })
})

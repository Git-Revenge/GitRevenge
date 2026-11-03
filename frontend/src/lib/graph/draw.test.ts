import { describe, expect, it } from 'vitest'
import { clearOf, clip, clippedName, elbow, graphWidth, initials, LANE_COUNT, laneActive, laneColor, laneVar, laneWindow, laneX, layoutColumns, METRICS, refColumnWidth, refLabels, relativeTime, repeatsAuthor, setLaneColors, sortRefs } from './draw'
import type { Ref, Row } from './rowCache'

describe('graph geometry', () => {
  it('spaces lanes evenly in both densities', () => {
    for (const m of Object.values(METRICS)) expect(laneX(1, m) - laneX(0, m)).toBe(m.laneW)
  })

  it('caps the graph column width', () => {
    const m = METRICS.comfortable
    expect(graphWidth(3, 1000, m)).toBeLessThan(graphWidth(10, 1000, m))
    expect(graphWidth(500, 300, m)).toBe(300)
  })

  it('gives the timeline taller rows and drops the compact columns', () => {
    expect(METRICS.comfortable.rowH).toBeGreaterThan(METRICS.compact.rowH)
    const comfy = layoutColumns(1200, 100, 'comfortable')
    expect(comfy.authorW + comfy.dateW).toBe(0)
    const compact = layoutColumns(1200, 100, 'compact')
    expect(compact.authorW).toBeGreaterThan(0)
    expect(compact.dateW).toBeGreaterThan(0)
  })

  it('cycles colors, including for negative indexes', () => {
    const colors = Array.from({ length: LANE_COUNT }, (_, i) => `#00000${i}`)
    setLaneColors(colors)
    expect(laneColor(LANE_COUNT)).toBe(colors[0])
    expect(laneColor(-1)).toBe(colors[LANE_COUNT - 1])
    expect(laneVar(LANE_COUNT + 2)).toBe('var(--lane-2)')
  })
})

describe('columns', () => {
  it('gives labels a column when the window has room, and puts the message after the graph', () => {
    const lay = layoutColumns(1200, 100, 'compact')
    expect(lay.refW).toBe(refColumnWidth(1200))
    expect(lay.refW).toBeGreaterThan(0)
    expect(lay.textX).toBeGreaterThan(lay.refW + lay.graphW)
  })

  it('keeps labels inline in a narrow window', () => {
    const lay = layoutColumns(500, 100, 'compact')
    expect(lay.refW).toBe(0)
    expect(lay.textX).toBeGreaterThan(100)
  })

  it('lays the right-hand columns edge to edge, ending at the window', () => {
    const lay = layoutColumns(1200, 100, 'compact')
    expect(lay.authorX + lay.authorW).toBe(lay.dateX)
    expect(lay.dateX + lay.dateW).toBe(lay.shaX)
    expect(lay.shaX + lay.shaW).toBe(1200)
  })

  it('drops the date before the author as the window narrows', () => {
    expect(layoutColumns(900, 100, 'compact').dateW).toBe(0)
    expect(layoutColumns(900, 100, 'compact').authorW).toBeGreaterThan(0)
    expect(layoutColumns(700, 100, 'compact').authorW).toBe(0)
  })
})

describe('lanes', () => {
  const row = (col: number, author = 'Ana', parents = 1) => ({ col, author, parents }) as Row

  it("calls a lane busy where it holds the row's commit", () => {
    expect(laneActive(2, row(2), {})).toBe(true)
    expect(laneActive(1, row(2), {})).toBe(false)
  })

  it('keeps a lane busy between commits a row either side', () => {
    expect(laneActive(1, row(2), { prev: row(1), next: row(1) })).toBe(true)
    expect(laneActive(1, row(2), { prev: row(1), next: row(3) })).toBe(false)
    expect(laneActive(1, row(2), { prev: row(1) })).toBe(false)
  })

  it("spots an author's run on a lane, across one row of other work", () => {
    expect(repeatsAuthor(row(2), { prev: row(2) })).toBe(true)
    expect(repeatsAuthor(row(2), { prev: row(3), prev2: row(2) })).toBe(true)
  })

  it('breaks a run on another author, a merge, a gap or another lane', () => {
    expect(repeatsAuthor(row(2), { prev: row(2, 'Bo') })).toBe(false)
    expect(repeatsAuthor(row(2), { prev: row(2, 'Ana', 2) })).toBe(false)
    expect(repeatsAuthor(row(2, 'Ana', 2), { prev: row(2) })).toBe(false)
    expect(repeatsAuthor(row(2), { prev: row(3), prev2: row(4) })).toBe(false)
    expect(repeatsAuthor(row(2), {})).toBe(false)
  })
})

describe('elbow', () => {
  it('leaves a node sideways along its row, then turns down', () => {
    const e = elbow(0, 10, 50, 40, true)
    expect(e.before).toEqual([42, 10])
    expect(e.corner).toEqual([50, 10])
    expect(e.after).toEqual([50, 18])
    expect(e.end).toEqual([50, 40])
  })

  it('comes down, then turns in along the row to reach a node', () => {
    const e = elbow(50, 0, 0, 30, false)
    expect(e.before).toEqual([50, 22])
    expect(e.corner).toEqual([50, 30])
    expect(e.after).toEqual([42, 30])
    expect(e.end).toEqual([0, 30])
  })

  it('tightens the corner to fit a short hop', () => {
    const e = elbow(0, 0, 3, 30, true)
    expect(e.before).toEqual([0, 0])
    expect(e.after).toEqual([3, 3])
  })
})

describe('clearOf', () => {
  it('keeps the hue while fading a full hex colour out', () => {
    expect(clearOf('#cb0162')).toBe('#cb016200')
  })

  it('falls back for anything else', () => {
    expect(clearOf('#888')).toBe('transparent')
    expect(clearOf('rgb(1, 2, 3)')).toBe('transparent')
  })
})

describe('refLabels', () => {
  const ref = (short: string, kind: Ref['kind'], head = false) => ({ name: short, short, kind, head, commit: '', upstream: '', ahead: 0, behind: 0, gone: false }) as Ref
  const said = (refs: Ref[]) => refLabels(refs).map((l) => [l.text, l.remote])

  it('swaps origin/ for the remote mark', () => {
    expect(said([ref('origin/feature/x', 'remote')])).toEqual([['feature/x', true]])
  })

  it('folds a local branch and its origin counterpart into one label', () => {
    expect(said([ref('main', 'local', true), ref('origin/main', 'remote')])).toEqual([['main', true]])
  })

  it('leaves a local branch that is not on origin alone', () => {
    expect(said([ref('wip', 'local'), ref('origin/other', 'remote')])).toEqual([
      ['wip', false],
      ['other', true],
    ])
  })

  it('keeps the prefix of other remotes, so they stay apart', () => {
    expect(said([ref('origin/x', 'remote'), ref('fork/x', 'remote')])).toEqual([
      ['x', true],
      ['fork/x', true],
    ])
  })

  it("doesn't fold a local branch into another remote's branch of the same name", () => {
    expect(said([ref('x', 'local'), ref('fork/x', 'remote')])).toEqual([
      ['x', false],
      ['fork/x', true],
    ])
  })

  it('leaves origin/HEAD whole, since "HEAD" alone would mislead', () => {
    expect(said([ref('origin/HEAD', 'remote')])).toEqual([['origin/HEAD', true]])
  })

  it('marks tags and stashes, which are never on a remote', () => {
    expect(said([ref('v1.0', 'tag'), ref('stash@{0}', 'stash')])).toEqual([
      ['◆ v1.0', false],
      ['≡ stash@{0}', false],
    ])
  })
})

describe('clip', () => {
  // One pixel per character.
  const ctx = { measureText: (s: string) => ({ width: s.length }) as TextMetrics }

  it('leaves text that fits alone', () => {
    expect(clip(ctx, 'hello', 10)).toBe('hello')
  })

  it('truncates with an ellipsis', () => {
    const out = clip(ctx, 'hello world', 6)
    expect(out).toBe('hello…')
  })

  it('handles no room at all', () => {
    expect(clip(ctx, 'hello', 0)).toBe('')
  })
})

describe('clippedName', () => {
  // One pixel a character, so room is a count of characters.
  const ctx = { measureText: (s: string) => ({ width: s.length }) as TextMetrics }

  it('keeps a name that fits, however short', () => {
    expect(clippedName(ctx, 'v1.0', 4)).toBe('v1.0')
  })

  it('cuts a name while enough of it is left to read', () => {
    expect(clippedName(ctx, 'origin/feature/offline-cache', 12)).toBe('origin/feat…')
    expect(clippedName(ctx, 'origin/feature/offline-cache', 9)).toBe('origin/f…')
  })

  it('gives up when too little would be left', () => {
    expect(clippedName(ctx, 'origin/feature/offline-cache', 8)).toBeNull()
    expect(clippedName(ctx, 'origin/feature/offline-cache', 0)).toBeNull()
  })

  it("doesn't count a tag's mark as part of its name", () => {
    expect(clippedName(ctx, '◆ a-very-long-tag-name', 9)).toBeNull()
    expect(clippedName(ctx, '◆ a-very-long-tag-name', 11)).toBe('◆ a-very-l…')
  })
})

describe('sortRefs', () => {
  const ref = (short: string, kind: 'local' | 'remote' | 'tag' | 'stash', head = false, branchKind = '') =>
    ({ name: short, short, kind, head, branchKind, commit: '', upstream: '', ahead: 0, behind: 0, gone: false }) as const
  const order = (refs: Ref[]) => sortRefs(refs).map((r) => r.short)

  it('ranks by what matters most', () => {
    const refs = [
      ref('stash@{0}', 'stash'),
      ref('dependabot/x', 'local', false, 'bot'),
      ref('gh-pages', 'local', false, 'docs'),
      ref('dev', 'local'),
      ref('feature/a', 'local', false, 'feature'),
      ref('staging', 'local', false, 'environment'),
      ref('v1', 'tag'),
      ref('release/1', 'local', false, 'release'),
      ref('main', 'local', false, 'trunk'),
      ref('wip', 'local', true),
    ]
    expect(order(refs)).toEqual(['wip', 'main', 'release/1', 'v1', 'staging', 'feature/a', 'dev', 'dependabot/x', 'gh-pages', 'stash@{0}'])
  })

  it("doesn't let a long run of ordinary branches push Trunk or a tag out", () => {
    const refs = [ref('a', 'local'), ref('b', 'local'), ref('v1', 'tag'), ref('origin/main', 'remote', false, 'trunk')]
    expect(order(refs).slice(0, 2)).toEqual(['origin/main', 'v1'])
  })

  it('puts local before remote at the same rank, and otherwise keeps the order given', () => {
    expect(order([ref('origin/b', 'remote'), ref('origin/a', 'remote'), ref('c', 'local')])).toEqual(['c', 'origin/b', 'origin/a'])
  })
})

describe('relativeTime', () => {
  const now = Date.UTC(2026, 9, 29, 12) / 1000

  it('counts minutes, hours and days, then shows a date', () => {
    expect(relativeTime(now - 30, now)).toBe('now')
    expect(relativeTime(now - 14 * 60, now)).toBe('14m ago')
    expect(relativeTime(now - 5 * 3600, now)).toBe('5h ago')
    expect(relativeTime(now - 3 * 86400, now)).toBe('3d ago')
    expect(relativeTime(now - 30 * 86400, now)).not.toMatch(/ago|20\d\d/)
    expect(relativeTime(Date.UTC(2024, 0, 15) / 1000, now)).toMatch(/2024/)
  })
})

describe('initials', () => {
  it('takes the first and last word', () => {
    expect(initials('Ris Adams')).toBe('RA')
    expect(initials('Ana María de la Cruz')).toBe('AC')
    expect(initials('dependabot[bot]')).toBe('D')
    expect(initials('jane.doe')).toBe('JD')
    expect(initials('')).toBe('?')
  })
})

describe('laneWindow', () => {
  it('shows from the first lane when the nodes in view fit', () => {
    expect(laneWindow(0, 5, null, 10)).toBe(0)
  })

  it('scrolls right just far enough for the nodes in view', () => {
    expect(laneWindow(40, 45, null, 10)).toBe(36)
    expect(laneWindow(800, 801, null, 10)).toBe(792)
  })

  it('centres the selected node when the nodes in view are wider than the column', () => {
    expect(laneWindow(0, 900, 300, 10)).toBe(295)
    expect(laneWindow(100, 900, null, 10)).toBe(95)
  })
})

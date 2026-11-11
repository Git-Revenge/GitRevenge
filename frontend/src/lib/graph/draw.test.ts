import { describe, expect, it } from 'vitest'
import { clip, DATE_COLUMN, noteMarkSpan, middleCut, fitLabelColumn, LABEL_COLUMN, labelColumnNeed, moreLabels, elbow, graphWidth, initials, LANE_COUNT, laneActive, laneColor, laneVar, laneWindow, laneX, layoutColumns, mainRemote, METRICS, refColumnWidth, refLabels, SEAL_OFFSET, relativeTime, remoteOf, repeatsAuthor, setLaneColors, sortRefs, selectionBar, hereNotePlace, HERE_NOTE, sameDay, dayLabel, timeLabel, metaLine, bisectRow, startsRun, RUN_PAUSE } from './draw'
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

  // #433: a seal touched the last digit of a date with a year in it.
  it("leaves room for a seal after the longest date", () => {
    const monoCharWidth = 7.3
    const longestDate = 'Sep 17, 2024'.length * monoCharWidth
    const sealRadius = 5.5
    expect(DATE_COLUMN - longestDate - (SEAL_OFFSET + sealRadius)).toBeGreaterThanOrEqual(8)
  })

  // #620: a note's mark has a slot of its own, left of the seal's.
  it("leaves room for a note's mark after the longest date, clear of the seal", () => {
    const monoCharWidth = 7.3
    const longestDate = 'Sep 17, 2024'.length * monoCharWidth
    const lay = layoutColumns(1200, 100, 'compact')
    const [from, to] = noteMarkSpan(lay)
    expect(from - (lay.dateX + longestDate)).toBeGreaterThanOrEqual(8)
    // The slot ends before the seal's ring begins.
    expect(to).toBeLessThanOrEqual(lay.shaX - SEAL_OFFSET - 5.5 + 2.5)
    expect(to - from).toBeGreaterThanOrEqual(16)
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

describe('the label column', () => {
  // One pixel a character, so widths are counts of characters.
  const ctx = { measureText: (s: string) => ({ width: s.length }) as TextMetrics }
  const branch = (short: string) => ({ name: `refs/heads/${short}`, short, kind: 'local', head: false, commit: '', upstream: '', ahead: 0, behind: 0, gone: false }) as Ref

  it('keeps a wanted width within its bounds', () => {
    expect(fitLabelColumn(60)).toBe(LABEL_COLUMN.min)
    expect(fitLabelColumn(180.4)).toBe(180)
    expect(fitLabelColumn(900)).toBe(LABEL_COLUMN.max)
  })

  it('takes the fitted width when the window has room for a column', () => {
    expect(refColumnWidth(1200, 200)).toBe(200)
    expect(refColumnWidth(1200, 900)).toBe(LABEL_COLUMN.max)
    expect(refColumnWidth(500, 200)).toBe(0)
    expect(layoutColumns(1200, 100, 'compact', 0, 'origin', 200).refW).toBe(200)
  })

  it('needs room for the first label, and for a count when there are more', () => {
    expect(labelColumnNeed(ctx, [], 'origin')).toBe(0)
    const one = labelColumnNeed(ctx, [branch('topic')], 'origin')
    // Margins of 8 either side, 14 of padding in the label, and the name.
    expect(one).toBe(8 + 5 + 14 + 8)
    // "+1" and the gap before it.
    expect(labelColumnNeed(ctx, [branch('topic'), branch('other')], 'origin')).toBe(one + 5 + 2)
  })

  it('counts the labels after the first', () => {
    expect(moreLabels(0)).toBe('')
    expect(moreLabels(1)).toBe('')
    expect(moreLabels(3)).toBe('+2')
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


describe('refLabels', () => {
  const ref = (short: string, kind: Ref['kind'], head = false) => ({ name: short, short, kind, head, commit: '', upstream: '', ahead: 0, behind: 0, gone: false }) as Ref
  const said = (refs: Ref[]) => refLabels(refs).map((l) => [l.text, l.remotes])

  it('swaps origin/ for the remote mark', () => {
    expect(said([ref('origin/feature/x', 'remote')])).toEqual([['feature/x', 1]])
  })

  it('folds a local branch and its origin counterpart into one label', () => {
    expect(said([ref('main', 'local', true), ref('origin/main', 'remote')])).toEqual([['main', 1]])
  })

  it('leaves a local branch that is not on origin alone', () => {
    expect(said([ref('wip', 'local'), ref('origin/other', 'remote')])).toEqual([
      ['wip', 0],
      ['other', 1],
    ])
  })

  it('keeps the prefix of a branch on just one other remote', () => {
    expect(said([ref('fork/x', 'remote')])).toEqual([['fork/x', 1]])
  })

  it('folds the same branch across remotes, counting them', () => {
    expect(said([ref('origin/x', 'remote'), ref('fork/x', 'remote'), ref('up/x', 'remote')])).toEqual([['x', 3]])
  })

  it('drops the prefix when several other remotes share the name', () => {
    expect(said([ref('fork/x', 'remote'), ref('up/x', 'remote')])).toEqual([['x', 2]])
  })

  it('folds a local branch with every remote that has the name', () => {
    expect(said([ref('main', 'local', true), ref('origin/main', 'remote'), ref('fork/main', 'remote')])).toEqual([['main', 2]])
  })

  it('counts from the upstream when a branch tracks another name', () => {
    const tracking = { ...ref('topic', 'local'), upstream: 'fork/feature/topic' }
    expect(said([tracking, ref('fork/feature/topic', 'remote'), ref('origin/feature/topic', 'remote')])).toEqual([['topic', 2]])
  })

  it('says where each name lives', () => {
    const places = (refs: Ref[]) => refLabels(refs).map((l) => l.places)
    expect(places([ref('main', 'local'), ref('origin/main', 'remote'), ref('fork/main', 'remote')])).toEqual([['local', 'origin', 'fork']])
    expect(places([ref('wip', 'local')])).toEqual([['local']])
    expect(places([ref('origin/x', 'remote')])).toEqual([['origin']])
    expect(places([ref('v1', 'tag')])).toEqual([[]])
  })

  it('keeps different branches apart', () => {
    expect(said([ref('origin/a', 'remote'), ref('fork/b', 'remote')])).toEqual([
      ['a', 1],
      ['fork/b', 1],
    ])
  })

  it('never folds a remote HEAD', () => {
    expect(said([ref('origin/HEAD', 'remote'), ref('fork/HEAD', 'remote')])).toEqual([
      ['origin/HEAD', 1],
      ['fork/HEAD', 1],
    ])
  })

  it('folds a local branch with the remote it tracks, whatever the remote is called', () => {
    const tracking = { ...ref('x', 'local'), upstream: 'fork/x' }
    expect(said([tracking, ref('fork/x', 'remote')])).toEqual([['x', 1]])
  })

  it('folds a branch with its upstream under a different name', () => {
    const tracking = { ...ref('topic', 'local'), upstream: 'origin/feature/topic' }
    expect(said([tracking, ref('origin/feature/topic', 'remote')])).toEqual([['topic', 1]])
  })

  it("doesn't fold a tracking branch with its namesake on the main remote", () => {
    const tracking = { ...ref('x', 'local'), upstream: 'fork/x' }
    expect(said([tracking, ref('origin/x', 'remote')])).toEqual([
      ['x', 0],
      ['x', 1],
    ])
  })

  it('leaves off the prefix of whichever remote is the main one', () => {
    const labels = refLabels([ref('github/x', 'remote'), ref('origin/y', 'remote')], 'github')
    expect(labels.map((l) => l.text)).toEqual(['x', 'origin/y'])
  })

  it('keeps every prefix when there is no main remote', () => {
    expect(refLabels([ref('a/x', 'remote')], '').map((l) => l.text)).toEqual(['a/x'])
  })

  it("doesn't fold a local branch into another remote's branch of the same name", () => {
    expect(said([ref('x', 'local'), ref('fork/x', 'remote')])).toEqual([
      ['x', 0],
      ['fork/x', 1],
    ])
  })

  it('leaves origin/HEAD whole, since "HEAD" alone would mislead', () => {
    expect(said([ref('origin/HEAD', 'remote')])).toEqual([['origin/HEAD', 1]])
  })

  it('marks tags and stashes, which are never on a remote', () => {
    expect(said([ref('v1.0', 'tag'), ref('stash@{0}', 'stash')])).toEqual([
      ['◆ v1.0', 0],
      ['≡ stash@{0}', 0],
    ])
  })
})

describe('mainRemote', () => {
  const ref = (short: string, kind: Ref['kind'], extra: Partial<Ref> = {}) => ({ name: short, short, kind, head: false, commit: '', upstream: '', ahead: 0, behind: 0, gone: false, ...extra }) as Ref

  it("takes the remote of the checked-out branch's upstream", () => {
    expect(mainRemote([ref('main', 'local', { head: true, upstream: 'github/main' }), ref('origin/main', 'remote')])).toBe('github')
  })

  it('falls back to origin, then to the only remote', () => {
    expect(mainRemote([ref('a/x', 'remote'), ref('origin/x', 'remote')])).toBe('origin')
    expect(mainRemote([ref('main', 'local', { head: true }), ref('github/x', 'remote')])).toBe('github')
  })

  it('has none when it cannot tell', () => {
    expect(mainRemote([ref('a/x', 'remote'), ref('b/x', 'remote')])).toBe('')
    expect(mainRemote([ref('main', 'local', { head: true })])).toBe('')
  })

  it('reads the remote from the front of the name', () => {
    expect(remoteOf({ short: 'fork/feature/x' })).toBe('fork')
    expect(remoteOf({ short: 'main' })).toBe('')
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

describe('middleCut', () => {
  // One pixel a character, so room is a count of characters.
  const ctx = { measureText: (s: string) => ({ width: s.length }) as TextMetrics }

  it('keeps a name that fits, however short', () => {
    expect(middleCut(ctx, 'v1.0', 4)).toBe('v1.0')
  })

  it('cuts the middle out, so names that start alike can still be told apart', () => {
    expect(middleCut(ctx, '617-watch-test-ends', 15)).toBe('617-wat…st-ends')
    expect(middleCut(ctx, '617-watch-test-cleanup', 15)).toBe('617-wat…cleanup')
    expect(middleCut(ctx, 'feature/offline-cache', 16)).toBe('feature/…e-cache')
  })

  it('gives the odd character to the start', () => {
    expect(middleCut(ctx, 'origin/feature/offline-cache', 10)).toBe('origi…ache')
  })

  it('gives up when too little would be left', () => {
    expect(middleCut(ctx, 'origin/feature/offline-cache', 9)).toBe('orig…ache')
    expect(middleCut(ctx, 'origin/feature/offline-cache', 8)).toBeNull()
    expect(middleCut(ctx, 'origin/feature/offline-cache', 0)).toBeNull()
  })

  it("keeps a tag's mark in front, and doesn't count it as part of the name", () => {
    expect(middleCut(ctx, '◆ a-very-long-tag-name', 10)).toBeNull()
    expect(middleCut(ctx, '◆ a-very-long-tag-name', 11)).toBe('◆ a-ve…name')
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

describe('selectionBar', () => {
  it("is a 3px bar at the left edge, inset 4px from the row's top and bottom", () => {
    expect(selectionBar(100, 52)).toEqual({ x: 0, y: 104, w: 3, h: 44, r: 2 })
    expect(selectionBar(0, 30)).toEqual({ x: 0, y: 4, w: 3, h: 22, r: 2 })
  })
})

describe('hereNotePlace', () => {
  it('sits a gap after the labels when the column has room, and stays away otherwise', () => {
    expect(hereNotePlace(40, 60, 150)).toBe(48)
    expect(hereNotePlace(40, 95, 150)).toBeNull()
    expect(hereNotePlace(0, 130, 150)).toBe(8)
    expect(HERE_NOTE).toMatch(/here/)
  })
})

describe('dayLabel', () => {
  const now = new Date(2026, 9, 29, 12).getTime() / 1000

  it('names today and yesterday, then the weekday and date, with the year once it differs', () => {
    expect(dayLabel(now - 3600, now)).toBe('Today')
    expect(dayLabel(now - 86400, now)).toBe('Yesterday')
    expect(dayLabel(new Date(2026, 9, 26, 9).getTime() / 1000, now)).toMatch(/26/)
    expect(dayLabel(new Date(2026, 9, 26, 9).getTime() / 1000, now)).not.toMatch(/2026/)
    expect(dayLabel(new Date(2024, 0, 15).getTime() / 1000, now)).toMatch(/2024/)
  })

  it('tells days apart by the local calendar', () => {
    const late = new Date(2026, 9, 28, 23, 50).getTime() / 1000
    const early = new Date(2026, 9, 29, 0, 10).getTime() / 1000
    expect(sameDay(late, early)).toBe(false)
    expect(sameDay(early, early + 3600)).toBe(true)
    expect(timeLabel(early)).toMatch(/\d/)
  })
})

describe('metaLine', () => {
  const now = new Date(2026, 9, 29, 12).getTime() / 1000
  const day = (d: number, h: number) => new Date(2026, 9, d, h).getTime() / 1000
  const row = (author: string, authorTime: number, col = 0, parents = 1): Row => ({ index: 0, col, color: 0, up: [], down: [], width: 1, id: 'x', parents, author, authorTime, subject: 's' })

  it('names the day, author and time on the first row of a day, without a divider when nothing is above', () => {
    const m = metaLine(row('Ana', day(29, 9)), {}, now)
    expect(m.text).toMatch(/^Today · Ana · /)
    expect(m.divider).toBe(false)
  })

  it('draws the divider when the loaded row above is from another day', () => {
    const m = metaLine(row('Ana', day(28, 18)), { prev: row('Ana', day(29, 9)) }, now)
    expect(m.text).toMatch(/^Yesterday · Ana · /)
    expect(m.divider).toBe(true)
  })

  it('drops the day inside it, and always says who: only a commit that starts a run has this line', () => {
    const prev = row('Ana', day(29, 10))
    const other = metaLine(row('Ben', day(29, 9)), { prev }, now)
    expect(other.text).toMatch(/^Ben · /)
    expect(other.divider).toBe(false)
    expect(metaLine(row('Ana', day(29, 9)), { prev }, now).text).toMatch(/^Ana · /)
  })
})

// #622: a second line only when it has something to say.
describe('startsRun', () => {
  const at = (h: number, min = 0, d = 29) => new Date(2026, 9, d, h, min).getTime() / 1000
  const row = (author: string, authorTime: number, col = 0, parents = 1): Row => ({ index: 0, col, color: 0, up: [], down: [], width: 1, id: 'x', parents, author, authorTime, subject: 's' })

  it('is false for a commit carrying on its author\'s run: same lane, same day, soon after', () => {
    expect(startsRun(row('Ana', at(9)), { prev: row('Ana', at(10)) }, false)).toBe(false)
  })

  it('is true when the author changes on the lane', () => {
    expect(startsRun(row('Ben', at(9)), { prev: row('Ana', at(10)) }, false)).toBe(true)
  })

  it('is true for the first row of a day, whoever is above it', () => {
    expect(startsRun(row('Ana', at(23, 0, 28)), { prev: row('Ana', at(0, 30, 29)) }, false)).toBe(true)
    // Even half an hour apart, across midnight.
    expect(startsRun(row('Ana', at(23, 50, 28)), { prev: row('Ana', at(0, 10, 29)) }, false)).toBe(true)
  })

  it('is true after a pause of two hours, and not a minute before', () => {
    expect(RUN_PAUSE).toBe(7200)
    const prev = row('Ana', at(12))
    expect(startsRun(row('Ana', at(10, 1)), { prev }, false)).toBe(false)
    expect(startsRun(row('Ana', at(10, 0)), { prev }, false)).toBe(true)
    expect(startsRun(row('Ana', at(9, 0)), { prev }, false)).toBe(true)
  })

  it('measures the pause either way, for commits dated out of order', () => {
    expect(startsRun(row('Ana', at(13)), { prev: row('Ana', at(10)) }, false)).toBe(true)
  })

  it('is true for the checked-out commit, whatever is above it', () => {
    expect(startsRun(row('Ana', at(9)), { prev: row('Ana', at(10)) }, true)).toBe(true)
  })

  it('is true for a merge, and for the commit under one', () => {
    expect(startsRun(row('Ana', at(9), 0, 2), { prev: row('Ana', at(10)) }, false)).toBe(true)
    expect(startsRun(row('Ana', at(9)), { prev: row('Ana', at(10), 0, 2) }, false)).toBe(true)
  })

  it('looks past one row on another lane for the commit above on its own', () => {
    const elsewhere = row('Ben', at(10), 1)
    expect(startsRun(row('Ana', at(9)), { prev: elsewhere, prev2: row('Ana', at(10, 30)) }, false)).toBe(false)
    expect(startsRun(row('Ana', at(9)), { prev: elsewhere, prev2: row('Cy', at(10, 30)) }, false)).toBe(true)
    // Nothing on its lane within two rows: it starts one.
    expect(startsRun(row('Ana', at(9)), { prev: elsewhere, prev2: row('Ana', at(10, 30), 2) }, false)).toBe(true)
    expect(startsRun(row('Ana', at(9)), { prev: elsewhere }, false)).toBe(true)
  })

  it('is true until the row above has loaded', () => {
    expect(startsRun(row('Ana', at(9)), {}, false)).toBe(true)
  })
})

describe('bisectRow', () => {
  const colors = { bad: '#800' }
  const b = { terms: { z: 'bad', a: 'good', k: 'skip' }, inQuestion: new Set(['z', 'c', 'm']), current: 'c', found: '' }

  it('marks the commit under test, with its note and the band', () => {
    expect(bisectRow('c', b, colors)).toEqual({ extra: { mark: 'testing', band: true, note: 'testing this one' }, faded: false })
  })

  it('gives each answered commit its mark, and never fades it', () => {
    expect(bisectRow('z', b, colors)).toEqual({ extra: { mark: 'bad' }, faded: false })
    expect(bisectRow('a', b, colors)).toEqual({ extra: { mark: 'good' }, faded: false })
    expect(bisectRow('k', b, colors)).toEqual({ extra: { mark: 'skip' }, faded: false })
  })

  it('bands what is still in question and fades the rest', () => {
    expect(bisectRow('m', b, colors)).toEqual({ extra: { band: true }, faded: false })
    expect(bisectRow('old', b, colors)).toEqual({ extra: {}, faded: true })
  })

  it('fades nothing when the range is not known', () => {
    expect(bisectRow('old', { ...b, inQuestion: null }, colors)).toEqual({ extra: {}, faded: false })
  })

  it('marks the answer once found, and fades everything unanswered', () => {
    const done = { ...b, inQuestion: null, current: '', found: 'm' }
    expect(bisectRow('m', done, colors)).toEqual({ extra: { mark: 'found', note: 'it broke here', noteColor: '#800' }, faded: false })
    expect(bisectRow('old', done, colors)).toEqual({ extra: {}, faded: true })
    expect(bisectRow('a', done, colors).faded).toBe(false)
  })
})

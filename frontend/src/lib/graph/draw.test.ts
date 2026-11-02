import { describe, expect, it } from 'vitest'
import { clip, clippedName, graphWidth, initials, LANE_COUNT, laneColor, laneVar, laneWindow, laneX, layoutColumns, METRICS, relativeTime, setLaneColors, sortRefs } from './draw'

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
  const ref = (short: string, kind: 'local' | 'remote' | 'tag' | 'stash', head = false) =>
    ({ name: short, short, kind, head, commit: '', upstream: '', ahead: 0, behind: 0, gone: false }) as const

  it('puts the checked-out branch first, then local, remote, tag', () => {
    const out = sortRefs([ref('v1', 'tag'), ref('origin/main', 'remote'), ref('dev', 'local'), ref('main', 'local', true)])
    expect(out.map((r) => r.short)).toEqual(['main', 'dev', 'origin/main', 'v1'])
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

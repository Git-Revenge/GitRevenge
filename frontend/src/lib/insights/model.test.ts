import { describe, expect, it } from 'vitest'
import { ago, attention, byDay, byReference, duration, groupLabel, hotFiles, latest, median, perDay, sharedAsOf, tiles, utcDay, utcMoment, type Item, type Ranked, type Tagged } from './model'

const DAY = 86400
const item = (repo: string, sha: string, committed: number, extra: Partial<Item> = {}): Tagged<Item> => ({
  repo,
  value: { sha, title: sha, author: 'A', email: 'a@example.com', committed, refs: [], revert: false, fixup: false, ...extra },
})
const branch = (repo: string, name: string, lastCommit: number, flags: string[], conflicts?: boolean): Ranked => ({
  repo,
  value: { name, sha: name, author: 'A', local: false, lastCommit, flags },
  ...(conflicts === undefined ? {} : { conflict: { repo, targetSha: 't', branch: name, sha: name, conflicts, files: [] } }),
})

describe('attention', () => {
  it('ranks conflicts, then flags, then recency, then repo and name', () => {
    const got = attention([
      branch('r1', 'quiet', 50, []),
      branch('r1', 'two-flags', 10, ['stale', 'drifting']),
      branch('r2', 'conflicts', 5, [], true),
      branch('r1', 'one-flag-new', 40, ['stale']),
      branch('r1', 'one-flag-old', 30, ['stale']),
      branch('r1', 'b-tie', 30, ['stale']),
      branch('r0', 'b-tie', 30, ['stale']),
      branch('r1', 'clean', 60, [], false),
    ]).map((b) => `${b.repo}:${b.value.name}`)
    expect(got).toEqual(['r2:conflicts', 'r1:two-flags', 'r1:one-flag-new', 'r0:b-tie', 'r1:b-tie', 'r1:one-flag-old'])
  })
})

describe('tiles and lists', () => {
  const items = [
    item('r1', 'c', 300, { email: 'A@example.com' }),
    item('r1', 'a', 200, { revert: true }),
    item('r2', 'b', 300, { email: 'b@example.com', fixup: true }),
  ]
  it('counts ships, people by email, attention, conflicts and rework', () => {
    expect(tiles(items, [branch('r1', 'x', 1, ['stale']), branch('r1', 'y', 1, [], true), branch('r1', 'z', 1, [])])).toEqual({
      shipped: 3,
      people: 2,
      attention: 2,
      conflicts: 1,
      rework: 2,
    })
  })
  it('orders the latest ships with full tie-breaks', () => {
    expect(latest(items).map((i) => `${i.repo}:${i.value.sha}`)).toEqual(['r1:c', 'r2:b', 'r1:a'])
  })
})

describe('perDay', () => {
  it('buckets by UTC day, including empty days', () => {
    const since = 10 * DAY
    const days = perDay([item('r', 'a', since + 5), item('r', 'b', since + 2 * DAY + 1), item('r', 'c', since + 2 * DAY + 2), item('r', 'old', since - 1)], since, since + 2 * DAY + 100)
    expect(days.map((d) => d.count)).toEqual([1, 0, 2])
    expect(days[0].day).toBe(since)
  })
})

describe('formatting', () => {
  it('counts a group\'s repositories in the scope menu', () => {
    expect(groupLabel('Harbour team', 3)).toBe('Harbour team · 3 repos')
    expect(groupLabel('Solo', 1)).toBe('Solo · 1 repo')
  })

  const t = Date.UTC(2026, 9, 29, 17, 42) / 1000
  it('writes UTC dates', () => {
    expect(utcDay(t)).toBe('Oct 29')
    expect(utcDay(Date.UTC(2025, 11, 31) / 1000, t)).toBe('Dec 31, 2025')
    expect(utcMoment(t)).toBe('Oct 29, 17:42')
  })
  it('says how long ago', () => {
    expect(ago(1000, 1030)).toBe('just now')
    expect(ago(0, 2 * 3600)).toBe('2 hours ago')
    expect(ago(0, 3 * DAY)).toBe('3 days ago')
  })
  it('shares the latest as-of point', () => {
    expect(sharedAsOf([5, 9, 7])).toBe(9)
    expect(sharedAsOf([])).toBe(0)
  })
})

describe('shipped grouping', () => {
  const shipped = (repo: string, sha: string, committed: number, refs: string[], cycle = 0) => {
    const it = item(repo, sha, committed, { refs })
    return { repo, value: { ...it.value, cycle } }
  }
  const items = [
    shipped('r', 'a', 500, ['TIDE-2']),
    shipped('r', 'b', 400, ['TIDE-1', 'TIDE-9'], 7200),
    shipped('r', 'c', 300, []),
    shipped('r', 'd', 200, ['TIDE-1'], 3600),
    shipped('r', 'e', 100, ['TIDE-3']),
    shipped('r', 'f', 600, ['TIDE-4']),
  ]

  it('groups by first reference, biggest then newest, no-reference last', () => {
    const groups = byReference(items)
    expect(groups.map((g) => `${g.ref}:${g.items.map((i) => i.value.sha).join('')}`)).toEqual(['TIDE-1:bd', 'TIDE-4:f', 'TIDE-2:a', 'TIDE-3:e', ':c'])
    expect(groups[0].cycle).toBe(5400)
    expect(groups[1].cycle).toBe(0)
  })

  it('groups by UTC day, newest first', () => {
    const days = byDay([shipped('r', 'x', DAY + 5, []), shipped('r', 'y', 2 * DAY + 1, []), shipped('r', 'z', DAY + 9, [])])
    expect(days.map((d) => [d.day, d.items.map((i) => i.value.sha).join('')])).toEqual([
      [2 * DAY, 'y'],
      [DAY, 'zx'],
    ])
  })

  it('takes medians of known values and formats durations', () => {
    expect(median([0, 3, 1, 2])).toBe(2)
    expect(median([0, 4, 2])).toBe(3)
    expect(median([0])).toBe(0)
    expect([30 * 60, 5.5 * 3600, 30 * 3600, 100 * 3600].map(duration)).toEqual(['30 min', '5.5 h', '30 h', '4.2 days'])
  })
})

describe('hotFiles', () => {
  it('orders across repositories with full tie-breaks', () => {
    const f = (repo: string, path: string, lines: number, items: number) => ({ repo, value: { path, lines, items, openBranches: 0 } })
    expect(hotFiles([f('r2', 'a', 10, 1), f('r1', 'b', 30, 1), f('r1', 'c', 10, 2), f('r1', 'a', 10, 1)]).map((h) => `${h.repo}:${h.value.path}`)).toEqual([
      'r1:b',
      'r1:c',
      'r1:a',
      'r2:a',
    ])
  })
})

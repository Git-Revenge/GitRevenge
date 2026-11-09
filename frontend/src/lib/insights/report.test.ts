import { describe, expect, it } from 'vitest'
import type { Ranked, Tagged } from './model'
import { attentionCSV, exportName, hotspotsCSV, inputLines, shippedCSV, summary, type ReportInputs, type Shipped, type Signal } from './report'

const T = Date.UTC(2026, 6, 22, 9, 0) / 1000
const inputs: ReportInputs = {
  scope: 'Harbour team',
  windows: [
    { repo: '/r/tidewater', name: 'tidewater', target: 'origin/main', targetSha: 'aaa111', asOf: T, since: Date.UTC(2026, 6, 9) / 1000, until: T, range: '' },
    { repo: '/r/api', name: 'harbour-api', target: 'main', targetSha: 'bbb222', asOf: T, since: Date.UTC(2026, 6, 9) / 1000, until: T, range: '' },
  ],
  thresholds: { stale: 7, longLived: 14, behind: 50, size: 500 },
}
const name = (p: string) => p.split('/').pop() ?? p
const item = (repo: string, sha: string, title: string, refs: string[], committed: number, extra: Partial<Shipped> = {}): Tagged<Shipped> => ({
  repo,
  value: { sha, title, author: 'Maya Okafor', email: 'maya@x', committed, refs, revert: false, fixup: false, files: 2, added: 10, removed: 3, merge: false, cycle: 0, ...extra },
})
const items = [item('/r/tidewater', 's1', 'Cache readings', ['TIDE-88'], T - 100, { merge: true, cycle: 7200 }), item('/r/api', 's2', 'Ignore logs, "really"', [], T - 50)]
const flagged: Ranked<Signal>[] = [
  {
    repo: '/r/tidewater',
    value: { name: 'origin/feature/units', sha: 'c1', author: 'Leo', local: false, lastCommit: T, flags: ['drifting'], staleDays: 2, ageDays: 9, behind: 61, added: 100, removed: 43 },
    conflict: { repo: '/r/tidewater', targetSha: 'aaa111', branch: 'origin/feature/units', sha: 'c1', conflicts: true, files: ['src/alerts.js'] },
  },
  { repo: '/r/api', value: { name: 'spike', sha: 'c2', author: 'Priya', local: true, lastCommit: T, flags: ['stale'], staleDays: 12, ageDays: 13, behind: 7, added: 80, removed: 8 } },
]

describe('inputs', () => {
  it('names every repository commit, the UTC window, as-of and thresholds', () => {
    expect(inputLines(inputs)).toEqual([
      ['Scope', 'Harbour team'],
      ['Repository tidewater', 'origin/main @ aaa111'],
      ['Repository harbour-api', 'main @ bbb222'],
      ['Window', '2026-07-09T00:00:00Z to 2026-07-22T09:00:00Z'],
      ['As of', '2026-07-22T09:00:00Z'],
      ['References', 'built in: PROJ-123 and #123'],
      ['Thresholds', 'stale 7d, long-lived 14d, behind 50, size 500 lines'],
    ])
  })
})

describe('summary', () => {
  it('writes the inputs, grouped ships and flagged branches with pending checks', () => {
    const md = summary(inputs, items, flagged, [], { shipped: true, attention: true, hotspots: false, groupByReference: true }, name)
    expect(md).toBe(
      [
        '# Harbour team: 2026-07-09T00:00:00Z to 2026-07-22T09:00:00Z',
        '',
        'Scope: Harbour team  ',
        'Repository tidewater: origin/main @ aaa111  ',
        'Repository harbour-api: main @ bbb222  ',
        'Window: 2026-07-09T00:00:00Z to 2026-07-22T09:00:00Z  ',
        'As of: 2026-07-22T09:00:00Z  ',
        'References: built in: PROJ-123 and #123  ',
        'Thresholds: stale 7d, long-lived 14d, behind 50, size 500 lines  ',
        '',
        '## Shipped (2)',
        '',
        '**TIDE-88**',
        '- Cache readings (tidewater, Maya Okafor)',
        '',
        '**No reference**',
        '- Ignore logs, "really" (api, Maya Okafor)',
        '',
        '## Needs attention (2)',
        '',
        '- origin/feature/units (tidewater): would conflict (src/alerts.js), 61 behind',
        '- spike (api), local only: no commits for 12 days, conflict check pending',
        '',
      ].join('\n'),
    )
  })

  it('writes hotspots with the right plurals', () => {
    const hot = [
      { repo: '/r/tidewater', value: { path: 'a.js', lines: 1, items: 1, openBranches: 1 } },
      { repo: '/r/api', value: { path: 'b.js', lines: 9, items: 2, openBranches: 0 } },
    ]
    const md = summary(inputs, [], [], hot, { shipped: false, attention: false, hotspots: true, groupByReference: false }, name)
    expect(md).toContain('- a.js (tidewater): 1 line in 1 item, 1 open branch touches it\n- b.js (api): 9 lines in 2 items\n')
  })

  it('is the same every time for the same results', () => {
    const parts = { shipped: true, attention: true, hotspots: true, groupByReference: false }
    expect(summary(inputs, [...items].reverse(), flagged, [], parts, name)).toBe(summary(inputs, items, flagged, [], parts, name))
  })
})

describe('CSV', () => {
  it('puts the inputs first, then quoted rows', () => {
    const csv = shippedCSV(inputs, items, name)
    const lines = csv.split('\r\n')
    expect(lines[0]).toBe('Scope,Harbour team')
    expect(lines[7]).toBe('')
    expect(lines[8]).toBe('repository,commit,committed_utc,title,author,email,references,files,added,removed,merge,cycle_hours,revert,fixup,bot')
    expect(lines[9]).toBe('api,s2,2026-07-22T08:59:10Z,"Ignore logs, ""really""",Maya Okafor,maya@x,,2,10,3,no,,no,no,no')
    expect(lines[10]).toContain('TIDE-88,2,10,3,yes,2.0,no,no')
  })

  it('marks pending conflict checks and lists hot files', () => {
    expect(attentionCSV(inputs, flagged, name)).toContain('api,spike,c2,yes,Priya,pending,,12,13,7,88,stale')
    expect(hotspotsCSV(inputs, [{ repo: '/r/api', value: { path: 'a.go', lines: 5, items: 2, openBranches: 1 } }], name)).toContain('api,a.go,5,2,1')
  })

  it('names files after the scope, list and as-of day', () => {
    expect(exportName('Harbour team', 'shipped', T, 'csv')).toBe('insights-harbour-team-shipped-2026-07-22.csv')
  })
})

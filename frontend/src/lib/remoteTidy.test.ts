import { describe, expect, it } from 'vitest'
import { addTidyRows, tidyCounts, tidyLabel, tidyStatus } from './remoteTidy'
import type { TreeRow } from './sidebarTree'

const folder = (label: string, depth = 0, open = true, remote = true): TreeRow => ({ type: 'folder', key: `remotes:${label}`, label, depth, open, count: 1, holdsHead: false, remote, startsGroup: false })
const ref = (short: string, depth = 1): TreeRow => ({ type: 'ref', ref: { name: short, short, kind: 'remote', commit: '', head: false, upstream: '', ahead: 0, behind: 0, gone: false }, label: short, depth, stacked: false, context: false, startsGroup: false })
const outline = (rows: TreeRow[]) => rows.map((r) => '  '.repeat(r.depth) + (r.type === 'tidy' ? `[${r.label}]` : r.label))

describe('tidy in the Remotes panel', () => {
  it('counts each remote’s branches, a local branch under its upstream’s remote', () => {
    const marks = { 'origin/a': 'merged', 'origin/b': 'gone', 'team/x/c': 'quiet', fix: 'gone' }
    const refs = [{ kind: 'local' as const, short: 'fix', upstream: 'team/x/fix' }]
    expect(tidyCounts(marks, refs, ['origin', 'team', 'team/x'])).toEqual({ origin: 2, 'team/x': 2 })
  })

  it('puts a row at the foot of each open remote with something to tidy', () => {
    const rows = [folder('origin'), ref('origin/a'), folder('release', 1, true, false), ref('origin/release/1', 2), folder('upstream'), ref('upstream/a'), folder('closed', 0, false)]
    expect(outline(addTidyRows(rows, { origin: 2, closed: 4 }))).toEqual([
      'origin',
      '  origin/a',
      '  release',
      '    origin/release/1',
      '  [2 could be tidied]',
      'upstream',
      '  upstream/a',
      'closed',
    ])
  })

  it('adds nothing when nothing needs tidying', () => {
    const rows = [folder('origin'), ref('origin/a')]
    expect(addTidyRows(rows, {})).toBe(rows)
  })

  it('says each mark in words', () => {
    expect(tidyStatus('gone')).toMatchObject({ text: 'gone', spoken: 'deleted on the remote' })
    expect(tidyStatus('merged')?.text).toBe('merged')
    expect(tidyStatus('quiet')?.text).toBe('quiet')
    expect(tidyStatus(undefined)).toBeUndefined()
    expect(tidyLabel('origin', 1)).toBe('1 branch on origin could be tidied')
  })
})

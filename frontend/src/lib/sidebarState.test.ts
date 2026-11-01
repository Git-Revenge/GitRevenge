import { describe, expect, it } from 'vitest'
import type { Ref } from './graph/rowCache'
import { clampWidth, DEFAULT_WIDTH, defaultSidebar, loadSidebar, MAX_WIDTH, MIN_WIDTH, otherPanelWithMatches, ROW_HEIGHT, saveSidebar, visibleRange } from './sidebarState'
import { buildPanels } from './sidebarTree'

function memory(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v), data }
}
const key = 'gitrevenge.sidebar:/r'

describe('loadSidebar', () => {
  it('uses the defaults when nothing is saved', () => {
    expect(loadSidebar('/r', memory())).toEqual(defaultSidebar())
  })

  it('round-trips what was saved', () => {
    const store = memory()
    const state = { panel: 'tags', collapsed: true, width: 300, open: { 'branches:fix': false }, sort: 'recent' } as const
    saveSidebar('/r', { ...state, open: { ...state.open } }, store)
    expect(loadSidebar('/r', store)).toEqual(state)
  })

  it('keeps each repository separate', () => {
    const store = memory()
    saveSidebar('/a', { ...defaultSidebar(), panel: 'tags' }, store)
    expect(loadSidebar('/b', store).panel).toBe('branches')
  })

  it('falls back to the defaults for unreadable data', () => {
    expect(loadSidebar('/r', memory({ [key]: 'not json' }))).toEqual(defaultSidebar())
    expect(loadSidebar('/r', memory({ [key]: '"text"' }))).toEqual(defaultSidebar())
    expect(loadSidebar('/r', memory({ [key]: 'null' }))).toEqual(defaultSidebar())
  })

  it('throws out individual bad values and keeps good ones', () => {
    const saved = { panel: 'nope', collapsed: 'yes', width: 'wide', sort: 'size', open: { good: true, bad: 'x' } }
    expect(loadSidebar('/r', memory({ [key]: JSON.stringify({ ...saved, collapsed: true }) }))).toEqual({ ...defaultSidebar(), collapsed: true, open: { good: true } })
    expect(loadSidebar('/r', memory({ [key]: JSON.stringify(saved) })).width).toBe(DEFAULT_WIDTH)
    expect(loadSidebar('/r', memory({ [key]: JSON.stringify({ open: ['x'] }) })).open).toEqual({})
  })

  it('brings a saved width back within limits', () => {
    expect(loadSidebar('/r', memory({ [key]: JSON.stringify({ width: 9999 }) })).width).toBe(MAX_WIDTH)
  })

  it('survives a store that throws', () => {
    const blocked = {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('full')
      },
    }
    expect(loadSidebar('/r', blocked)).toEqual(defaultSidebar())
    expect(() => saveSidebar('/r', defaultSidebar(), blocked)).not.toThrow()
  })
})

describe('clampWidth', () => {
  it('keeps widths within limits, in whole pixels', () => {
    expect(clampWidth(10)).toBe(MIN_WIDTH)
    expect(clampWidth(99999)).toBe(MAX_WIDTH)
    expect(clampWidth(250.6)).toBe(251)
  })
})

describe('otherPanelWithMatches', () => {
  const ref = (short: string, kind: Ref['kind']): Ref => ({ name: short, short, kind, commit: '', head: false, upstream: '', ahead: 0, behind: 0, gone: false })
  const panels = buildPanels([ref('main', 'local'), ref('v1', 'tag'), ref('v2', 'tag')])

  it('names a panel that has matches, with its count', () => {
    expect(otherPanelWithMatches(panels, 'branches', 'v')).toEqual({ panel: 'tags', count: 2 })
  })

  it('skips the current panel', () => {
    expect(otherPanelWithMatches(panels, 'tags', 'v')).toBeNull()
  })

  it('finds nothing without a query or a match', () => {
    expect(otherPanelWithMatches(panels, 'branches', '  ')).toBeNull()
    expect(otherPanelWithMatches(panels, 'branches', 'nope')).toBeNull()
  })

  it('works from the Repository panel', () => {
    expect(otherPanelWithMatches(panels, 'repo', 'main')).toEqual({ panel: 'branches', count: 1 })
  })
})

describe('visibleRange', () => {
  it('draws the view plus overscan at the top', () => {
    expect(visibleRange(0, ROW_HEIGHT * 10, 1000, 5)).toEqual({ start: 0, end: 15 })
  })

  it('moves with the scroll position', () => {
    expect(visibleRange(ROW_HEIGHT * 100, ROW_HEIGHT * 10, 1000, 5)).toEqual({ start: 95, end: 115 })
  })

  it('stops at the end of the list', () => {
    expect(visibleRange(ROW_HEIGHT * 995, ROW_HEIGHT * 10, 1000, 5)).toEqual({ start: 990, end: 1000 })
  })

  it('copes with a list shorter than the scroll position', () => {
    expect(visibleRange(ROW_HEIGHT * 500, ROW_HEIGHT * 10, 3, 5)).toEqual({ start: 3, end: 3 })
    expect(visibleRange(0, 0, 0)).toEqual({ start: 0, end: 0 })
  })
})

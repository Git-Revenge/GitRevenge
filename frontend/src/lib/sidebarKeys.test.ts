import { describe, expect, it } from 'vitest'
import type { Ref } from './graph/rowCache'
import { railMove, scrollToShow, treeAction, treePositions } from './sidebarKeys'
import { buildPanels, panelRows, type TreeRow } from './sidebarTree'

const ref = (short: string, extra: Partial<Ref> = {}): Ref => ({ name: short, short, kind: 'local', commit: short, head: false, upstream: '', ahead: 0, behind: 0, gone: false, ...extra })

/*
 0 fix/
 1   a
 2   b
 3     stacked
 4 old/      (closed)
 5 loose
*/
const refs = [ref('fix/a'), ref('fix/b'), ref('stacked', { stackParent: 'fix/b' }), ref('old/x'), ref('old/y'), ref('loose')]
const closedKey = 'branches:old'
const rows: TreeRow[] = panelRows(buildPanels(refs).branches, { open: { [closedKey]: false } })
const press = (key: string, active: number, shiftKey = false) => treeAction({ key, shiftKey }, rows, active, 3)
const move = (index: number) => ({ type: 'move', index })

describe('the test tree', () => {
  it('is the shape the tests assume', () => {
    expect(rows.map((r) => `${r.depth}:${r.label}`)).toEqual(['0:fix', '1:a', '1:b', '2:stacked', '0:old', '0:loose'])
  })
})

describe('moving', () => {
  it('moves up and down, stopping at the ends', () => {
    expect(press('ArrowDown', 0)).toEqual(move(1))
    expect(press('ArrowUp', 1)).toEqual(move(0))
    expect(press('ArrowUp', 0)).toEqual(move(0))
    expect(press('ArrowDown', 5)).toEqual(move(5))
  })

  it('goes to the ends with Home and End', () => {
    expect(press('Home', 3)).toEqual(move(0))
    expect(press('End', 1)).toEqual(move(5))
  })

  it('moves a page at a time, stopping at the ends', () => {
    expect(press('PageDown', 1)).toEqual(move(4))
    expect(press('PageDown', 4)).toEqual(move(5))
    expect(press('PageUp', 4)).toEqual(move(1))
    expect(press('PageUp', 1)).toEqual(move(0))
  })
})

describe('Right', () => {
  it('opens a closed folder', () => {
    expect(press('ArrowRight', 4)).toEqual({ type: 'toggle' })
  })

  it('steps into an open folder', () => {
    expect(press('ArrowRight', 0)).toEqual(move(1))
  })

  it('steps down to a stacked branch', () => {
    expect(press('ArrowRight', 2)).toEqual(move(3))
  })

  it('does nothing on a row with nothing under it', () => {
    expect(press('ArrowRight', 1)).toBeNull()
    expect(press('ArrowRight', 5)).toBeNull()
  })
})

describe('Left', () => {
  it('closes an open folder', () => {
    expect(press('ArrowLeft', 0)).toEqual({ type: 'toggle' })
  })

  it('steps out to the folder', () => {
    expect(press('ArrowLeft', 2)).toEqual(move(0))
  })

  it('steps out to the stack parent', () => {
    expect(press('ArrowLeft', 3)).toEqual(move(2))
  })

  it('does nothing at the top level', () => {
    expect(press('ArrowLeft', 4)).toBeNull()
    expect(press('ArrowLeft', 5)).toBeNull()
  })

  it('moves to the parent when the filter holds a folder open', () => {
    const nested = panelRows(buildPanels([ref('a/b/one'), ref('a/b/two'), ref('a/c')]).branches, { query: 'a' })
    expect(nested.map((r) => r.label)).toEqual(['a', 'b', 'one', 'two', 'c'])
    expect(treeAction({ key: 'ArrowLeft' }, nested, 1, 3, true)).toEqual(move(0))
    expect(treeAction({ key: 'ArrowLeft' }, nested, 0, 3, true)).toBeNull()
  })
})

describe('acting on a row', () => {
  it('toggles a folder with Enter or Space', () => {
    expect(press('Enter', 0)).toEqual({ type: 'toggle' })
    expect(press(' ', 4)).toEqual({ type: 'toggle' })
  })

  it('shows a ref with Enter, and checks it out with Space', () => {
    expect(press('Enter', 1)).toEqual({ type: 'pick' })
    expect(press(' ', 1)).toEqual({ type: 'checkout' })
  })

  it('opens the menu with the Menu key or Shift+F10, on refs only', () => {
    expect(press('ContextMenu', 1)).toEqual({ type: 'menu' })
    expect(press('F10', 1, true)).toEqual({ type: 'menu' })
    expect(press('F10', 1)).toBeNull()
    expect(press('ContextMenu', 0)).toBeNull()
    expect(treeAction({ key: 'ContextMenu', shiftKey: false }, [{ ...rows[0], stack: true } as TreeRow], 0, 10)).toEqual({ type: 'menu' })
    expect(press('F10', 0, true)).toBeNull()
  })
})

describe('edge cases', () => {
  it('ignores other keys', () => {
    expect(press('a', 1)).toBeNull()
    expect(press('Tab', 1)).toBeNull()
  })

  it('does nothing in an empty tree', () => {
    expect(treeAction({ key: 'ArrowDown' }, [], 0, 3)).toBeNull()
    expect(treeAction({ key: 'Enter' }, [], 0, 3)).toBeNull()
  })

  it('recovers with arrows when the active row is gone, without acting on it', () => {
    expect(treeAction({ key: 'ArrowUp' }, rows, 99, 3)).toEqual(move(5))
    expect(treeAction({ key: 'Enter' }, rows, 99, 3)).toBeNull()
  })
})

describe('treePositions', () => {
  it('counts each row among its siblings', () => {
    expect(treePositions(rows)).toEqual([
      { pos: 1, size: 3 }, // fix/
      { pos: 1, size: 2 }, // a
      { pos: 2, size: 2 }, // b
      { pos: 1, size: 1 }, // stacked
      { pos: 2, size: 3 }, // old/
      { pos: 3, size: 3 }, // loose
    ])
  })

  it('starts a new count under each parent', () => {
    const p = treePositions(panelRows(buildPanels([ref('a/one'), ref('a/two'), ref('b/one'), ref('b/two'), ref('b/three')]).branches))
    expect(p.map((x) => x.size)).toEqual([2, 2, 2, 2, 3, 3, 3])
  })

  it('handles an empty tree', () => {
    expect(treePositions([])).toEqual([])
  })
})

describe('scrollToShow', () => {
  it('leaves a visible row alone', () => {
    expect(scrollToShow(5, 28, 0, 280)).toBe(0)
  })

  it('scrolls up to a row above the view', () => {
    expect(scrollToShow(2, 28, 280, 280)).toBe(56)
  })

  it('scrolls down just enough for a row below the view', () => {
    expect(scrollToShow(10, 28, 0, 280)).toBe(28)
  })
})

describe('railMove', () => {
  it('wraps with the arrows', () => {
    expect(railMove('ArrowDown', 0, 3)).toBe(1)
    expect(railMove('ArrowDown', 2, 3)).toBe(0)
    expect(railMove('ArrowUp', 0, 3)).toBe(2)
  })

  it('goes to the ends with Home and End', () => {
    expect(railMove('Home', 2, 3)).toBe(0)
    expect(railMove('End', 0, 3)).toBe(2)
  })

  it('ignores other keys', () => {
    expect(railMove('a', 0, 3)).toBeNull()
  })
})

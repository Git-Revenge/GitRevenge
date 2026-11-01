import type { TreeRow } from './sidebarTree'

/** What a key press in the sidebar's tree does. */
export type TreeAction =
  | { type: 'move'; index: number }
  /** Open or close the active folder. */
  | { type: 'toggle' }
  /** Show the active ref in the graph. */
  | { type: 'pick' }
  /** Check the active ref out. */
  | { type: 'checkout' }
  /** Open the active ref's menu. */
  | { type: 'menu' }

export interface KeyPress {
  key: string
  shiftKey?: boolean
}

/** The row above this one with less depth: its folder or stack parent; -1 at the top level. */
function parentOf(rows: TreeRow[], index: number): number {
  for (let i = index - 1; i >= 0; i--) if (rows[i].depth < rows[index].depth) return i
  return -1
}

const hasChildren = (rows: TreeRow[], index: number) => index + 1 < rows.length && rows[index + 1].depth > rows[index].depth

/**
 * The action for a key press, or null if the tree doesn't use it. Folders can be held open by
 * the filter, in which case they can't be closed and Left moves to the parent instead.
 */
export function treeAction(press: KeyPress, rows: TreeRow[], active: number, pageSize: number, heldOpen = false): TreeAction | null {
  if (rows.length === 0) return null
  const row = rows[active]
  const last = rows.length - 1
  const move = (index: number): TreeAction => ({ type: 'move', index: Math.max(0, Math.min(last, index)) })
  switch (press.key) {
    case 'ArrowDown':
      return move(active + 1)
    case 'ArrowUp':
      return move(active - 1)
    case 'Home':
      return move(0)
    case 'End':
      return move(last)
    case 'PageDown':
      return move(active + pageSize)
    case 'PageUp':
      return move(active - pageSize)
  }
  if (!row) return null
  switch (press.key) {
    case 'ArrowRight':
      if (row.type === 'folder' && !row.open) return { type: 'toggle' }
      return hasChildren(rows, active) ? move(active + 1) : null
    case 'ArrowLeft': {
      if (row.type === 'folder' && row.open && !heldOpen) return { type: 'toggle' }
      const parent = parentOf(rows, active)
      return parent >= 0 ? move(parent) : null
    }
    case 'Enter':
      return row.type === 'folder' ? { type: 'toggle' } : { type: 'pick' }
    case ' ':
      return row.type === 'folder' ? { type: 'toggle' } : { type: 'checkout' }
    case 'ContextMenu':
      return row.type === 'ref' ? { type: 'menu' } : null
    case 'F10':
      return press.shiftKey && row.type === 'ref' ? { type: 'menu' } : null
  }
  return null
}

/** Each row's position among its siblings and how many there are. Only the rows in view are in
 * the DOM, so screen readers can't work this out for themselves. */
export function treePositions(rows: TreeRow[]): { pos: number; size: number }[] {
  const open: { size: number }[] = []
  const sets: { pos: number; set: { size: number } }[] = []
  for (const row of rows) {
    open.length = Math.min(open.length, row.depth + 1)
    const set = (open[row.depth] ??= { size: 0 })
    set.size++
    sets.push({ pos: set.size, set })
  }
  return sets.map((s) => ({ pos: s.pos, size: s.set.size }))
}

/** The scrollTop that brings a row into view, moving as little as possible. */
export function scrollToShow(index: number, rowHeight: number, scrollTop: number, viewHeight: number): number {
  const top = index * rowHeight
  if (top < scrollTop) return top
  if (top + rowHeight > scrollTop + viewHeight) return top + rowHeight - viewHeight
  return scrollTop
}

/** The next index along the rail, wrapping; Home and End go to the ends. Null for other keys. */
export function railMove(key: string, index: number, count: number): number | null {
  if (key === 'ArrowDown') return (index + 1) % count
  if (key === 'ArrowUp') return (index - 1 + count) % count
  if (key === 'Home') return 0
  if (key === 'End') return count - 1
  return null
}

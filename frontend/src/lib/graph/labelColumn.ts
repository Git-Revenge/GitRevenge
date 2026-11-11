import { fitLabelColumn, LABEL_COLUMN } from './draw'

/** How long the graph has to hold still before the label column refits, so it doesn't twitch
 * while labels scroll past. */
export const REFIT_AFTER_MS = 200

/**
 * The label column's width for this frame. A width the person dragged wins. Otherwise the column
 * fits the widest label in view (`need`, 0 when there are none), but only moves once the graph
 * has held still; with no labels in view it stays as it is, since there's nothing to fit.
 */
export function nextLabelWidth(current: number | null, chosen: number | null, need: number, still: boolean): number {
  if (chosen !== null) return fitLabelColumn(chosen)
  if (current === null) return need ? fitLabelColumn(need) : LABEL_COLUMN.default
  if (!need || !still) return current
  return fitLabelColumn(need)
}

const keyFor = (path: string) => `gitrevenge.labelColumn:${path}`

type Store = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

/** The width someone dragged the label column to in a repository, or null if they never did. */
export function loadLabelWidth(path: string, store: Store): number | null {
  let saved: unknown
  try {
    saved = JSON.parse(store.getItem(keyFor(path)) ?? 'null')
  } catch {
    return null
  }
  return typeof saved === 'number' && Number.isFinite(saved) ? fitLabelColumn(saved) : null
}

/** Remembers the chosen width; null goes back to fitting the labels. It's only a convenience,
 * so a full or blocked store is ignored. */
export function saveLabelWidth(path: string, width: number | null, store: Store): void {
  try {
    if (width === null) store.removeItem(keyFor(path))
    else store.setItem(keyFor(path), JSON.stringify(fitLabelColumn(width)))
  } catch {
    // Nothing to do: the column still works, and just won't remember this.
  }
}

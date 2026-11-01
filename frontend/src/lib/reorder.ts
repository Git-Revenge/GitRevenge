/** Dragging things into a new order: which gap the pointer is over, and where that puts them. */

export interface Box {
  left: number
  top: number
  right: number
  bottom: number
}

/**
 * The gap the pointer is over in a vertical list, from 0 (before the first item) to
 * boxes.length (after the last).
 */
export function gapInList(y: number, boxes: Box[]): number {
  const i = boxes.findIndex((b) => y < (b.top + b.bottom) / 2)
  return i < 0 ? boxes.length : i
}

/**
 * The gap the pointer is over in a wrapping grid: the first item whose row hasn't ended above the
 * pointer and which the pointer is left of the middle of, or on a row below the pointer.
 */
export function gapInGrid(x: number, y: number, boxes: Box[]): number {
  const i = boxes.findIndex((b) => y < b.top || (y < b.bottom && x < (b.left + b.right) / 2))
  return i < 0 ? boxes.length : i
}

export function isInside(x: number, y: number, b: Box): boolean {
  return x >= b.left && x < b.right && y >= b.top && y < b.bottom
}

/**
 * The index an item ends up at when dropped in a gap, with the item itself taken out of the
 * count, or null when the drop leaves it where it was.
 */
export function indexForGap(from: number, gap: number): number | null {
  const to = gap > from ? gap - 1 : gap
  return to === from ? null : to
}

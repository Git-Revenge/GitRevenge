/**
 * The graph's rows, for a history too long to measure.
 *
 * Rows aren't all commits, and aren't all one height: a day gets a line of its own, a commit
 * that carries on a run is shorter than one that starts it, and a folded lane is one line for
 * many commits. What a row is depends on its neighbours, and most of a long history isn't loaded,
 * so nothing here adds heights up from the top. The view is placed by an anchor instead: the
 * slot at its top edge and how far into that slot it has scrolled. Everything else is found by
 * walking from the anchor, which only ever covers a few screens.
 *
 * A slot is one commit's place in the list together with any rows that come with it (the day
 * line above the first commit of a day, say). A caller may also give slots to things that aren't
 * commits, such as a row of its own above the first commit.
 */

export type RowKind = 'lead' | 'day' | 'fold' | 'commit'

/** One row a slot brings. */
export interface SlotRow {
  kind: RowKind
  /** Its height in px; 0 for a row that isn't shown, such as a commit folded away. */
  h: number
}

export interface Slots {
  /** How many slots there are. */
  count: number
  /** The rows of slot i, top to bottom. The last is the slot's own row. A slot that isn't loaded
   * answers with its best guess. */
  rows(i: number): readonly SlotRow[]
}

/** Where the view's top edge is: `offset` px into slot `index`. */
export interface Anchor {
  index: number
  offset: number
}

/** A row as placed in the view: `y` is its top edge, from the view's. */
export interface ViewRow {
  kind: RowKind
  /** The slot it belongs to. */
  slot: number
  y: number
  h: number
}

/** A slot's height: all its rows. */
export function slotHeight(s: Slots, i: number): number {
  let h = 0
  for (const r of s.rows(i)) h += r.h
  return h
}

/** The height of a slot's own row, its last. */
export function ownHeight(s: Slots, i: number): number {
  const rows = s.rows(i)
  return rows.length ? rows[rows.length - 1].h : 0
}

/** The anchor at the very top. */
export const START: Anchor = { index: 0, offset: 0 }

/** An anchor that names a real place: a slot that exists, and an offset inside it. */
export function normalize(s: Slots, a: Anchor): Anchor {
  if (s.count <= 0) return START
  const index = Math.max(0, Math.min(s.count - 1, Math.floor(a.index)))
  if (index !== a.index) return { index, offset: 0 }
  return { index, offset: Math.max(0, Math.min(a.offset, slotHeight(s, index))) }
}

/** The anchor `dy` px further down (or up, when negative), stopping at either end of the list.
 * It doesn't know the view's height: see `clampToEnd` for keeping the last row at the bottom. */
export function moveBy(s: Slots, a: Anchor, dy: number): Anchor {
  if (s.count <= 0) return START
  let { index, offset } = normalize(s, a)
  offset += dy
  while (offset < 0 && index > 0) {
    index--
    offset += slotHeight(s, index)
  }
  if (offset < 0) return START
  for (;;) {
    const h = slotHeight(s, index)
    if (offset < h || index >= s.count - 1) return { index, offset: Math.min(offset, h) }
    offset -= h
    index++
  }
}

/** The lowest the view can go: the anchor that puts the last row's bottom edge at the bottom of
 * a view `viewH` tall, or the top when everything fits. */
export function endAnchor(s: Slots, viewH: number): Anchor {
  let below = 0
  for (let i = s.count - 1; i >= 0; i--) {
    below += slotHeight(s, i)
    if (below >= viewH) return { index: i, offset: below - viewH }
  }
  return START
}

/** Negative when a is above b, positive when below, 0 at the same place. */
export function compareAnchors(a: Anchor, b: Anchor): number {
  return a.index - b.index || a.offset - b.offset
}

/** An anchor kept from going past the end. */
export function clampToEnd(s: Slots, a: Anchor, viewH: number): Anchor {
  const n = normalize(s, a)
  const end = endAnchor(s, viewH)
  return compareAnchors(n, end) > 0 ? end : n
}

/** The rows that show in a view `viewH` tall, top to bottom. Rows of no height are left out. */
export function layout(s: Slots, a: Anchor, viewH: number): ViewRow[] {
  const out: ViewRow[] = []
  if (s.count <= 0) return out
  const at = normalize(s, a)
  let y = 0 - at.offset
  for (let i = at.index; i < s.count && y < viewH; i++) {
    for (const r of s.rows(i)) {
      if (r.h > 0 && y + r.h > 0 && y < viewH) out.push({ kind: r.kind, slot: i, y, h: r.h })
      y += r.h
    }
  }
  return out
}

/** The anchor that puts slot i's own row `share` of the way down the view: 0 at the top edge,
 * 0.5 with its top at the middle. */
export function anchorFor(s: Slots, i: number, viewH: number, share: number): Anchor {
  if (s.count <= 0) return START
  const index = Math.max(0, Math.min(s.count - 1, i))
  const own = { index, offset: slotHeight(s, index) - ownHeight(s, index) }
  return moveBy(s, own, -viewH * share)
}

/** The anchor that puts the bottom edge of slot i's own row at the bottom of the view. */
export function anchorForBottom(s: Slots, i: number, viewH: number): Anchor {
  if (s.count <= 0) return START
  const index = Math.max(0, Math.min(s.count - 1, i))
  return moveBy(s, { index, offset: slotHeight(s, index) }, -viewH)
}

/** How far it is from one anchor down to another, in px (negative when `to` is above), or null
 * when that's more than `limit`: far enough that walking there isn't worth it. */
export function distance(s: Slots, from: Anchor, to: Anchor, limit: number): number | null {
  const a = normalize(s, from)
  const b = normalize(s, to)
  const sign = compareAnchors(a, b) <= 0 ? 1 : -1
  const [lo, hi] = sign > 0 ? [a, b] : [b, a]
  let d = 0 - lo.offset
  for (let i = lo.index; i < hi.index; i++) {
    d += slotHeight(s, i)
    if (d > limit) return null
  }
  d += hi.offset
  return d > limit ? null : sign * d || 0
}

/** Where slot i's own row is: its top edge from the view's, or null when it's more than `reach`
 * px above the view's top or below its bottom. */
export function yOf(s: Slots, a: Anchor, i: number, viewH: number, reach: number): number | null {
  if (i < 0 || i >= s.count) return null
  const at = normalize(s, a)
  if (i >= at.index) {
    let y = 0 - at.offset
    for (let k = at.index; k < i; k++) {
      y += slotHeight(s, k)
      if (y > viewH + reach) return null
    }
    y += slotHeight(s, i) - ownHeight(s, i)
    return y > viewH + reach ? null : y
  }
  let y = 0 - at.offset
  for (let k = at.index - 1; k > i; k--) {
    y -= slotHeight(s, k)
    if (y < -reach) return null
  }
  y -= ownHeight(s, i)
  return y < -reach ? null : y
}

/** An anchor as one number, for a scrollbar: the slot, plus how far through it. */
export function position(s: Slots, a: Anchor): number {
  const at = normalize(s, a)
  const h = slotHeight(s, at.index)
  return at.index + (h > 0 ? Math.min(1, at.offset / h) : 0)
}

/** The anchor at a position (see `position`). */
export function anchorAt(s: Slots, pos: number): Anchor {
  if (s.count <= 0 || !(pos > 0)) return START
  const index = Math.min(s.count - 1, Math.floor(pos))
  const h = slotHeight(s, index)
  return { index, offset: Math.min(h, (pos - index) * h) }
}

/** The row at a height in the view, if any. */
export function rowAt(rows: readonly ViewRow[], y: number): ViewRow | undefined {
  for (const r of rows) if (y >= r.y && y < r.y + r.h) return r
  return undefined
}

/** The slot `by` own-rows on from slot i, counting only slots whose own row shows, and stopping
 * at the first or last such slot. Slot i itself needn't be showing. */
export function stepSlots(s: Slots, i: number, by: number): number {
  const dir = by < 0 ? -1 : 1
  let at = Math.max(0, Math.min(s.count - 1, i))
  let left = Math.abs(by)
  let last = ownHeight(s, at) > 0 ? at : -1
  while (left > 0) {
    const next = at + dir
    if (next < 0 || next >= s.count) break
    at = next
    if (ownHeight(s, at) > 0) {
      last = at
      left--
    }
  }
  return last < 0 ? Math.max(0, Math.min(s.count - 1, i)) : last
}

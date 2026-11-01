/**
 * Browsers cap an element's height (Chromium at about 33.5M px), and a very long history is
 * taller than that: 1.3M rows at 52px is 68M px. Past a safe height, the scroll area is
 * capped and its scroll position is scaled to a position in the full ("virtual") height.
 */
export const MAX_SCROLL_PX = 8_000_000

export interface ScrollMap {
  /** Height of the scroll area's spacer, in px. */
  spacer: number
  /** Virtual px per scroll px: 1 when the history fits, more when it's scaled. */
  ratio: number
  /** The largest virtual top, so the last row can sit at the bottom of the view. */
  maxTop: number
}

export function scrollMap(contentPx: number, viewportPx: number, max = MAX_SCROLL_PX): ScrollMap {
  const maxTop = Math.max(0, contentPx - viewportPx)
  if (contentPx <= max) return { spacer: contentPx, ratio: 1, maxTop }
  return { spacer: max, ratio: maxTop / Math.max(1, max - viewportPx), maxTop }
}

/** The virtual top for a scroll position. */
export const toVirtual = (scrollTop: number, m: ScrollMap) => Math.min(m.maxTop, scrollTop * m.ratio)

/** The scroll position for a virtual top. */
export const toScroll = (top: number, m: ScrollMap) => top / m.ratio

/**
 * The virtual top after a scroll event. ownScroll is the scrollTop we last set ourselves, or
 * null. Our own scroll keeps top as it is, since reading it back would round it to the scale.
 */
export function topAfterScroll(scrollTop: number, ownScroll: number | null, top: number, m: ScrollMap): number {
  return ownScroll !== null && Math.abs(scrollTop - ownScroll) <= 1 ? top : toVirtual(scrollTop, m)
}

import { describe, expect, it } from 'vitest'
import { scrollMap, toScroll, toVirtual, topAfterScroll } from './scrollMap'

describe('scrollMap', () => {
  it('leaves a history that fits unscaled', () => {
    const m = scrollMap(5_000, 600, 10_000)
    expect(m).toEqual({ spacer: 5_000, ratio: 1, maxTop: 4_400 })
    expect(toVirtual(1_234, m)).toBe(1_234)
  })

  it('caps a taller history and maps the ends to the ends', () => {
    const rows = 1_300_000
    const m = scrollMap(rows * 52, 800, 8_000_000)
    expect(m.spacer).toBe(8_000_000)
    expect(toVirtual(0, m)).toBe(0)
    // Scrolled to the bottom of the capped area, the last row sits at the bottom of the view.
    expect(toVirtual(8_000_000 - 800, m)).toBeCloseTo(rows * 52 - 800)
  })

  it('round-trips a virtual position', () => {
    const m = scrollMap(68_000_000, 800, 8_000_000)
    expect(toVirtual(toScroll(30_000_000, m), m)).toBeCloseTo(30_000_000)
  })
})

describe('topAfterScroll', () => {
  const m = scrollMap(5_000, 600, 10_000)

  it('follows a scroll the person made', () => {
    expect(topAfterScroll(300, null, 100, m)).toBe(300)
  })

  // The graph couldn't get back to the top: position 0 looked like our own scroll (#340).
  it('reaches the very top', () => {
    expect(topAfterScroll(0, null, 172, m)).toBe(0)
  })

  it('reaches the very top from one pixel down', () => {
    expect(topAfterScroll(1, null, 172, m)).toBe(1)
    expect(topAfterScroll(0, null, 1, m)).toBe(0)
  })

  it('ignores the scroll event caused by setting the position, within rounding', () => {
    expect(topAfterScroll(300, 300, 300.4, m)).toBe(300.4)
    expect(topAfterScroll(301, 300, 300.4, m)).toBe(300.4)
    expect(topAfterScroll(0, 0, 0.3, m)).toBe(0.3)
  })

  it('follows a real scroll that lands after one of ours', () => {
    expect(topAfterScroll(0, 300, 300, m)).toBe(0)
  })

  it('keeps precision when scaled', () => {
    const big = scrollMap(68_000_000, 600, 8_000_000)
    expect(topAfterScroll(100, 100, 850.5, big)).toBe(850.5)
    expect(topAfterScroll(0, null, 850.5, big)).toBe(0)
  })
})

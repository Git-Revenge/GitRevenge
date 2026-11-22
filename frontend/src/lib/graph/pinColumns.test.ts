import { describe, expect, it } from 'vitest'
import { layoutColumns, PIN_COLUMN, PIN_COLUMNS } from './draw'

// Compact gives each trailer key pinned to rows a column of its own, left of the author,
// while the message can spare the room (#868).

describe('a pinned key’s column', () => {
  it('is not there with nothing pinned, or in the timeline', () => {
    expect(layoutColumns(1500, 100, 'compact').pinX).toEqual([])
    expect(layoutColumns(1500, 100, 'compact', 0, 'origin', 150, 0).pinX).toEqual([])
    // The timeline writes a pinned key's values after the subject.
    expect(layoutColumns(1500, 100, 'comfortable', 0, 'origin', 150, 2).pinX).toEqual([])
  })

  it('sits just left of the author, and moves nothing else', () => {
    const plain = layoutColumns(1500, 100, 'compact')
    const one = layoutColumns(1500, 100, 'compact', 0, 'origin', 150, 1)
    expect(one.pinX).toEqual([plain.authorX - PIN_COLUMN])
    expect(one.pinW).toBe(PIN_COLUMN)
    for (const column of ['textX', 'authorX', 'authorW', 'dateX', 'dateW', 'shaX', 'refW'] as const) expect(one[column]).toBe(plain[column])
  })

  it('gives two keys a column each, in order, side by side', () => {
    const lay = layoutColumns(1500, 100, 'compact', 0, 'origin', 150, 2)
    expect(lay.pinX).toHaveLength(2)
    expect(lay.pinX[1] - lay.pinX[0]).toBe(PIN_COLUMN)
    expect(lay.pinX[1] + PIN_COLUMN).toBe(lay.authorX)
  })

  it('has no more columns than it has room for, however many keys are pinned', () => {
    expect(PIN_COLUMNS).toBe(2)
    expect(layoutColumns(2400, 100, 'compact', 0, 'origin', 150, 5).pinX).toHaveLength(PIN_COLUMNS)
  })

  it('gives columns up as the window narrows, and the message keeps its room', () => {
    let last = PIN_COLUMNS
    for (let width = 2000; width >= 500; width -= 20) {
      const lay = layoutColumns(width, 100, 'compact', 0, 'origin', 150, 2)
      // Never more as it narrows.
      expect(lay.pinX.length).toBeLessThanOrEqual(last)
      last = lay.pinX.length
      // With a column, the message still has a few hundred pixels to be read in.
      if (lay.pinX.length) expect(lay.pinX[0] - lay.textX).toBeGreaterThanOrEqual(360)
    }
    expect(last).toBe(0)
    // Two fit a wide window, one a middling one, none a narrow one.
    expect(layoutColumns(1500, 100, 'compact', 0, 'origin', 150, 2).pinX).toHaveLength(2)
    expect(layoutColumns(1200, 100, 'compact', 0, 'origin', 150, 2).pinX).toHaveLength(1)
    expect(layoutColumns(1000, 100, 'compact', 0, 'origin', 150, 2).pinX).toHaveLength(0)
  })

  it('takes its room from the message when there is no author column', () => {
    // Too narrow for an author or a date: the column sits left of the SHA.
    const lay = layoutColumns(790, 60, 'compact', 0, 'origin', 120, 1)
    expect(lay.authorW).toBe(0)
    if (lay.pinX.length) expect(lay.pinX[0] + PIN_COLUMN).toBe(lay.shaX)
  })
})

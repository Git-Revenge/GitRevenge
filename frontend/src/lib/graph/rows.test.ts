import { describe, expect, it } from 'vitest'
import { anchorAt, anchorFor, anchorForBottom, clampToEnd, compareAnchors, distance, endAnchor, layout, moveBy, normalize, ownHeight, position, rowAt, slotHeight, START, stepSlots, yOf, type SlotRow, type Slots } from './rows'

/** Every slot one commit row of the same height: the graph as it has always been. */
const even = (count: number, h = 50): Slots => ({ count, rows: () => [{ kind: 'commit', h }] })

/**
 * A mixed list, six slots:
 *   0  a lead row (40)
 *   1  a day line (26) then a starter commit (52)
 *   2  a follower commit (30)
 *   3  a commit folded away (0)
 *   4  a fold line (28) then a commit folded away (0)
 *   5  a day line (26) then a starter commit (52)
 */
const mixedRows: SlotRow[][] = [
  [{ kind: 'lead', h: 40 }],
  [
    { kind: 'day', h: 26 },
    { kind: 'commit', h: 52 },
  ],
  [{ kind: 'commit', h: 30 }],
  [{ kind: 'commit', h: 0 }],
  [
    { kind: 'fold', h: 28 },
    { kind: 'commit', h: 0 },
  ],
  [
    { kind: 'day', h: 26 },
    { kind: 'commit', h: 52 },
  ],
]
const mixed: Slots = { count: mixedRows.length, rows: (i) => mixedRows[i] }
// Slot tops: 0, 40, 118, 148, 148, 176; the list is 254 tall.

describe('slots', () => {
  it('measure a slot by all its rows, and its own row by the last', () => {
    expect(slotHeight(mixed, 1)).toBe(78)
    expect(ownHeight(mixed, 1)).toBe(52)
    expect(slotHeight(mixed, 4)).toBe(28)
    expect(ownHeight(mixed, 4)).toBe(0)
  })
})

describe('normalize', () => {
  it('keeps an anchor inside the list and inside its slot', () => {
    expect(normalize(even(10), { index: 3, offset: 20 })).toEqual({ index: 3, offset: 20 })
    expect(normalize(even(10), { index: 3, offset: 80 })).toEqual({ index: 3, offset: 50 })
    expect(normalize(even(10), { index: 3, offset: -5 })).toEqual({ index: 3, offset: 0 })
    expect(normalize(even(10), { index: 99, offset: 20 })).toEqual({ index: 9, offset: 0 })
    expect(normalize(even(10), { index: -4, offset: 20 })).toEqual({ index: 0, offset: 0 })
  })

  it('is the start for an empty list', () => {
    expect(normalize(even(0), { index: 3, offset: 20 })).toEqual(START)
  })
})

describe('moveBy', () => {
  it('walks down and up through slots of one height', () => {
    expect(moveBy(even(100), START, 125)).toEqual({ index: 2, offset: 25 })
    expect(moveBy(even(100), { index: 2, offset: 25 }, -125)).toEqual(START)
    expect(moveBy(even(100), { index: 5, offset: 10 }, -20)).toEqual({ index: 4, offset: 40 })
  })

  it('walks through slots of different heights, and over ones of no height', () => {
    expect(moveBy(mixed, START, 40)).toEqual({ index: 1, offset: 0 })
    expect(moveBy(mixed, START, 130)).toEqual({ index: 2, offset: 12 })
    // 148 is where slots 3 (nothing) and 4 (the fold line) begin: it lands on the one with a row.
    expect(moveBy(mixed, START, 148)).toEqual({ index: 4, offset: 0 })
    expect(moveBy(mixed, { index: 5, offset: 0 }, -10)).toEqual({ index: 4, offset: 18 })
    expect(moveBy(mixed, { index: 4, offset: 0 }, -1)).toEqual({ index: 2, offset: 29 })
  })

  it('stops at either end', () => {
    expect(moveBy(even(4), { index: 1, offset: 0 }, -500)).toEqual(START)
    expect(moveBy(even(4), { index: 1, offset: 0 }, 5000)).toEqual({ index: 3, offset: 50 })
    expect(moveBy(even(0), START, 100)).toEqual(START)
  })

  it('comes back to where it started', () => {
    const from = { index: 2, offset: 7 }
    for (const dy of [1, 29, 60, 100]) expect(moveBy(mixed, moveBy(mixed, from, dy), -dy)).toEqual(from)
  })
})

describe('the end', () => {
  it("puts the last row's bottom edge at the bottom of the view", () => {
    // 100 slots of 50 = 5000 tall; a 320 view ends with its top at 4680.
    expect(endAnchor(even(100), 320)).toEqual({ index: 93, offset: 30 })
    // The mixed list is 254 tall, so a 100 view ends with its top at 154: 6 into slot 4's fold line.
    expect(endAnchor(mixed, 100)).toEqual({ index: 4, offset: 6 })
  })

  it('is the top when everything fits', () => {
    expect(endAnchor(even(4), 320)).toEqual(START)
    expect(endAnchor(even(0), 320)).toEqual(START)
  })

  it('holds an anchor back from going past it', () => {
    expect(clampToEnd(even(100), { index: 99, offset: 0 }, 320)).toEqual({ index: 93, offset: 30 })
    expect(clampToEnd(even(100), { index: 10, offset: 5 }, 320)).toEqual({ index: 10, offset: 5 })
  })

  it('orders anchors', () => {
    expect(compareAnchors({ index: 1, offset: 9 }, { index: 2, offset: 0 })).toBeLessThan(0)
    expect(compareAnchors({ index: 2, offset: 9 }, { index: 2, offset: 0 })).toBeGreaterThan(0)
    expect(compareAnchors({ index: 2, offset: 3 }, { index: 2, offset: 3 })).toBe(0)
  })
})

describe('layout', () => {
  it('places the rows that show, from a row cut by the top edge to one cut by the bottom', () => {
    const rows = layout(even(100), { index: 2, offset: 20 }, 120)
    expect(rows).toEqual([
      { kind: 'commit', slot: 2, y: -20, h: 50 },
      { kind: 'commit', slot: 3, y: 30, h: 50 },
      { kind: 'commit', slot: 4, y: 80, h: 50 },
    ])
  })

  it('places rows of each kind and height, and leaves out rows of no height', () => {
    expect(layout(mixed, START, 500)).toEqual([
      { kind: 'lead', slot: 0, y: 0, h: 40 },
      { kind: 'day', slot: 1, y: 40, h: 26 },
      { kind: 'commit', slot: 1, y: 66, h: 52 },
      { kind: 'commit', slot: 2, y: 118, h: 30 },
      { kind: 'fold', slot: 4, y: 148, h: 28 },
      { kind: 'day', slot: 5, y: 176, h: 26 },
      { kind: 'commit', slot: 5, y: 202, h: 52 },
    ])
  })

  it("starts part way into a slot, past rows that have scrolled off", () => {
    // 30 into slot 1: its day line (26) is gone, and 4 of its commit row.
    expect(layout(mixed, { index: 1, offset: 30 }, 60).map((r) => [r.kind, r.slot, r.y])).toEqual([
      ['commit', 1, -4],
      ['commit', 2, 48],
    ])
  })

  it('is empty for an empty list', () => {
    expect(layout(even(0), START, 300)).toEqual([])
  })

  it('finds the row at a height', () => {
    const rows = layout(mixed, START, 500)
    expect(rowAt(rows, 0)?.kind).toBe('lead')
    expect(rowAt(rows, 65)?.kind).toBe('day')
    expect(rowAt(rows, 66)).toMatchObject({ kind: 'commit', slot: 1 })
    expect(rowAt(rows, 150)).toMatchObject({ kind: 'fold', slot: 4 })
    expect(rowAt(rows, 254)).toBeUndefined()
    expect(rowAt(rows, -1)).toBeUndefined()
  })
})

describe('going to a slot', () => {
  it("puts its own row a share of the way down the view", () => {
    // Slot 40's row is at 2000. At the top edge, the anchor is the slot itself.
    expect(anchorFor(even(100), 40, 300, 0)).toEqual({ index: 40, offset: 0 })
    // A third down a 300 view: the top is at 1900.
    expect(anchorFor(even(100), 40, 300, 1 / 3)).toEqual({ index: 38, offset: 0 })
    expect(anchorFor(even(100), 40, 300, 0.5)).toEqual({ index: 37, offset: 0 })
  })

  it("goes by the slot's own row, below any rows that come with it", () => {
    // Slot 5's commit row is at 202: its day line is above the view's top edge.
    expect(anchorFor(mixed, 5, 100, 0)).toEqual({ index: 5, offset: 26 })
  })

  it('stops at the top when there is not that much above it', () => {
    expect(anchorFor(even(100), 1, 300, 0.5)).toEqual(START)
  })

  it("puts the bottom of a slot's row at the bottom of the view", () => {
    // Slot 40 ends at 2050; a 300 view then starts at 1750.
    expect(anchorForBottom(even(100), 40, 300)).toEqual({ index: 35, offset: 0 })
    expect(anchorForBottom(even(100), 2, 300)).toEqual(START)
  })
})

describe('distance', () => {
  it('measures between two anchors, either way', () => {
    expect(distance(even(100), { index: 2, offset: 10 }, { index: 5, offset: 20 }, 10_000)).toBe(160)
    expect(distance(even(100), { index: 5, offset: 20 }, { index: 2, offset: 10 }, 10_000)).toBe(-160)
    expect(distance(even(100), { index: 5, offset: 20 }, { index: 5, offset: 20 }, 10_000)).toBe(0)
    expect(distance(mixed, START, { index: 5, offset: 26 }, 10_000)).toBe(202)
  })

  it('agrees with moveBy', () => {
    const from = { index: 1, offset: 5 }
    expect(distance(mixed, from, moveBy(mixed, from, 97), 10_000)).toBe(97)
  })

  it('gives up past the limit, so a far jump is never walked', () => {
    expect(distance(even(1_000_000), START, { index: 900_000, offset: 0 }, 900)).toBeNull()
    expect(distance(even(100), START, { index: 10, offset: 0 }, 500)).toBe(500)
    expect(distance(even(100), START, { index: 10, offset: 1 }, 500)).toBeNull()
  })
})

describe('yOf', () => {
  it("finds a slot's own row from the view's top edge", () => {
    const a = { index: 10, offset: 20 }
    expect(yOf(even(100), a, 10, 300, 600)).toBe(-20)
    expect(yOf(even(100), a, 12, 300, 600)).toBe(80)
    expect(yOf(even(100), a, 8, 300, 600)).toBe(-120)
    expect(yOf(mixed, START, 1, 300, 600)).toBe(66)
    expect(yOf(mixed, { index: 5, offset: 0 }, 1, 300, 600)).toBe(-110)
  })

  it('is null for a slot beyond reach, or not in the list', () => {
    const a = { index: 50, offset: 0 }
    expect(yOf(even(100), a, 90, 300, 600)).toBeNull()
    expect(yOf(even(100), a, 10, 300, 600)).toBeNull()
    expect(yOf(even(100), a, 68, 300, 600)).toBe(900)
    expect(yOf(even(100), a, 69, 300, 600)).toBeNull()
    expect(yOf(even(100), a, 38, 300, 600)).toBe(-600)
    expect(yOf(even(100), a, 37, 300, 600)).toBeNull()
    expect(yOf(even(100), a, 100, 300, 600)).toBeNull()
    expect(yOf(even(100), a, -1, 300, 600)).toBeNull()
  })
})

describe('position', () => {
  it('is the slot plus how far through it, and comes back the same', () => {
    expect(position(even(100), { index: 4, offset: 25 })).toBe(4.5)
    expect(anchorAt(even(100), 4.5)).toEqual({ index: 4, offset: 25 })
    expect(position(mixed, { index: 1, offset: 39 })).toBe(1.5)
    expect(anchorAt(mixed, 1.5)).toEqual({ index: 1, offset: 39 })
  })

  it('stays inside the list', () => {
    expect(anchorAt(even(10), -3)).toEqual(START)
    expect(anchorAt(even(10), 500)).toEqual({ index: 9, offset: 50 })
    expect(anchorAt(even(0), 3)).toEqual(START)
    expect(position(mixed, { index: 3, offset: 0 })).toBe(3)
  })

  it('is proportional to px when every slot is one height, as a native scrollbar is', () => {
    const s = even(1000, 52)
    const top = 31_234
    expect(position(s, moveBy(s, START, top)) * 52).toBeCloseTo(top)
  })
})

describe('stepSlots', () => {
  it('moves by slots when every one shows', () => {
    expect(stepSlots(even(10), 3, 1)).toBe(4)
    expect(stepSlots(even(10), 3, -2)).toBe(1)
    expect(stepSlots(even(10), 3, 50)).toBe(9)
    expect(stepSlots(even(10), 3, -50)).toBe(0)
  })

  it("steps over slots whose own row doesn't show", () => {
    // In the mixed list only slots 0, 1, 2 and 5 have a row of their own.
    expect(stepSlots(mixed, 2, 1)).toBe(5)
    expect(stepSlots(mixed, 5, -1)).toBe(2)
    expect(stepSlots(mixed, 1, 2)).toBe(5)
    expect(stepSlots(mixed, 5, 1)).toBe(5)
  })

  it('starts from a hidden slot by finding the next one that shows', () => {
    expect(stepSlots(mixed, 3, 1)).toBe(5)
    expect(stepSlots(mixed, 3, -1)).toBe(2)
  })
})

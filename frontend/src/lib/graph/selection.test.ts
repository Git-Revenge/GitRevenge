import { describe, expect, it } from 'vitest'
import { add, between, fromRows, has, nearest, size, tidy, toggle } from './selection'

describe('a selection of rows', () => {
  it('keeps runs sorted, joining those that touch or overlap', () => {
    expect(tidy([{ lo: 8, hi: 9 }, { lo: 0, hi: 2 }, { lo: 3, hi: 4 }, { lo: 6, hi: 8 }])).toEqual([
      { lo: 0, hi: 4 },
      { lo: 6, hi: 9 },
    ])
    expect(tidy([{ lo: 5, hi: 4 }])).toEqual([])
  })

  it('answers whether a row is in it, and how many there are', () => {
    const runs = tidy([{ lo: 2, hi: 4 }, { lo: 10, hi: 100_009 }])
    expect([1, 2, 4, 5, 9, 10, 50_000, 100_009, 100_010].map((i) => has(runs, i))).toEqual([false, true, true, false, false, true, true, true, false])
    expect(size(runs)).toBe(100_003)
    expect(size([])).toBe(0)
  })

  it('selects every row between two, either way round', () => {
    expect(between(7, 3)).toEqual([{ lo: 3, hi: 7 }])
    expect(add(between(1, 2), between(4, 4))).toEqual([
      { lo: 1, hi: 2 },
      { lo: 4, hi: 4 },
    ])
  })

  it('toggles one row, splitting a run it was inside', () => {
    expect(toggle(between(1, 5), 3)).toEqual([
      { lo: 1, hi: 2 },
      { lo: 4, hi: 5 },
    ])
    expect(toggle(between(1, 5), 1)).toEqual([{ lo: 2, hi: 5 }])
    expect(toggle(between(1, 2), 3)).toEqual([{ lo: 1, hi: 3 }])
    expect(toggle(between(4, 4), 4)).toEqual([])
  })

  it('is made again from rows, leaving out commits that have none', () => {
    expect(fromRows([5, -1, 3, 4, -2, 9])).toEqual([
      { lo: 3, hi: 5 },
      { lo: 9, hi: 9 },
    ])
  })

  it('finds the nearest selected row', () => {
    const runs = tidy([{ lo: 2, hi: 3 }, { lo: 9, hi: 9 }])
    expect(nearest(runs, 6)).toBe(9)
    expect(nearest(runs, 5)).toBe(3)
    expect(nearest(runs, 2)).toBe(2)
    expect(nearest([], 2)).toBe(-1)
  })
})

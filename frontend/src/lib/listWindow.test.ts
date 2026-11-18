import { describe, expect, it } from 'vitest'
import { drawnRows, moveTarget, rowTops, visibleRows } from './listWindow'

/** A list of n rows, all this tall. */
const even = (n: number, height = 30) => rowTops(Array.from({ length: n }), () => height)

describe('rowTops', () => {
  it('adds up the heights, ending with the height of the list', () => {
    expect(rowTops(['folder', 'file', 'file', 'folder'], (r) => (r === 'folder' ? 24 : 30))).toEqual([0, 24, 54, 84, 108])
  })

  it('is just the end for an empty list', () => {
    expect(rowTops([], () => 30)).toEqual([0])
  })
})

describe('visibleRows', () => {
  it('draws the view plus overscan at the top', () => {
    expect(visibleRows(even(1000), 0, 300, 5)).toEqual({ start: 0, end: 15 })
  })

  it('moves with the scroll position', () => {
    expect(visibleRows(even(1000), 3000, 300, 5)).toEqual({ start: 95, end: 115 })
  })

  it('counts a row that is only partly in view', () => {
    expect(visibleRows(even(1000), 3010, 300, 0)).toEqual({ start: 100, end: 111 })
  })

  it('stops at the end of the list', () => {
    expect(visibleRows(even(1000), 29_850, 300, 5)).toEqual({ start: 990, end: 1000 })
  })

  it('finds its place among rows of different heights', () => {
    // A heading of 24 above every nine files of 30: each group is 294 tall.
    const tops = rowTops(Array.from({ length: 1000 }, (_, i) => i % 10 === 0), (heading) => (heading ? 24 : 30))
    expect(visibleRows(tops, 2940, 294, 0)).toEqual({ start: 100, end: 110 })
    expect(visibleRows(tops, 2940 + 24, 30, 0)).toEqual({ start: 101, end: 102 })
  })

  it('draws only the first rows of a list still below the view', () => {
    expect(visibleRows(even(1000), -5000, 300, 5)).toEqual({ start: 0, end: 5 })
  })

  it('draws only the last rows of a list scrolled away above the view', () => {
    expect(visibleRows(even(1000), 90_000, 300, 5)).toEqual({ start: 994, end: 1000 })
  })

  it('copes with a short list, and an empty one', () => {
    expect(visibleRows(even(3), 0, 300)).toEqual({ start: 0, end: 3 })
    expect(visibleRows(even(0), 0, 300)).toEqual({ start: 0, end: 0 })
    expect(visibleRows(even(0), 500, 0)).toEqual({ start: 0, end: 0 })
  })
})

describe('drawnRows', () => {
  it('is the range', () => {
    expect(drawnRows({ start: 3, end: 6 })).toEqual([3, 4, 5])
    expect(drawnRows({ start: 0, end: 0 })).toEqual([])
  })

  it('keeps a row above or below the range, in order', () => {
    expect(drawnRows({ start: 3, end: 6 }, 0)).toEqual([0, 3, 4, 5])
    expect(drawnRows({ start: 3, end: 6 }, 9)).toEqual([3, 4, 5, 9])
  })

  it('draws a kept row in the range once', () => {
    expect(drawnRows({ start: 3, end: 6 }, 4)).toEqual([3, 4, 5])
  })
})

describe('moveTarget', () => {
  //            0     1    2    3     4    5
  const rows = ['F', 'f', 'f', 'F', 'f', 'F']
  const move = (key: string, from: number) => moveTarget(key, rows, from, (r) => r === 'f')

  it('moves up and down, past rows that take no focus', () => {
    expect(move('ArrowDown', 1)).toBe(2)
    expect(move('ArrowDown', 2)).toBe(4)
    expect(move('ArrowUp', 4)).toBe(2)
  })

  it('stays put at the ends', () => {
    expect(move('ArrowUp', 1)).toBe(1)
    expect(move('ArrowDown', 4)).toBe(4)
  })

  it('goes to the first and last rows that take focus', () => {
    expect(move('Home', 4)).toBe(1)
    expect(move('End', 1)).toBe(4)
  })

  it('leaves other keys alone', () => {
    expect(move('PageDown', 1)).toBeNull()
    expect(move('Enter', 1)).toBeNull()
  })

  it('stays put when no row takes focus', () => {
    expect(moveTarget('End', ['F', 'F'], 0, () => false)).toBe(0)
    expect(moveTarget('Home', [], 0, () => true)).toBe(0)
  })
})

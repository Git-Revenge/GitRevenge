import { describe, expect, it } from 'vitest'
import { gapInGrid, gapInList, indexForGap, isInside, type Box } from './reorder'

const row = (top: number): Box => ({ left: 0, right: 100, top, bottom: top + 20 })
const rows = [row(0), row(20), row(40)]

describe('gapInList', () => {
  it('uses each row’s middle', () => {
    expect(gapInList(5, rows)).toBe(0)
    expect(gapInList(15, rows)).toBe(1)
    expect(gapInList(25, rows)).toBe(1)
    expect(gapInList(35, rows)).toBe(2)
    expect(gapInList(55, rows)).toBe(3)
  })

  it('handles being above or below the list', () => {
    expect(gapInList(-50, rows)).toBe(0)
    expect(gapInList(500, rows)).toBe(3)
  })

  it('handles an empty list', () => {
    expect(gapInList(5, [])).toBe(0)
  })
})

describe('gapInGrid', () => {
  // Two rows of two cards, 100 wide and 100 tall.
  const card = (left: number, top: number): Box => ({ left, right: left + 100, top, bottom: top + 100 })
  const cards = [card(0, 0), card(100, 0), card(0, 100), card(100, 100)]

  it('uses the middle of each card along its row', () => {
    expect(gapInGrid(20, 50, cards)).toBe(0)
    expect(gapInGrid(80, 50, cards)).toBe(1)
    expect(gapInGrid(120, 50, cards)).toBe(1)
    expect(gapInGrid(180, 50, cards)).toBe(2)
  })

  it('moves on to the next row', () => {
    expect(gapInGrid(20, 150, cards)).toBe(2)
    expect(gapInGrid(120, 150, cards)).toBe(3)
    expect(gapInGrid(180, 150, cards)).toBe(4)
  })

  it('handles the pointer outside the grid', () => {
    expect(gapInGrid(-50, -50, cards)).toBe(0)
    expect(gapInGrid(500, 500, cards)).toBe(4)
  })

  it('uses the next row’s start when right of a short row', () => {
    expect(gapInGrid(500, 50, cards)).toBe(2)
  })
})

describe('isInside', () => {
  it('includes the top left edge, not the bottom right', () => {
    const b = row(0)
    expect(isInside(0, 0, b)).toBe(true)
    expect(isInside(99, 19, b)).toBe(true)
    expect(isInside(100, 10, b)).toBe(false)
    expect(isInside(10, 20, b)).toBe(false)
  })
})

describe('indexForGap', () => {
  it('moves an item later, allowing for itself leaving the list', () => {
    expect(indexForGap(0, 3)).toBe(2)
    expect(indexForGap(0, 2)).toBe(1)
  })

  it('moves an item earlier', () => {
    expect(indexForGap(2, 0)).toBe(0)
    expect(indexForGap(2, 1)).toBe(1)
  })

  it('does nothing when dropped on either side of itself', () => {
    expect(indexForGap(1, 1)).toBeNull()
    expect(indexForGap(1, 2)).toBeNull()
  })
})

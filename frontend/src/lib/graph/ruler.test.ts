import { describe, expect, it } from 'vitest'
import { boxFor, MIN_BOX, monthAt, percentThrough, placeTicks, positionFor, rowToY, tickAt, tickSummary, unreadFrom, type RulerRows } from './ruler'

const rows = (over: Partial<RulerRows> = {}): RulerRows => ({ head: -1, tips: [], tags: [], notes: [], matches: [], ...over })

describe('rowToY', () => {
  it('puts a row as far down the ruler as it is through the rows', () => {
    expect(rowToY(0, 1000, 500)).toBe(0)
    expect(rowToY(500, 1000, 500)).toBe(250)
    expect(rowToY(1000, 1000, 500)).toBe(500)
  })

  it('stays on the ruler, and is 0 with no rows', () => {
    expect(rowToY(5000, 1000, 500)).toBe(500)
    expect(rowToY(-5, 1000, 500)).toBe(0)
    expect(rowToY(3, 0, 500)).toBe(0)
  })
})

describe('placeTicks', () => {
  it('puts each thing at its row, in order down the ruler', () => {
    const ticks = placeTicks(rows({ head: 100, tips: [0, 900], tags: [500], notes: [300], matches: [700] }), 1000, 500)
    expect(ticks.map((t) => t.y)).toEqual([0, 50, 150, 250, 350, 450])
    expect(ticks[1]).toMatchObject({ head: true, tips: 0 })
    expect(ticks[2]).toMatchObject({ notes: 1 })
    expect(ticks[3]).toMatchObject({ tags: 1 })
    expect(ticks[4]).toMatchObject({ matches: 1 })
  })

  it('draws things nearer than 2px as one tick that counts them', () => {
    // Rows 100, 101 and 102 of 1000 are 0.5px apart on a 500px ruler; row 110 is 5px on.
    const ticks = placeTicks(rows({ head: 100, tags: [101, 102, 110], notes: [100] }), 1000, 500)
    expect(ticks).toHaveLength(2)
    expect(ticks[0]).toMatchObject({ y: 50, head: true, tags: 2, notes: 1 })
    expect(ticks[1]).toMatchObject({ y: 55, tags: 1 })
  })

  it('measures a group from its first tick, so a long run of close things is several ticks', () => {
    const ticks = placeTicks(rows({ tags: [0, 2, 4, 6, 8] }), 1000, 500)
    // 1px apart: the first three are within 2px of the first, then a new tick begins.
    expect(ticks.map((t) => [t.y, t.tags])).toEqual([
      [0, 2],
      [2, 2],
      [4, 1],
    ])
  })

  it('leaves out anything not among the rows', () => {
    expect(placeTicks(rows({ head: -1, tips: [-1, 1000, 5000], tags: [999] }), 1000, 500)).toHaveLength(1)
    expect(placeTicks(rows({ head: 3 }), 0, 500)).toEqual([])
  })
})

describe('what a tick says', () => {
  it('names what is there, counted', () => {
    const [tick] = placeTicks(rows({ head: 10, tips: [10], tags: [10, 10, 10], notes: [10], matches: [10, 10] }), 1000, 500)
    expect(tickSummary(tick)).toBe('the checked-out commit, 1 branch, 3 tags, 1 note, 2 matches')
    expect(tickSummary(placeTicks(rows({ tips: [1, 1] }), 1000, 500)[0])).toBe('2 branches')
  })

  it('is found from a height near it', () => {
    const ticks = placeTicks(rows({ tags: [100, 600] }), 1000, 500)
    expect(tickAt(ticks, 51)?.y).toBe(50)
    expect(tickAt(ticks, 297)?.y).toBe(300)
    expect(tickAt(ticks, 60)).toBeUndefined()
  })
})

describe('the box', () => {
  it('is as tall as the share of the rows in view, and as far down as the view is', () => {
    expect(boxFor(0, 100, 1000, 500)).toEqual({ top: 0, height: 50 })
    expect(boxFor(450, 100, 1000, 500)).toEqual({ top: 225, height: 50 })
    expect(boxFor(900, 100, 1000, 500)).toEqual({ top: 450, height: 50 })
  })

  it('is never too short to grab, and still reaches the bottom at the end', () => {
    const box = boxFor(1_499_980, 20, 1_500_000, 500)
    expect(box.height).toBe(MIN_BOX)
    expect(box.top + box.height).toBe(500)
    expect(boxFor(0, 20, 1_500_000, 500)).toEqual({ top: 0, height: MIN_BOX })
  })

  it('fills the ruler when everything is in view', () => {
    expect(boxFor(0, 30, 12, 500)).toEqual({ top: 0, height: 500 })
  })

  it('is nothing with no rows', () => {
    expect(boxFor(0, 10, 0, 500)).toEqual({ top: 0, height: 0 })
  })

  it('gives back the position it was drawn from', () => {
    for (const [count, visible] of [
      [1000, 100],
      [1_500_000, 20],
    ]) {
      for (const position of [0, count / 3, count - visible]) {
        const { top } = boxFor(position, visible, count, 500)
        expect(positionFor(top, visible, count, 500)).toBeCloseTo(position, 3)
      }
    }
  })

  it('stays within the rows when dragged past either end', () => {
    expect(positionFor(-40, 100, 1000, 500)).toBe(0)
    expect(positionFor(9000, 100, 1000, 500)).toBe(900)
    expect(positionFor(100, 30, 12, 500)).toBe(0)
  })

  it('says how far through the history it is, as a scrollbar does', () => {
    expect(percentThrough(0, 100, 1000)).toBe(0)
    expect(percentThrough(450, 100, 1000)).toBe(50)
    expect(percentThrough(900, 100, 1000)).toBe(100)
    expect(percentThrough(0, 30, 12)).toBe(0)
  })
})

describe('months', () => {
  const months = [
    { row: 0, time: 300 },
    { row: 40, time: 200 },
    { row: 95, time: 100 },
  ]

  it('finds the month a row is in', () => {
    expect(monthAt(months, 0)).toBe(300)
    expect(monthAt(months, 39)).toBe(300)
    expect(monthAt(months, 40)).toBe(200)
    expect(monthAt(months, 5000)).toBe(100)
  })

  it('is null with no marks', () => {
    expect(monthAt([], 3)).toBeNull()
  })
})

describe('unreadFrom', () => {
  it('is where the rows not yet read begin', () => {
    expect(unreadFrom(2000, 10_000, 500)).toBe(100)
  })

  it('is null once every row is read', () => {
    expect(unreadFrom(10_000, 10_000, 500)).toBeNull()
    expect(unreadFrom(12_000, 10_000, 500)).toBeNull()
  })
})

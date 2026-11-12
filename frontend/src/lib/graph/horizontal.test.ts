import { describe, expect, it } from 'vitest'
import { clampOffset, COL, colAt, colX, dayCount, daySpans, LANE, lanePitch, laneY, LEAN_REACH, maxOffset, offsetForThumb, offsetToShow, segPath, subjectLength, THUMB_MIN, thumbFor, visibleRange, WORDS, wordsHeight } from './horizontal'

const DAY = 24 * 60 * 60
// Noon, so a few hours either way stay in the same day in any time zone.
const NOON = Math.floor(new Date(2026, 10, 11, 12).getTime() / 1000)
const at = (days: number, hours = 0) => ({ authorTime: NOON - days * DAY - hours * 3600 })

describe('columns', () => {
  it('centres a commit in its column', () => {
    expect(colX(0)).toBe(COL / 2)
    expect(colX(3)).toBe(3 * COL + COL / 2)
  })

  it('finds the column under a point, and none past the end', () => {
    expect(colAt(10, 0, 5)).toBe(0)
    expect(colAt(10, COL * 2, 5)).toBe(2)
    expect(colAt(COL * 5 + 1, 0, 5)).toBe(-1)
  })

  it('scrolls no further than the last column at the right edge', () => {
    expect(maxOffset(400, 100)).toBe(100 * COL - 400)
    expect(maxOffset(400, 3)).toBe(0)
    expect(clampOffset(-20, 400, 100)).toBe(0)
    expect(clampOffset(1e9, 400, 100)).toBe(100 * COL - 400)
  })

  it('draws what shows, the columns whose subjects lean in, and one beyond', () => {
    expect(visibleRange(0, 440, 100)).toEqual({ first: 0, last: 11 })
    const r = visibleRange(COL * 20, 440, 100)
    expect(r.first).toBe(20 - LEAN_REACH)
    expect(r.last).toBe(31)
    expect(visibleRange(0, 440, 3)).toEqual({ first: 0, last: 3 })
    expect(visibleRange(0, 440, 0)).toEqual({ first: 0, last: 0 })
  })
})

describe('offsetToShow', () => {
  it('leaves the view alone when the column already shows with room to spare', () => {
    expect(offsetToShow(COL * 10, 440, 14, 100)).toBe(COL * 10)
  })

  it('brings a column off the left back with one column before it', () => {
    expect(offsetToShow(COL * 10, 440, 8, 100)).toBe(COL * 7)
    expect(offsetToShow(COL * 10, 440, 0, 100)).toBe(0)
  })

  it('brings a column off the right back with one column after it', () => {
    expect(offsetToShow(0, 440, 20, 100)).toBe(22 * COL - 440)
    expect(offsetToShow(0, 440, 99, 100)).toBe(maxOffset(440, 100))
  })
})

describe('bands', () => {
  it('gives the subjects their full band when the view is tall, and less when the lanes need it', () => {
    expect(wordsHeight(700, 2)).toBe(WORDS.max)
    expect(wordsHeight(260, 4)).toBe(WORDS.min)
    const between = wordsHeight(260, 2)
    expect(between).toBeGreaterThan(WORDS.min)
    expect(between).toBeLessThan(WORDS.max)
  })

  it('lets a subject run longer in a taller band', () => {
    expect(subjectLength(WORDS.max)).toBeGreaterThan(subjectLength(WORDS.min))
    expect(subjectLength(WORDS.max)).toBeGreaterThan(200)
  })

  it('packs lanes closer when many share the room, down to a floor', () => {
    expect(lanePitch(2, 300)).toBe(LANE.max)
    expect(lanePitch(12, 240)).toBe(Math.floor((240 - LANE.pad * 2) / 12))
    expect(lanePitch(80, 240)).toBe(LANE.min)
    expect(laneY(0, 26)).toBe(LANE.pad)
    expect(laneY(2, 26)).toBe(LANE.pad + 52)
  })
})

describe('segPath', () => {
  const cx = 100
  const y = (col: number) => laneY(col, 26)

  it('runs a line that keeps its lane straight across its half of the column', () => {
    expect(segPath({ from: 1, to: 1 }, cx, 'up', 26)).toBe(`M${cx - COL / 2} ${y(1)}H${cx}`)
    expect(segPath({ from: 1, to: 1 }, cx, 'down', 26)).toBe(`M${cx} ${y(1)}H${cx + COL / 2}`)
  })

  it('starts an up line at the newer side on its old lane and ends it mid-column on its new one', () => {
    const d = segPath({ from: 0, to: 2 }, cx, 'up', 26)
    expect(d.startsWith(`M${cx - COL / 2} ${y(0)}`)).toBe(true)
    expect(d.endsWith(`V${y(2)}`)).toBe(true)
  })

  it('starts a down line mid-column and ends it at the older side on its new lane', () => {
    const d = segPath({ from: 2, to: 0 }, cx, 'down', 26)
    expect(d.startsWith(`M${cx} ${y(2)}`)).toBe(true)
    expect(d.endsWith(`H${cx + COL / 2}`)).toBe(true)
    expect(d).toContain(`${y(0)}`)
  })

  it('joins a down line to the next column’s up line', () => {
    // The older end of one column is the newer end of the next, on the same lane.
    const down = segPath({ from: 0, to: 1 }, cx, 'down', 26)
    const up = segPath({ from: 1, to: 1 }, cx + COL, 'up', 26)
    expect(down.endsWith(`H${cx + COL / 2}`)).toBe(true)
    expect(up.startsWith(`M${cx + COL / 2} ${y(1)}`)).toBe(true)
  })
})

describe('days', () => {
  it('groups neighbouring commits made on the same day', () => {
    const rows = [at(0), at(0, 1), at(1), at(1, 2), at(1, 3), at(4)]
    expect(daySpans(rows, 10).map((d) => [d.start, d.end])).toEqual([
      [10, 12],
      [12, 15],
      [15, 16],
    ])
    expect(daySpans(rows, 10)[1].time).toBe(rows[2].authorTime)
  })

  it('ends a day at a row that isn’t read yet', () => {
    const rows = [at(0), undefined, at(0, 1)]
    expect(daySpans(rows, 0).map((d) => [d.start, d.end])).toEqual([
      [0, 1],
      [2, 3],
    ])
  })

  it('counts a whole day outwards from any of its commits', () => {
    const rows = [at(0), at(1), at(1, 1), at(1, 2), at(2)]
    const get = (i: number) => rows[i]
    expect(dayCount(get, 2, rows.length)).toBe(3)
    expect(dayCount(get, 0, rows.length)).toBe(1)
    expect(dayCount(get, 4, rows.length)).toBe(1)
  })

  it('has no count for a day that runs into rows not read yet', () => {
    const rows = [at(1), at(1, 1), undefined]
    expect(dayCount((i) => rows[i], 0, 3)).toBeNull()
    // The history ends there: the day is whole.
    expect(dayCount((i) => rows[i], 0, 2)).toBe(2)
  })
})

describe('the scrollbar', () => {
  it('fills the track when everything fits', () => {
    expect(thumbFor(0, 500, 5, 400)).toEqual({ left: 0, width: 400 })
  })

  it('is as long as the share in view, and never too short to grab', () => {
    const t = thumbFor(0, 440, 100, 400)
    expect(t.width).toBeCloseTo((440 / (100 * COL)) * 400)
    expect(thumbFor(0, 440, 100000, 400).width).toBe(THUMB_MIN)
  })

  it('reaches the end of the track at the end of the history', () => {
    const t = thumbFor(maxOffset(440, 100), 440, 100, 400)
    expect(t.left + t.width).toBeCloseTo(400)
  })

  it('maps back from a thumb position to the same offset', () => {
    const offset = COL * 30
    const t = thumbFor(offset, 440, 100, 400)
    expect(offsetForThumb(t.left, 440, 100, 400)).toBeCloseTo(offset)
    expect(offsetForThumb(9999, 440, 100, 400)).toBe(maxOffset(440, 100))
  })
})

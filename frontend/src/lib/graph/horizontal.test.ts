import { describe, expect, it } from 'vitest'
import { anchorAt, anchorOffset, anchorToShow, chipRooms, clampOffset, dayStretches, moveAnchor, nextShown, placeColumns, settle, spotAt, COL, colX, dayBandText, dayCount, LABEL_MAX, LANE, lanesAt, lanesAtEdge, laneScrollToShow, lanesExtent, laneY, maxLaneScroll, maxOffset, mergedName, namesWidth, NAMES, offsetForThumb, segPath, subjectLength, THUMB_MIN, thumbFor, WORDS, wordsHeight } from './horizontal'

const DAY = 24 * 60 * 60
// Noon, so a few hours either way stay in the same day in any time zone.
const NOON = Math.floor(new Date(2026, 10, 11, 12).getTime() / 1000)
const at = (days: number, hours = 0) => ({ authorTime: NOON - days * DAY - hours * 3600 })

describe('columns', () => {
  it('centres a commit in its column', () => {
    expect(colX(0)).toBe(COL / 2)
    expect(colX(3)).toBe(3 * COL + COL / 2)
  })

  it('scrolls no further than the last column at the right edge', () => {
    expect(maxOffset(400, 100)).toBe(100 * COL - 400)
    expect(maxOffset(400, 3)).toBe(0)
    expect(clampOffset(-20, 400, 100)).toBe(0)
    expect(clampOffset(1e9, 400, 100)).toBe(100 * COL - 400)
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

  it('keeps lanes one height, and scrolls them when there are more than fit', () => {
    expect(laneY(0, LANE.max)).toBe(LANE.pad)
    expect(laneY(2, LANE.max)).toBe(LANE.pad + 2 * LANE.max)
    expect(lanesExtent(1)).toBe(LANE.pad * 2)
    expect(lanesExtent(5)).toBe(LANE.pad * 2 + 4 * LANE.max)
    expect(maxLaneScroll(3, 400)).toBe(0)
    expect(maxLaneScroll(40, 300)).toBe(lanesExtent(40) - 300)
  })

  it('scrolls the lanes to a lane out of sight, and not at all to one that shows', () => {
    expect(laneScrollToShow(0, 2, 40, 300)).toBe(0)
    const down = laneScrollToShow(0, 30, 40, 300)
    expect(laneY(30, LANE.max) - down).toBeLessThan(300)
    expect(laneY(30, LANE.max) - down).toBeGreaterThan(0)
    expect(laneScrollToShow(down, 0, 40, 300)).toBe(0)
    expect(laneScrollToShow(0, 39, 40, 300)).toBe(maxLaneScroll(40, 300))
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

describe('lane names', () => {
  const pass = (lane: number, color: number) => ({ from: lane, to: lane, color })
  // Commit on lane 0 (colour 0), with lane 2 (colour 7) passing by.
  const first = { col: 0, color: 0, up: [pass(0, 0), pass(2, 7)], down: [pass(0, 0), pass(2, 7)] }
  // Further right, a commit on lane 1 (colour 4): a lane that isn't at the left edge.
  const second = { col: 1, color: 4, up: [pass(0, 0), pass(2, 7)], down: [pass(0, 0), pass(2, 7), { from: 1, to: 0, color: 4 }] }

  it('reads the lanes through a column: its own and the ones passing by', () => {
    expect([...lanesAt(first)]).toEqual([
      [0, 0],
      [2, 7],
    ])
    // A line that changes lane isn't a lane of this column.
    expect(lanesAt(second).get(1)).toBe(4)
    expect(lanesAt(second).size).toBe(3)
  })

  it('names each row by the lane at the left edge, or the first to turn up further right', () => {
    const rows = lanesAtEdge([first, undefined, second])
    expect(rows.get(0)).toEqual({ color: 0, later: false })
    expect(rows.get(2)).toEqual({ color: 7, later: false })
    expect(rows.get(1)).toEqual({ color: 4, later: true })
  })

  it('gives a tag chip the room up to the next one', () => {
    const rooms = chipRooms([2, 3, 10])
    expect(rooms.get(2)).toBe(COL - 6)
    expect(rooms.get(3)).toBe(LABEL_MAX)
    expect(rooms.get(10)).toBe(LABEL_MAX)
  })

  it('narrows the pinned column in a narrow view, within limits', () => {
    expect(namesWidth(1200)).toBe(NAMES.max)
    expect(namesWidth(400)).toBe(136)
    expect(namesWidth(200)).toBe(96)
  })
})

describe('the day band', () => {
  const time = Math.floor(new Date(2026, 10, 3, 12).getTime() / 1000)
  const full = 'Tue, Nov 3'

  it('says the day and its count when there is room for both', () => {
    expect(dayBandText(full, '14 commits', time, 400)).toEqual({ label: full, count: '14 commits' })
  })

  it('drops the count before it shortens the name', () => {
    expect(dayBandText(full, '14 commits', time, 132)).toEqual({ label: full, count: '' })
  })

  it('never cuts a name part way: a narrow day says the date, a one-commit day the day', () => {
    const two = dayBandText(full, '2 commits', time, 2 * COL)
    expect(two.count).toBe('')
    expect(two.label).toMatch(/^(Nov 3|3)$/)
    expect(dayBandText(full, '1 commit', time, COL)).toEqual({ label: '3', count: '' })
  })
})

describe('a merged lane\u2019s name', () => {
  it('is the branch or tag git says it merged', () => {
    expect(mergedName("Merge branch 'topic'")).toBe('topic')
    expect(mergedName("Merge branch 'topic' into main")).toBe('topic')
    expect(mergedName("Merge tag 'platform-drivers-x86-v7.4-1' of git://git.kernel.org/pub/scm/linux")).toBe('platform-drivers-x86-v7.4-1')
    expect(mergedName("Merge remote-tracking branch 'origin/release'")).toBe('origin/release')
    expect(mergedName("Merge branches 'a' and 'b'")).toBe('a')
  })

  it('is a pull request\u2019s branch and number', () => {
    expect(mergedName('Merge pull request #655 from Git-Revenge/652-map-view')).toBe('652-map-view #655')
    expect(mergedName('Merge pull request #7 from topic')).toBe('topic #7')
  })

  it('reads the other hosts\u2019 wording too', () => {
    expect(mergedName('Merged in feature/login (pull request #12)')).toBe('feature/login')
    expect(mergedName('Merge main into topic')).toBe('main')
  })

  it('is nothing when the subject does not say', () => {
    expect(mergedName('feat: a ruler in place of the scrollbar (#638)')).toBe('')
    expect(mergedName('')).toBe('')
  })
})

describe('columns that fold away', () => {
  const all = () => true
  // Commits 3 to 6 are folded away.
  const folded = (i: number) => i < 3 || i > 6

  it('steps over what doesn\u2019t show, and stops at the ends', () => {
    expect(nextShown(2, 1, 20, folded)).toBe(7)
    expect(nextShown(7, -1, 20, folded)).toBe(2)
    expect(nextShown(19, 1, 20, all)).toBeNull()
    expect(nextShown(0, -1, 20, all)).toBeNull()
    // A lead column sits before commit 0 and always shows.
    expect(nextShown(0, -1, 20, all, 1)).toBe(-1)
    expect(nextShown(-1, -1, 20, all, 1)).toBeNull()
  })

  it('places columns side by side, with nothing for a folded commit', () => {
    const spots = placeColumns({ i: 0, dx: 0 }, 6 * COL, 20, folded, 0, 0)
    expect(spots.map((s) => s.i)).toEqual([0, 1, 2, 7, 8, 9, 10])
    expect(spots.map((s) => s.x)).toEqual([0, 1, 2, 3, 4, 5, 6].map((k) => k * COL))
  })

  it('draws a few columns off the left edge, for their leaning subjects', () => {
    const spots = placeColumns({ i: 8, dx: 10 }, 3 * COL, 20, folded, 0, 3)
    expect(spots.map((s) => s.i)).toEqual([1, 2, 7, 8, 9, 10, 11, 12])
    expect(spots.find((s) => s.i === 8)!.x).toBe(-10)
    expect(spots.find((s) => s.i === 7)!.x).toBe(-10 - COL)
  })

  it('starts with the lead column when there is one', () => {
    expect(placeColumns({ i: -1, dx: 0 }, 3 * COL, 20, all, 1, 0).map((s) => s.i)).toEqual([-1, 0, 1, 2])
  })

  it('scrolls across shown columns only', () => {
    expect(moveAnchor({ i: 2, dx: 0 }, COL, 20, folded)).toEqual({ i: 7, dx: 0 })
    expect(moveAnchor({ i: 7, dx: 0 }, -COL - 10, 20, folded)).toEqual({ i: 1, dx: COL - 10 })
    expect(moveAnchor({ i: 0, dx: 0 }, -500, 20, all)).toEqual({ i: 0, dx: 0 })
    expect(moveAnchor({ i: 18, dx: 0 }, 500, 20, all)).toEqual({ i: 19, dx: 0 })
    expect(moveAnchor({ i: 0, dx: 0 }, -COL, 20, all, 1)).toEqual({ i: -1, dx: 0 })
  })

  it('settles an anchor on a column that shows', () => {
    expect(settle({ i: 4, dx: 12 }, 20, folded)).toEqual({ i: 7, dx: 0 })
    expect(settle({ i: 99, dx: 0 }, 20, all)).toEqual({ i: 19, dx: 0 })
    expect(settle({ i: -5, dx: 0 }, 20, all, 1)).toEqual({ i: -1, dx: 0 })
  })

  it('leaves the view alone for a column that shows with room to spare', () => {
    const a = { i: 0, dx: 0 }
    const spots = placeColumns(a, 10 * COL, 50, all)
    expect(anchorToShow(a, spots, 10 * COL, 4, 50, all)).toBe(a)
  })

  it('brings a column at the right edge in, and one far away to the left edge', () => {
    const a = { i: 0, dx: 0 }
    const spots = placeColumns(a, 10 * COL, 50, all)
    const near = anchorToShow(a, spots, 10 * COL, 9, 50, all)
    expect(placeColumns(near, 10 * COL, 50, all).find((s) => s.i === 9)!.x + 2 * COL).toBeLessThanOrEqual(10 * COL)
    expect(anchorToShow(a, spots, 10 * COL, 40, 50, all)).toEqual({ i: 39, dx: 0 })
    // The newest commit brings the lead column with it.
    const far = { i: 30, dx: 0 }
    expect(anchorToShow(far, placeColumns(far, 10 * COL, 50, all, 1), 10 * COL, 0, 50, all, 1)).toEqual({ i: -1, dx: 0 })
  })

  it('turns an anchor into a distance for the scrollbar and back', () => {
    expect(anchorOffset({ i: 5, dx: 10 })).toBe(5 * COL + 10)
    expect(anchorOffset({ i: -1, dx: 0 }, 1)).toBe(0)
    expect(anchorAt(5 * COL + 10, 20, all)).toEqual({ i: 5, dx: 10 })
    expect(anchorAt(4 * COL, 20, folded)).toEqual({ i: 7, dx: 0 })
    expect(anchorAt(0, 20, all, 1)).toEqual({ i: -1, dx: 0 })
  })

  it('finds the column under a point', () => {
    const spots = placeColumns({ i: 0, dx: 0 }, 6 * COL, 20, folded, 0, 0)
    expect(spotAt(spots, COL * 3 + 5)!.i).toBe(7)
    expect(spotAt(spots, -5)).toBeUndefined()
  })

  it('groups placed commits by day, across a fold', () => {
    const noon = Math.floor(new Date(2026, 10, 11, 12).getTime() / 1000)
    const time = (i: number) => (i < 8 ? noon : noon - 86400)
    const spots = placeColumns({ i: 0, dx: 0 }, 6 * COL, 20, folded, 0, 0)
    const days = dayStretches(spots, time)
    expect(days.map((d) => [d.first, d.x, d.width])).toEqual([
      [0, 0, 4 * COL],
      [8, 4 * COL, 3 * COL],
    ])
  })
})

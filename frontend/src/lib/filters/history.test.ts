import { describe, expect, it } from 'vitest'
import {
  aboutText,
  backToText,
  bigCount,
  changeMark,
  chipName,
  chipVerb,
  countText,
  dayOfSince,
  fileMark,
  historyKey,
  historyOptions,
  historyStatus,
  lastYear,
  narrowText,
  offersNarrowing,
  sinceChoice,
  sinceOfDay,
  tookText,
} from './history'

const now = new Date(2026, 10, 25, 12)
const at = (y: number, m: number, d: number) => Math.floor(new Date(y, m - 1, d).getTime() / 1000)
const state = (o: Partial<Parameters<typeof historyStatus>[0]> = {}) => ({
  options: historyOptions('km/h'),
  found: 0,
  searched: 0,
  total: 0,
  backTo: 0,
  running: false,
  stopped: false,
  ms: 0,
  estimateMs: 0,
  problem: '',
  ...o,
})

describe('historyOptions and historyKey', () => {
  it('starts as plain text, any case, everywhere, and names options apart', () => {
    expect(historyOptions('km/h')).toEqual({ text: 'km/h', matchCase: false, regex: false, lines: false, paths: [], since: 0 })
    expect(historyKey(historyOptions('km/h'))).toBe(historyKey(historyOptions('km/h')))
    expect(historyKey(historyOptions('km/h', { lines: true }))).not.toBe(historyKey(historyOptions('km/h')))
    expect(historyKey(historyOptions('km/h', { paths: ['src/'] }))).not.toBe(historyKey(historyOptions('km/h')))
  })
})

describe('the chip', () => {
  it('says what a commit has to have done', () => {
    expect(chipVerb({ lines: false })).toBe('adds or removes')
    expect(chipVerb({ lines: true })).toBe('changes a line with')
    expect(chipName({ text: 'km/h', lines: false })).toBe('Commits that add or remove km/h, options')
    expect(chipName({ text: 'km/h', lines: true })).toBe('Commits that change a line with km/h, options')
  })
})

describe('backToText', () => {
  it('is a day lately, a month this year, and a part of a year before', () => {
    expect(backToText(at(2026, 10, 20), now)).toBe('Oct 20')
    expect(backToText(at(2026, 3, 9), now)).toBe('March 2026')
    expect(backToText(at(2024, 11, 2), now)).toBe('late 2024')
    expect(backToText(at(2024, 6, 2), now)).toBe('mid 2024')
    expect(backToText(at(2014, 2, 2), now)).toBe('early 2014')
  })
})

describe('aboutText and tookText', () => {
  it('rounds an estimate to seconds, minutes or hours', () => {
    expect(aboutText(400)).toBe('about 1 second')
    expect(aboutText(40_000)).toBe('about 40 seconds')
    expect(aboutText(20 * 60_000)).toBe('about 20 minutes')
    expect(aboutText(61_000)).toBe('about 1 minute')
    expect(aboutText(2 * 3_600_000)).toBe('about 2 hours')
    expect(aboutText(5_400_000)).toBe('about 1.5 hours')
  })

  it('says how long a search took', () => {
    expect(tookText(20)).toBe('0.1 s')
    expect(tookText(240)).toBe('0.2 s')
    expect(tookText(40_000)).toBe('40 s')
    expect(tookText(347_000)).toBe('5 min 47 s')
    expect(tookText(120_000)).toBe('2 min')
  })

  it('counts a big history in millions', () => {
    expect(bigCount(1_511)).toBe('1,511')
    expect(bigCount(1_507_311)).toBe('1.5 million')
  })
})

describe('historyStatus', () => {
  it('while it runs: how many found, and how far back', () => {
    expect(historyStatus(state({ running: true, found: 83, backTo: at(2024, 11, 2) }), now)).toEqual({ tone: 'running', text: 'Searching… 83 found, back to late 2024.' })
    expect(historyStatus(state({ running: true }), now).text).toBe('Searching… 0 found.')
  })

  it('stopped: where, and that older commits are left', () => {
    expect(historyStatus(state({ stopped: true, found: 83, backTo: at(2024, 11, 2) }), now)).toEqual({ tone: 'stopped', text: 'Stopped at late 2024: 83 found. Older commits weren’t searched.' })
    expect(historyStatus(state({ stopped: true }), now).text).toBe('Stopped before any commit was searched.')
  })

  it('done: what it searched, and how long it took', () => {
    // The strip's count says how many commits: a second count could only disagree with it.
    expect(historyStatus(state({ found: 7, searched: 1_510, ms: 200 }), now)).toEqual({ tone: 'done', text: 'Searched the whole history · 0.2 s' })
    const since = state({ options: historyOptions('x', { since: at(2025, 1, 1) }), searched: 177_366, ms: 40_000 })
    expect(historyStatus(since, now).text).toBe('Searched 177,366 commits since Jan 1, 2025 · 40 s')
  })

  it('ended early: the backend’s words, a partial clone calmer than a mistake', () => {
    expect(historyStatus(state({ problem: 'Not a regular expression: its ( and ) don’t pair up.' }), now)).toEqual({ tone: 'problem', text: 'Not a regular expression: its ( and ) don’t pair up.' })
    const partial = "This is a partial clone, and some commits' files were never downloaded, so the search stopped at Mar 3, 2024. A search never downloads anything."
    expect(historyStatus(state({ problem: partial }), now)).toEqual({ tone: 'partial', text: partial })
  })
})

describe('narrowing', () => {
  const big = { running: true, ms: 6_000, estimateMs: 20 * 60_000, total: 1_507_311 }
  it('is offered a few seconds into a search with long to go, over everything', () => {
    expect(offersNarrowing(state(big))).toBe(true)
    expect(narrowText(big)).toBe('All 1.5 million commits will take about 20 minutes. Narrow it?')
  })

  it('isn’t offered early, near the end, done, or once narrowed', () => {
    expect(offersNarrowing(state({ ...big, ms: 2_000 }))).toBe(false)
    expect(offersNarrowing(state({ ...big, estimateMs: 50_000 }))).toBe(false)
    expect(offersNarrowing(state({ ...big, running: false }))).toBe(false)
    expect(offersNarrowing(state({ ...big, options: historyOptions('x', { since: 1 }) }))).toBe(false)
    expect(offersNarrowing(state({ ...big, options: historyOptions('x', { paths: ['mm/'] }) }))).toBe(false)
  })
})

describe('as far back as', () => {
  it('reads a since as the whole history, the last year or a date', () => {
    expect(sinceChoice(0, now)).toBe('all')
    expect(sinceChoice(lastYear(now), now)).toBe('year')
    expect(sinceChoice(lastYear(now) - 3_600, now)).toBe('year')
    expect(sinceChoice(at(2025, 1, 1), now)).toBe('date')
  })

  it('turns a date input into a since and back', () => {
    expect(sinceOfDay('2025-01-31')).toBe(at(2025, 1, 31))
    expect(dayOfSince(at(2025, 1, 31))).toBe('2025-01-31')
    expect(sinceOfDay('')).toBe(0)
    expect(dayOfSince(0)).toBe('')
  })
})

describe('changeMark', () => {
  it('says what one file gained or lost, in words and a tone', () => {
    expect(changeMark([{ file: 'public/map.js', added: 1, removed: 0 }], 'km/h')).toEqual({ label: '+1 map.js', tone: 'plus', says: 'Brings km/h into public/map.js' })
    expect(changeMark([{ file: 'src/feed.js', added: 0, removed: 1 }], 'km/h')).toEqual({ label: '−1 feed.js', tone: 'minus', says: 'Takes km/h out of src/feed.js' })
    expect(changeMark([{ file: 'src/feed.js', added: 0, removed: 3 }], 'km/h')!.says).toBe('Takes km/h out of src/feed.js, 3 times')
  })

  it('says ± where a line with it changed and it stayed', () => {
    expect(changeMark([{ file: 'docs/checks.md', added: 1, removed: 1 }], 'km/h')).toEqual({ label: '±1 checks.md', tone: 'even', says: 'Changes a line with km/h in docs/checks.md' })
  })

  it('adds up several files, naming each in full', () => {
    const m = changeMark(
      [
        { file: 'a/one.js', added: 2, removed: 0 },
        { file: 'b/two.js', added: 1, removed: 0 },
      ],
      'km/h',
    )
    expect(m).toEqual({ label: '+3 in 2 files', tone: 'plus', says: 'Brings km/h into a/one.js, 2 times\nBrings km/h into b/two.js' })
  })

  it('is nothing without changes', () => {
    expect(changeMark(undefined, 'km/h')).toBeNull()
    expect(changeMark([], 'km/h')).toBeNull()
  })

  it('marks each file in the commit panel', () => {
    expect(fileMark({ file: 'a', added: 2, removed: 0 })).toEqual({ label: '+2', tone: 'plus' })
    expect(fileMark({ file: 'a', added: 0, removed: 1 })).toEqual({ label: '−1', tone: 'minus' })
    expect(fileMark({ file: 'a', added: 1, removed: 1 })).toEqual({ label: '±1', tone: 'even' })
  })
})

describe('countText', () => {
  it('is empty until a count is known', () => {
    expect(countText(undefined)).toBe('')
    expect(countText(1)).toBe('1 commit')
    expect(countText(7)).toBe('7 commits')
  })
})

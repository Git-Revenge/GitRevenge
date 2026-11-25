import { describe, expect, it } from 'vitest'
import { filePatterns, findPattern, findRows, findSummary, inView, lineAt, lineStarts, markedParts, matchAtLine, matchesIn, splitPath, type FoundLine } from './find'

const found = (file: string, line: number, text: string, matches?: { from: number; to: number }[]): FoundLine => ({ file, line, text, cutStart: false, cutEnd: false, matches: matches ?? (text.includes('km/h') ? [{ from: text.indexOf('km/h'), to: text.indexOf('km/h') + 4 }] : []) })

describe('findRows', () => {
  it('heads each file once, with how many of its lines matched', () => {
    const rows = findRows([found('public/map.js', 6, 'a'), found('public/map.js', 9, 'b'), found('notes.md', 1, 'c')])
    expect(rows.map((r) => (r.type === 'file' ? `${r.file} (${r.count})` : `  ${r.found.line}`))).toEqual(['public/map.js (2)', '  6', '  9', 'notes.md (1)', '  1'])
  })
})

describe('splitPath', () => {
  it('splits off the folder, keeping its slash', () => {
    expect(splitPath('public/map.js')).toEqual({ folder: 'public/', name: 'map.js' })
    expect(splitPath('README.md')).toEqual({ folder: '', name: 'README.md' })
  })
})

describe('markedParts', () => {
  it('marks each match and keeps the text between', () => {
    const l = found('a', 1, 'speed in km/h, or KM/H', [
      { from: 9, to: 13 },
      { from: 18, to: 22 },
    ])
    expect(markedParts(l)).toEqual([
      { text: 'speed in ', hit: false },
      { text: 'km/h', hit: true },
      { text: ', or ', hit: false },
      { text: 'KM/H', hit: true },
    ])
  })
  it('counts as JS does, so a wave before the match takes two', () => {
    expect(markedParts(found('a', 1, '🌊 km/h', [{ from: 3, to: 7 }]))).toEqual([
      { text: '🌊 ', hit: false },
      { text: 'km/h', hit: true },
    ])
  })
  it("leaves a line whose pattern JS and Go couldn't read unmarked", () => {
    expect(markedParts(found('a', 1, 'abab'))).toEqual([{ text: 'abab', hit: false }])
  })
})

describe('inView', () => {
  it('starts a little before the first match, so a narrow panel shows it', () => {
    const l = found('n', 10, '"source": ["# A week of wind", "Readings for August. The alert limit is $v > 60$ km/h."]')
    const v = inView(l)
    expect(v.cutStart).toBe(true)
    expect(v.text.slice(v.matches[0].from, v.matches[0].to)).toBe('km/h')
    expect(v.matches[0].from).toBe(16)
  })
  it('drops an indent, and leaves a short line as it is', () => {
    const v = inView(found('m', 6, '    pin.title = km/h', [{ from: 16, to: 20 }]))
    expect(v.text).toBe('pin.title = km/h')
    expect(v.cutStart).toBe(false)
    expect(v.matches).toEqual([{ from: 12, to: 16 }])
    const short = found('m', 1, 'km/h', [{ from: 0, to: 4 }])
    expect(inView(short)).toBe(short)
  })
  it('never cuts a character in two', () => {
    const v = inView(found('e', 1, '🌊'.repeat(10) + ' km/h', [{ from: 21, to: 25 }]), 4)
    expect(v.text.startsWith('\udc0a')).toBe(false)
    expect(v.text.slice(v.matches[0].from, v.matches[0].to)).toBe('km/h')
  })
})

describe('filePatterns', () => {
  it('splits on commas and drops the empty ones', () => {
    expect(filePatterns(' src/, *.js ,, !*.min.js, !')).toEqual(['src/', '*.js', '!*.min.js'])
    expect(filePatterns('')).toEqual([])
  })
})

describe('findSummary', () => {
  it('says how many lines and files, and how long it took', () => {
    expect(findSummary(2, 2, 200)).toBe('2 lines in 2 files · 0.2 s')
    expect(findSummary(1, 1, 31)).toBe('1 line in 1 file · 31 ms')
    expect(findSummary(10000, 569)).toBe(`${(10000).toLocaleString()} lines in 569 files`)
  })
})

describe('matchesIn', () => {
  const doc = 'pin.title = `${s.wind} km/h`\nKM/H in caps\nkmh, km, xkm km-x\n'
  it('finds text as written, ignoring case unless asked', () => {
    expect(matchesIn(doc, { text: 'km/h', matchCase: false, regex: false, wholeWord: false })).toEqual([
      { from: 23, to: 27 },
      { from: 29, to: 33 },
    ])
    expect(matchesIn(doc, { text: 'km/h', matchCase: true, regex: false, wholeWord: false })).toEqual([{ from: 23, to: 27 }])
  })
  it('reads regular expression characters as written unless asked', () => {
    expect(matchesIn('a.b axb', { text: 'a.b', matchCase: true, regex: false, wholeWord: false })).toEqual([{ from: 0, to: 3 }])
    expect(matchesIn('a.b axb', { text: 'a.b', matchCase: true, regex: true, wholeWord: false })).toHaveLength(2)
  })
  it('keeps whole words only when asked, as git grep -w does', () => {
    // km alone, and before a dash or a slash, but never inside kmh or xkm.
    expect(matchesIn('kmh, km, xkm km-x km/h', { text: 'km', matchCase: true, regex: false, wholeWord: true }).map((m) => m.from)).toEqual([5, 13, 18])
    expect(matchesIn('kmh, km, xkm km-x km/h', { text: 'km', matchCase: true, regex: false, wholeWord: false })).toHaveLength(5)
  })
  it("reads ^ and $ as a line's ends, and never spans lines", () => {
    expect(matchesIn('ab\nab', { text: '^ab$', matchCase: true, regex: true, wholeWord: false })).toEqual([
      { from: 0, to: 2 },
      { from: 3, to: 5 },
    ])
    expect(matchesIn('a\nb', { text: 'a\\sb', matchCase: true, regex: true, wholeWord: false })).toEqual([])
  })
  it("finds nothing in a regular expression JS can't read, rather than failing", () => {
    expect(findPattern({ text: 'kmalloc_array(', matchCase: false, regex: true })).toBeNull()
    expect(matchesIn('kmalloc_array(x)', { text: 'kmalloc_array(', matchCase: false, regex: true, wholeWord: false })).toEqual([])
  })
})

describe('lineAt', () => {
  it('says which line a position is on', () => {
    const starts = lineStarts('a km/h\nnothing\nkm/h b\n')
    expect([0, 6, 7, 14, 15, 21, 22].map((p) => lineAt(starts, p))).toEqual([1, 1, 2, 2, 3, 3, 4])
  })
})

describe('matchAtLine', () => {
  it('starts at the first match on the line, or the next one after it', () => {
    const doc = 'a km/h\nnothing\nkm/h b\n'
    const ms = matchesIn(doc, { text: 'km/h', matchCase: false, regex: false, wholeWord: false })
    const starts = lineStarts(doc)
    expect(starts).toEqual([0, 7, 15, 22])
    expect(matchAtLine(ms, starts, 1)).toBe(0)
    expect(matchAtLine(ms, starts, 2)).toBe(1)
    expect(matchAtLine(ms, starts, 3)).toBe(1)
    expect(matchAtLine(ms, starts, 9)).toBe(1)
  })
})

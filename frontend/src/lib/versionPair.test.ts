import { describe, expect, it } from 'vitest'
import { aroundBy, differsSaid, filesOf, markedRows, opens, parsePair, sideRows } from './versionPair'

// The demo's feature/offline-cache before and after an autosquash rebase (2026-11-27).
const changed = [
  '1:  1637a6b ! 1:  f6df5ac Cache the last reading from each station',
  '    @@ src/cache.js (new)',
  '     +const cache = new Map()',
  '     +',
  '     +export function remember(id, reading) {',
  '    -+  cache.set(id, { reading, at: Date.now() })',
  '    ++  if (reading) cache.set(id, { reading, at: Date.now() })',
  '     +}',
].join('\n')
const around = [
  '2:  eda7bec ! 2:  f16c299 wip',
  '    @@ Commit message',
  '      ## src/cache.js ##',
  '     @@ src/cache.js: const cache = new Map()',
  '      export function remember(id, reading) {',
  '    -   cache.set(id, { reading, at: Date.now() })',
  '    +   if (reading) cache.set(id, { reading, at: Date.now() })',
  '      }',
  '     +',
  '     +export function recall(id, maxAgeMs = 15 * 60_000) {',
].join('\n')
const reworded = [
  '5:  0b752a0 ! 3:  a27c8c8 Report wind in metres',
  '    @@ Metadata',
  '     Author: Tyler Brooks <tyler@tidewater.example>',
  '     ',
  '      ## Commit message ##',
  '    -    Report wind in metres',
  '    +    Report wind in knots',
  '     ',
  '      ## a.txt ##',
  '     @@ a.txt: a line',
].join('\n')

describe('parsePair', () => {
  it('reads a changed file, each line with both marks', () => {
    const s = parsePair(changed)
    expect(s.map((x) => [x.name, x.kind])).toEqual([['src/cache.js', 'file']])
    expect(s[0].lines.map((l) => `${l.outer}${l.inner}`)).toEqual([' +', ' +', ' +', '-+', '++', ' +'])
    expect(s[0].lines[4].text).toBe('  if (reading) cache.set(id, { reading, at: Date.now() })')
  })

  it("takes a file's own header over the section range-diff began", () => {
    const s = parsePair(around)
    expect(s.map((x) => x.name)).toEqual(['src/cache.js'])
    expect(s[0].lines.map((l) => `${l.outer}${l.inner}`)).toEqual([' hunk', '  ', '- ', '+ ', '  ', ' +', ' +'])
  })

  it('reads a reworded message, and leaves out what matches', () => {
    const s = parsePair(reworded)
    expect(s.map((x) => [x.name, x.kind])).toEqual([['Commit message', 'message']])
    expect(s[0].lines.filter((l) => l.outer !== ' ').map((l) => `${l.outer}${l.text}`)).toEqual(['-Report wind in metres', '+Report wind in knots'])
  })
})

describe('the views', () => {
  it('Marked picks out the words that differ', () => {
    const rows = markedRows(parsePair(changed)[0])
    const gone = rows[3]
    const added = rows[4]
    expect(gone.pieces.filter((p) => p.marked)).toEqual([])
    expect(added.pieces.filter((p) => p.marked).map((p) => p.text)).toEqual(['if (reading) '])
    expect(added.pieces.map((p) => p.text).join('')).toBe(added.text)
  })

  it('Side by side sets each run of gone lines beside its new ones', () => {
    const rows = sideRows(parsePair(changed)[0])
    expect(rows).toHaveLength(5)
    expect(rows[3].left?.text).toBe('  cache.set(id, { reading, at: Date.now() })')
    expect(rows[3].right?.text).toBe('  if (reading) cache.set(id, { reading, at: Date.now() })')
    expect(rows[0].left).toBe(rows[0].right)
  })

  it('a run of different lengths leaves a side empty', () => {
    const text = ['1:  a ! 1:  b X', '    @@ f.js', '    -+one', '    ++uno', '    ++dos'].join('\n')
    const rows = sideRows(parsePair(text)[0])
    expect(rows.map((r) => [r.left?.text ?? null, r.right?.text ?? null])).toEqual([
      ['one', 'uno'],
      [null, 'dos'],
    ])
  })
})

describe('around it', () => {
  const pairs = [
    { kind: 'changed', new: 1, subject: 'Cache the last reading from each station', text: changed },
    { kind: 'around', new: 2, subject: 'wip', text: around },
    { kind: 'same', new: 3, subject: 'Fall back', text: '4:  50d62a1 = 3:  bce321d Fall back' },
  ]

  it('names the files a pair touches', () => {
    expect(filesOf(changed)).toEqual(['src/cache.js'])
    expect(filesOf(around)).toEqual(['src/cache.js'])
    expect(filesOf(reworded)).toEqual(['a.txt'])
  })

  it('finds the earlier pair that moved the lines around it', () => {
    expect(aroundBy(pairs, 1)?.subject).toBe('Cache the last reading from each station')
    expect(aroundBy(pairs, 0)).toBeNull()
  })

  it('says how much differs', () => {
    expect(differsSaid(parsePair(changed))).toBe('the message is the same · 1 line differs')
    expect(differsSaid(parsePair(reworded))).toBe('the message changed')
  })

  it('steps through the pairs that changed', () => {
    expect(pairs.map(opens)).toEqual([true, true, false])
  })
})

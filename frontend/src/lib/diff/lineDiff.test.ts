import { describe, expect, it } from 'vitest'
import { applyChanges, diffLines, splitLines, toHunks, type Op } from './lineDiff'

const L = (...lines: string[]) => lines.map((l) => l + '\n')
const kinds = (ops: Op[]) => ops.map((o) => o.kind[0]).join('')

describe('splitLines', () => {
  it('keeps terminators so text round-trips', () => {
    const text = 'a\r\nb\nc'
    expect(splitLines(text)).toEqual(['a\r\n', 'b\n', 'c'])
    expect(splitLines(text).join('')).toBe(text)
    expect(splitLines('')).toEqual([])
  })
})

describe('diffLines', () => {
  it('finds a minimal edit', () => {
    const ops = diffLines(L('a', 'b', 'c', 'd'), L('a', 'x', 'c', 'd', 'e'))
    expect(kinds(ops)).toBe('edaeea')
  })

  it('handles empty sides', () => {
    expect(kinds(diffLines([], L('a', 'b')))).toBe('aa')
    expect(kinds(diffLines(L('a'), []))).toBe('d')
    expect(diffLines([], [])).toEqual([])
  })

  it('puts deletions before additions within a change', () => {
    expect(kinds(diffLines(L('a', 'old1', 'old2', 'z'), L('a', 'new1', 'z')))).toBe('eddae')
  })

  it('always reproduces both sides', () => {
    // Pseudo-random edits on a pseudo-random file.
    let seed = 7
    const rnd = (n: number) => (seed = (seed * 1103515245 + 12345) % 2 ** 31) % n
    for (let trial = 0; trial < 50; trial++) {
      const a = Array.from({ length: rnd(40) }, () => `l${rnd(8)}\n`)
      const b = a.filter(() => rnd(4) !== 0).flatMap((l) => (rnd(5) === 0 ? [l, `n${rnd(8)}\n`] : [l]))
      const ops = diffLines(a, b)
      expect(applyChanges(ops, a, b, () => true)).toBe(b.join(''))
      expect(applyChanges(ops, a, b, () => false)).toBe(a.join(''))
    }
  })
})

describe('toHunks', () => {
  it('groups nearby changes and reports git-style headers', () => {
    const a = L(...'abcdefghijklmnopqrst'.split(''))
    const b = [...a]
    b[2] = 'C\n' // change near the top
    b[4] = 'E\n' // close enough to merge with the first
    b[17] = 'R\n' // far away: separate hunk
    const hunks = toHunks(diffLines(a, b))
    expect(hunks).toHaveLength(2)
    expect(hunks[0]).toMatchObject({ oldStart: 1, oldCount: 8, newStart: 1, newCount: 8 })
    expect(hunks[1]).toMatchObject({ oldStart: 15, oldCount: 6, newStart: 15, newCount: 6 })
  })

  it('numbers a pure insertion into an empty file like git', () => {
    const hunks = toHunks(diffLines([], L('a', 'b')))
    expect(hunks[0]).toMatchObject({ oldStart: 0, oldCount: 0, newStart: 1, newCount: 2 })
  })
})

describe('applyChanges for partial staging', () => {
  const index = L('one', 'two', 'three')
  const work = L('ONE', 'two', 'THREE', 'four')
  const ops = diffLines(index, work)

  it('stages only selected lines', () => {
    // Apply the change to line one (its deletion and addition) only.
    const sel = new Set(ops.flatMap((o, i) => ((o.kind === 'del' && index[o.oldIndex] === 'one\n') || (o.kind === 'add' && work[o.newIndex] === 'ONE\n') ? [i] : [])))
    expect(applyChanges(ops, index, work, (i) => sel.has(i))).toBe('ONE\ntwo\nthree\n')
  })

  it('discards selected lines by applying everything else', () => {
    const four = ops.findIndex((o) => o.kind === 'add' && work[o.newIndex] === 'four\n')
    expect(applyChanges(ops, index, work, (i) => i !== four)).toBe('ONE\ntwo\nTHREE\n')
  })
})

// A file whose last line has no line ending: staging part of a change after it used to join
// that line to the next ("b" and "c" as "bc"), a line in neither version.
describe('applyChanges where a side has no final newline', () => {
  const pick = (ops: Op[], lines: string[], kind: Op['kind'], text: string) => ops.findIndex((o) => o.kind === kind && lines[kind === 'add' ? o.newIndex : o.oldIndex] === text)
  const every = (oldText: string, newText: string) => {
    const [a, b] = [splitLines(oldText), splitLines(newText)]
    const ops = diffLines(a, b)
    const changes = ops.flatMap((o, i) => (o.kind === 'eq' ? [] : [i]))
    // Every way of choosing among the changes.
    return Array.from({ length: 1 << changes.length }, (_, mask) => {
      const chosen = new Set(changes.filter((_, k) => mask & (1 << k)))
      return applyChanges(ops, a, b, (i) => chosen.has(i))
    })
  }

  it('staging a line added after the old last line ends that line first', () => {
    const [a, b] = [splitLines('a\nb'), splitLines('a\nb\nc\n')]
    const ops = diffLines(a, b)
    const c = pick(ops, b, 'add', 'c\n')
    expect(applyChanges(ops, a, b, (i) => i === c)).toBe('a\nb\nc\n')
  })

  it('keeping an old line after a new last line that has no ending ends that line too', () => {
    // Whichever order the changes come in, no two lines may share one.
    for (const out of every('a\nb\nc\n', 'a\nB')) {
      for (const line of splitLines(out)) expect(['a\n', 'b\n', 'c\n', 'B\n', 'B']).toContain(line)
    }
  })

  it('uses the file’s own line ending', () => {
    const [a, b] = [splitLines('a\r\nb'), splitLines('a\r\nb\r\nc\r\n')]
    const ops = diffLines(a, b)
    const c = pick(ops, b, 'add', 'c\r\n')
    expect(applyChanges(ops, a, b, (i) => i === c)).toBe('a\r\nb\r\nc\r\n')
  })

  it('still gives back each side exactly when all or none of it is applied', () => {
    for (const [oldText, newText] of [
      ['a\nb', 'a\nb\nc\n'],
      ['a\nb\n', 'a\nb'],
      ['a\nb', 'a\nB'],
      ['', 'a'],
      ['a', ''],
      ['a\nb\nc', 'x\nb\ny'],
    ]) {
      const [a, b] = [splitLines(oldText), splitLines(newText)]
      const ops = diffLines(a, b)
      expect(applyChanges(ops, a, b, () => true)).toBe(newText)
      expect(applyChanges(ops, a, b, () => false)).toBe(oldText)
    }
  })

  it('never makes a line that neither side has, whatever is chosen', () => {
    for (const [oldText, newText] of [
      ['a\nb', 'a\nb\nc\n'],
      ['a\nb', 'a\nb\nc'],
      ['a\nb\nc', 'a\nc'],
      ['one\ntwo', 'ONE\ntwo\nthree'],
    ]) {
      const known = new Set([...splitLines(oldText), ...splitLines(newText)].map((l) => l.replace(/\n$/, '')))
      for (const out of every(oldText, newText)) {
        for (const line of out.split('\n')) if (line !== '') expect(known).toContain(line)
      }
    }
  })
})

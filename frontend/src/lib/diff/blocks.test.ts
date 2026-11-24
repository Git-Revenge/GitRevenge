import { describe, expect, it } from 'vitest'
import { bandPath, mapLine, marksByLine, moveNote, opsFromBlocks, stops, type Block, type Result } from './blocks'
import { withMarks } from './highlight'
import { applyChanges, diffLines, splitLines } from './lineDiff'

const block = (oldStart: number, oldEnd: number, newStart: number, newEnd: number, extra: Partial<Block> = {}): Block =>
  ({ oldStart, oldEnd, newStart, newEnd, ...extra }) as Block
const result = (blocks: Block[], moves: Result['moves'] = []): Result => ({ blocks, moves, algorithm: 'myers', added: 0, removed: 0 }) as Result

describe('opsFromBlocks', () => {
  it('makes the ops diffLines makes, so staging rebuilds the same file', () => {
    const a = splitLines('a\nb\nc\nd\n')
    const b = splitLines('a\nx\nc\nd\ne\n')
    const ops = opsFromBlocks([block(1, 2, 1, 2), block(4, 4, 4, 5)], a.length, b.length)
    expect(ops).toEqual(diffLines(a, b))
    expect(applyChanges(ops, a, b, () => true)).toBe(b.join(''))
    expect(applyChanges(ops, a, b, () => false)).toBe(a.join(''))
  })

  it('handles empty sides', () => {
    expect(opsFromBlocks([block(0, 0, 0, 2)], 0, 2).map((o) => o.kind)).toEqual(['add', 'add'])
    expect(opsFromBlocks([block(0, 1, 0, 0)], 1, 0).map((o) => o.kind)).toEqual(['del'])
    expect(opsFromBlocks([], 0, 0)).toEqual([])
  })

  it('refuses blocks that disagree with the text', () => {
    expect(() => opsFromBlocks([block(0, 1, 0, 1)], 1, 3)).toThrow()
  })
})

describe('marks and moves', () => {
  it('groups marked words by line', () => {
    const m = marksByLine([block(0, 1, 0, 1, { old: [{ line: 0, from: 1, to: 3 }], new: [{ line: 0, from: 1, to: 4 }, { line: 0, from: 6, to: 7 }] } as Partial<Block>)])
    expect(m.old.get(0)).toEqual([[1, 3]])
    expect(m.new.get(0)).toEqual([
      [1, 4],
      [6, 7],
    ])
  })

  it('says where a move went and came from', () => {
    const mv = { oldStart: 23, oldEnd: 27, newStart: 10, newEnd: 14, reindented: false }
    expect(moveNote(mv, 'old')).toBe('moved to line 11')
    expect(moveNote(mv, 'new')).toBe('moved from line 24, unchanged')
    expect(moveNote({ ...mv, reindented: true }, 'new')).toBe('moved from line 24, indented differently')
  })
})

describe('stops', () => {
  it('steps through blocks and moves in reading order, a moved-only block once', () => {
    // Lines 5-6 moved to 0-1; line 2 changed.
    const r = result([block(0, 0, 0, 2), block(2, 3, 4, 5), block(5, 7, 7, 7)], [{ oldStart: 5, oldEnd: 7, newStart: 0, newEnd: 2, reindented: false }])
    expect(stops(r).map((s) => s.kind)).toEqual(['move', 'change'])
  })

  it('keeps a block that only partly moved, without its moved lines', () => {
    const r = result([block(0, 3, 0, 0)], [{ oldStart: 0, oldEnd: 2, newStart: 5, newEnd: 7, reindented: false }])
    expect(stops(r).map((s) => [s.kind, s.oldStart, s.oldEnd])).toEqual([
      ['del', 2, 3],
      ['move', 0, 2],
    ])
  })
})

describe('mapLine', () => {
  const blocks = [block(2, 3, 2, 5), block(10, 12, 12, 12)]
  it('runs the sides in step between blocks', () => {
    expect(mapLine(blocks, 1, 'old')).toBe(1)
    expect(mapLine(blocks, 5, 'old')).toBe(7)
    expect(mapLine(blocks, 7, 'new')).toBe(5)
  })
  it('spreads a block over the other side', () => {
    expect(mapLine(blocks, 2.5, 'old')).toBe(3.5)
    expect(mapLine(blocks, 3.5, 'new')).toBeCloseTo(2.5)
  })
  it('puts a removal at the point it was taken from', () => {
    expect(mapLine(blocks, 10.5, 'old')).toBe(12)
    expect(mapLine(blocks, 12, 'old')).toBe(12)
    expect(mapLine(blocks, 12, 'new')).toBe(12)
  })
})

describe('bandPath', () => {
  it('curves from the old lines to the new and back', () => {
    expect(bandPath(0, 21, 42, 84, 74)).toBe('M0 0 C37 0 37 42 74 42 L74 84 C37 84 37 21 0 21 Z')
  })
})

describe('withMarks', () => {
  it('splits tokens at the marks and flags the changed pieces', () => {
    const tokens = [
      { text: 'func ', cls: 'tok-keyword' },
      { text: 'forecast(at)', cls: '' },
    ]
    expect(withMarks(tokens, [[14, 16]])).toEqual([
      { text: 'func ', cls: 'tok-keyword' },
      { text: 'forecast(', cls: '' },
      { text: 'at', cls: '', changed: true },
      { text: ')', cls: '' },
    ])
  })

  it('leaves unmarked lines alone', () => {
    const tokens = [{ text: 'x', cls: '' }]
    expect(withMarks(tokens, undefined)).toBe(tokens)
  })

  it('marks across token edges', () => {
    expect(withMarks([{ text: 'ab', cls: 'a' }, { text: 'cd', cls: 'b' }], [[1, 3]])).toEqual([
      { text: 'a', cls: 'a' },
      { text: 'b', cls: 'a', changed: true },
      { text: 'c', cls: 'b', changed: true },
      { text: 'd', cls: 'b' },
    ])
  })
})
